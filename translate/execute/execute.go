//Primary public processing functions to interact with this library. All of these functions are available through the command line interface
//go:build !gol10n_read_compiled_only

// Package execute contains the functions called by the main command line interface
package execute

import (
	"errors"
	"fmt"
	"github.com/dakusan/gol10n/translate"
	"os"
	"regexp"
	"strings"
	"sync"
)

// ProcessSettings are taken from $SettingsFileName and are used to automatically read translation text files, compiled translation files, and go dictionary files.
//
// Compiled files are read from (and not written to) if their modification timestamps are newer than their translation text files, unless IgnoreTimestamps=true.
//
// Updating the default language may force all other languages to be updated.
type ProcessSettings struct {
	//The settings from $SettingsFileName
	DefaultLanguage        string //The identifier for the default language
	InputPath              string //The directory with the translation text files
	GoOutputPath           string //The directory to output the generated Go files to. Each namespace gets its own directory and file in the format “$NamespaceName/translationIDs.go”
	CompiledOutputPath     string //The directory to output the compiled binary translation files to. Each language gets its own .gtr or .gtr.gz (gzip compressed) file
	GoDictHeader           string //Extra code included just above the const in generated go dictionaries
	CompressCompiled       bool   //Whether the compiled binary translation files are saved as .gtr or .gtr.gz (gzip compressed)
	AllowBigStrings        bool   //If the translation strings can be larger than 64KB. If true, and a large translation string is found, then compiled binary translation files will become larger
	AllowJSONTrailingComma bool   //If JSON files can have trailing commas. If true, a sanitization process is ran over the JSON that changes the regular expression “,\s*\n\s*}” to just “}”

	//Extra settings added by [command line] flags
	OutputGoDictionary bool `json:"-"` //Whether to output go dictionary files
	OutputCompiled     bool `json:"-"` //Whether to output compiled .gtr files
	IgnoreTimestamps   bool `json:"-"` //Whether to force outputting all files, ignoring timestamps
}

// ProcessedFile is an item in the list of processed files and what was done to/with them.
type ProcessedFile struct {
	LangIdentifier string
	InputFileName  string
	Warnings       []string
	Err            error
	Flags          ProcessedFileFlag
	Lang           *translate.Language //Only filled if Flags.PFF_Language_Success*
}
type ProcessedFileFlag uint

// ProcessedFileList is a list of ProcessedFiles keyed to their language identifier
type ProcessedFileList map[string]*ProcessedFile

// Directory processes all files in the InputPath directory. It also returns the resultant languages.
//
// No ProcessedFiles are returned if any of the following errors occur: Directory error, language identity used more than once, default language not found
func (settings *ProcessSettings) Directory() (ProcessedFileList, error) {
	//Check and update the settings
	if err := settings.checkSettings(); err != nil {
		return nil, err
	}

	//Get a list of the files in the directory
	var d []os.DirEntry
	if _d, err := os.ReadDir(settings.InputPath); err != nil {
		return nil, errors.New("Error reading input path: " + err.Error())
	} else {
		d = _d
	}

	//Get list of files to process
	var filesToProcess []ProcessedFile
	defaultLanguageFileIndex := -1
	{
		var langIdentsFound = make(ProcessedFileList)
		checkFiletype := regexp.MustCompile(`^[a-z]{2,3}(-[a-z]{2,3})?\.(` + YAML_Extension + `|` + JSON_Extension + `)$`)
		for _, f := range d {
			//Only process files whose file extension matches json or yaml
			fName := f.Name()
			if f.IsDir() || !checkFiletype.MatchString(strings.ToLower(fName)) {
				continue
			}

			//Skip file if duplicate language identifier
			langIdent := fName[0:strings.LastIndexByte(fName, '.')]
			if _, ok := langIdentsFound[langIdent]; ok {
				return nil, fmt.Errorf("Language identity “%s” found again in file: %s", langIdent, fName)
			}

			//Store the file to process
			filesToProcess = append(filesToProcess, ProcessedFile{
				LangIdentifier: langIdent,
				InputFileName:  fName,
				Flags:          PFF_Load_NotAttempted,
			})
			langIdentsFound[langIdent] = &filesToProcess[len(filesToProcess)-1]

			//Store the index for the default file
			if langIdent == settings.DefaultLanguage {
				defaultLanguageFileIndex = len(filesToProcess) - 1
				filesToProcess[defaultLanguageFileIndex].Flags |= PFF_Language_IsDefault
			}
		}
	}

	//Return an error if the default language was not found
	if defaultLanguageFileIndex == -1 {
		return nil, fmt.Errorf("Default language “%s” not found", settings.DefaultLanguage)
	}

	//Processed language files start in unhandledLanguages
	unhandledLanguages := make(ProcessedFileList, len(filesToProcess))
	handledLanguages := make(ProcessedFileList, len(filesToProcess))

	//Process the default language. If there is an error with it, stop here
	{
		pf := &filesToProcess[defaultLanguageFileIndex]
		pf.Err = settings.processFile(pf, false)
		if pf.Err != nil {
			unhandledLanguages[settings.DefaultLanguage] = pf
			return unhandledLanguages, fmt.Errorf("Default language error: " + pf.Err.Error())
		}
	}

	//Process the other languages
	{
		var waitForFiles sync.WaitGroup
		for _fIndex := range filesToProcess {
			if _fIndex != defaultLanguageFileIndex {
				waitForFiles.Add(1)
				go func(fIndex int, pf *ProcessedFile) {
					defer waitForFiles.Done()
					pf.Err = settings.processFile(pf, false)
				}(_fIndex, &filesToProcess[_fIndex])
			}
		}
		waitForFiles.Wait()

		//Change filesToProcess into a map (ProcessedFileList) keyed to their language identifier
		hasErrors := false
		for _fIndex, fInfo := range filesToProcess {
			unhandledLanguages[fInfo.LangIdentifier] = &filesToProcess[_fIndex]
			hasErrors = hasErrors || fInfo.Err != nil
		}

		//Return if there are errors
		if hasErrors {
			return unhandledLanguages, errors.New("There were errors while processing files")
		}
	}

	//Move a language from the unhandled list to the handled list
	moveLang := func(langIdent string) {
		handledLanguages[langIdent] = unhandledLanguages[langIdent]
		delete(unhandledLanguages, langIdent)
	}

	//Get a language from either the unhandled or the handled list
	getLang := func(langIdent string) *ProcessedFile {
		if pf, ok := handledLanguages[langIdent]; ok {
			return pf
		} else if pf, ok := unhandledLanguages[langIdent]; ok {
			return pf
		}
		return nil
	}

	//Set the default language as handled
	moveLang(settings.DefaultLanguage)
	defaultLanguage := handledLanguages[settings.DefaultLanguage]
	defaultLanguage.Flags = (defaultLanguage.Flags | PFF_Language_SuccessfullyLoaded) & ^PFF_Language_SuccessNoFallbackSet

	//Handle languages with errors or that use the default fallback language
	hasErrors := false
	for _, langIdent := range getMapKeys(unhandledLanguages) {
		pf := unhandledLanguages[langIdent]
		//Remove errored languages
		if pf.Lang == nil || pf.Err != nil || (pf.Flags&PFF_Language_SuccessNoFallbackSet) == 0 {
			moveLang(langIdent)
		} else
		//Set languages that use the default fallback
		if len(pf.Lang.FallbackName()) == 0 {
			if err := pf.Lang.SetFallback(defaultLanguage.Lang); err != nil {
				pf.Err = fmt.Errorf("Fallback “%s” had error while setting: %s", defaultLanguage.LangIdentifier, err.Error())
				hasErrors = true
			} else {
				pf.Flags = (pf.Flags | PFF_Language_SuccessfullyLoaded) & ^PFF_Language_SuccessNoFallbackSet
			}
			moveLang(langIdent)
		} else
		//Add errors for languages that have non-existent fallbacks
		if fallback := getLang(pf.Lang.FallbackName()); fallback == nil {
			pf.Err = fmt.Errorf("Fallback “%s” does not exist", pf.Lang.FallbackName())
			hasErrors = true
			moveLang(langIdent)
		}
	}

	//Set the fallback languages in place
	for len(unhandledLanguages) > 0 {
		//Set fallbacks on languages whose fallbacks have been handled
		numProcessedThisIteration := 0
		for _, langIdent := range getMapKeys(unhandledLanguages) {
			pf := unhandledLanguages[langIdent]
			if fb, ok := handledLanguages[pf.Lang.FallbackName()]; ok {
				if err := pf.Lang.SetFallback(fb.Lang); err != nil {
					pf.Err = fmt.Errorf("Fallback “%s” had error while setting: %s", pf.Lang.FallbackName(), err.Error())
					hasErrors = true
				} else {
					pf.Flags = (pf.Flags | PFF_Language_SuccessfullyLoaded) & ^PFF_Language_SuccessNoFallbackSet
				}
				moveLang(langIdent)
				numProcessedThisIteration++
			}
		}

		//If there are no more languages whose fallbacks can be set, add errors to them
		if numProcessedThisIteration == 0 {
			for _, langIdent := range getMapKeys(unhandledLanguages) {
				pf := unhandledLanguages[langIdent]
				pf.Err = fmt.Errorf("Language “%s” fallback “%s” could not be set", langIdent, pf.Lang.FallbackName())
				hasErrors = true
				moveLang(langIdent)
			}
		}
	}

	//Return if errors exist
	if hasErrors {
		return handledLanguages, errors.New("There were errors while processing fallbacks")
	}

	//Return success
	return handledLanguages, nil
}

// File processes a single language and its fallbacks (and default). It returns the resultant languages (fallbacks, default, self).
//
// The languages in the fallback chain and the default language are also processed for the returned Language objects.
func (settings *ProcessSettings) File(languageIdentifier string) (loadedLanguages ProcessedFileList, err error) {
	//Process the language, its fallbacks, and the default
	var languageLoadOrder []string
	if loadedLanguages, languageLoadOrder, err = settings.processLangAndDefault(languageIdentifier, true, false); err != nil {
		return loadedLanguages, err
	}

	//The default language automatically succeeds
	{
		pf := loadedLanguages[languageLoadOrder[0]]
		pf.Flags = (pf.Flags | PFF_Language_SuccessfullyLoaded) & ^PFF_Language_SuccessNoFallbackSet
	}

	//Set the fallback languages going backwards through the languagesToLoad list
	for i := len(languageLoadOrder) - 1; i > 0; i-- {
		pf := loadedLanguages[languageLoadOrder[i]]
		if len(pf.Lang.FallbackName()) == 0 {
			if err := pf.Lang.SetFallback(loadedLanguages[settings.DefaultLanguage].Lang); err != nil {
				pf.Err = fmt.Errorf("Error setting fallback to “%s”", settings.DefaultLanguage)
				return loadedLanguages, fmt.Errorf("Error setting “%s” fallback to “%s”", pf.LangIdentifier, settings.DefaultLanguage)
			}
		} else if err := pf.Lang.SetFallback(loadedLanguages[pf.Lang.FallbackName()].Lang); err != nil {
			pf.Err = fmt.Errorf("Error setting fallback to “%s”", pf.Lang.FallbackName())
			return loadedLanguages, fmt.Errorf("Error setting “%s” fallback to “%s”", pf.LangIdentifier, pf.Lang.FallbackName())
		}

		//Set as successfully loaded
		pf.Flags = (pf.Flags | PFF_Language_SuccessfullyLoaded) & ^PFF_Language_SuccessNoFallbackSet
	}

	//Return success
	return loadedLanguages, nil
}

// FileNoReturn processes a single language.
//
// The default language will also need to be processed for the dictionary, but will only have the dictionary written out for it if it needs updating.
//
// The languages in the fallback chain will not be processed. Because of this, there will be no Language objects returned.
func (settings *ProcessSettings) FileNoReturn(languageIdentifier string) error {
	//Process the language and the default only
	_, _, err := settings.processLangAndDefault(languageIdentifier, false, false)
	return err
}

// FileCompileOnly processes a single translation text file. It does not attempt to look at fallbacks, default languages, or already-compiled files.
//
// This will only work if a compiled dictionary already exists.
func (settings *ProcessSettings) FileCompileOnly(languageIdentifier string) error {
	//Process the language only
	_, _, err := settings.processLangAndDefault(languageIdentifier, false, true)
	return err
}

//------------------Combined processing for the above functions-----------------

func (settings *ProcessSettings) checkSettings() error {
	//Check default language name
	var errs []string
	if !regexp.MustCompile(`^[a-z]{2,3}(-[a-z]{2,3})?$`).MatchString(strings.ToLower(settings.DefaultLanguage)) {
		errs = append(errs, fmt.Sprintf("Invalid default language identifier: %s", settings.DefaultLanguage))
	}

	//Confirm a directory path is valid and make sure the path ends in a forward slash
	checkDir := func(dirPath, dirName string) string {
		//Make sure the path ends in a forward slash
		if len(dirPath) == 0 || dirPath[len(dirPath)-1] != '/' {
			dirPath = dirPath + string('/')
		}

		//Confirm directory path is valid
		if info, err := os.Stat(dirPath); err != nil {
			errs = append(errs, fmt.Sprintf("Directory “%s” at “%s” could not be opened: %s", dirName, dirPath, err.Error()))
		} else if !info.IsDir() {
			errs = append(errs, fmt.Sprintf("Tried to read directory “%s” at “%s” but it is not a directory", dirName, dirPath))
		}

		return dirPath
	}

	//Check input and output directories
	settings.InputPath = checkDir(settings.InputPath, "Input path")
	if settings.OutputGoDictionary {
		settings.GoOutputPath = checkDir(settings.GoOutputPath, "Go dictionary path")
	}
	if settings.OutputCompiled {
		settings.CompiledOutputPath = checkDir(settings.CompiledOutputPath, "Compiled output path")
	}

	//Handle if there are errors
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	//Return success
	return nil
}

func (settings *ProcessSettings) processFile(pf *ProcessedFile, compiledDictionaryLoadOnly bool) error {
	//Constants for errors
	type errAction string
	type errFileType string
	//goland:noinspection GoSnakeCaseUsage
	const (
		ea_get            errAction   = "get"
		ea_open           errAction   = "open"
		ea_load           errAction   = "load"
		ea_save           errAction   = "save"
		eft_comp_dict     errFileType = "compiled dictionary file"
		eft_comp_var_dict errFileType = "compiled variable dictionary file"
		eft_comp_lang     errFileType = "compiled translation file"
		eft_lang          errFileType = "language file"
	)

	//The most common error return
	couldNotErr := func(action errAction, identifier errFileType, filename string, err error) error {
		if err == nil {
			return fmt.Errorf("Could not %s %s “%s”", action, identifier, filename)
		}
		return fmt.Errorf("Could not %s %s “%s”: %s", action, identifier, filename, err.Error())
	}

	//Load the compiled dictionary
	compiledFileExt := cond(settings.CompressCompiled, GTR_Extension_Compressed, GTR_Extension_Uncompressed)
	loadCompiledDictionary := func() (bool, error) {
		{
			var dictFile *os.File
			dictFileName := DictionaryFileBase + compiledFileExt
			if dictInfo, err := os.Stat(settings.CompiledOutputPath + dictFileName); err != nil || dictInfo.IsDir() {
				return false, nil
			} else if dictFile, err = os.Open(settings.CompiledOutputPath + dictFileName); err != nil {
				return false, nil
			}
			defer func() { _ = dictFile.Close() }()

			//Load the dictionary. Return if error occurs
			if err, ok := translate.LF_GTR.LoadDictionary(dictFile, settings.CompressCompiled); !ok {
				return false, couldNotErr(ea_load, eft_comp_dict, dictFileName, err)
			}
		}

		//Load the dictionary variables. If failure, then do not try to load compiled file
		dictVarFailed := true
		defer func() {
			if dictVarFailed {
				translate.LanguageFile(translate.LF_GTR).ClearCurrentDictionary()
			}
		}()
		var dictVarFile *os.File
		dictVarFileName := VarDictionaryFileBase + compiledFileExt
		if dictVarInfo, err := os.Stat(settings.CompiledOutputPath + dictVarFileName); err != nil || dictVarInfo.IsDir() {
			return false, nil
		} else if dictVarFile, err = os.Open(settings.CompiledOutputPath + dictVarFileName); err != nil {
			return false, nil
		}
		defer func() { _ = dictVarFile.Close() }()
		if err := translate.LF_GTR.LoadDictionaryVars(dictVarFile, settings.CompressCompiled); err != nil {
			return false, couldNotErr(ea_load, eft_comp_var_dict, dictVarFileName, err)
		}
		dictVarFailed = false
		return true, nil
	}

	//Attempt to load a compiled version
	loadCompiled := func(fileName string) (bool, error) {
		//Open the file
		f, err := os.Open(settings.CompiledOutputPath + fileName)
		if err != nil {
			return false, nil
		}
		defer func() { _ = f.Close() }()

		//Read the dictionary file if not already loaded
		if !translate.LanguageFile(translate.LF_YAML).HasCurrentDictionary() {
			//Attempt to load the file. If failure, then do not try to load compiled file
			if success, err := loadCompiledDictionary(); !success {
				return success, err
			}
		}

		//Load the language
		pf.Flags = (pf.Flags | PFF_Load_Compiled) & ^PFF_Load_NotAttempted
		if pf.LangIdentifier == settings.DefaultLanguage {
			pf.Lang, err = translate.LF_GTR.LoadDefault(f, settings.CompressCompiled)
		} else {
			pf.Lang, err = translate.LF_GTR.Load(f, settings.CompressCompiled)
		}

		//If ErrDictionaryDoesNotMatch then return as failed without error
		if err != nil && regexp.MustCompile(`^@\d+ `+regexp.QuoteMeta(translate.ErrDictionaryDoesNotMatch)+`$`).MatchString(err.Error()) {
			pf.Flags = (pf.Flags | PFF_Load_NotAttempted) & ^PFF_Load_Compiled
			return false, nil
		}

		//Return error
		if err != nil {
			pf.Flags |= PFF_Error_DuringProcessing
			return false, couldNotErr(ea_load, eft_comp_lang, pf.LangIdentifier+compiledFileExt, err)
		}

		//Make sure the language identifier matches what’s in the file
		if pf.Lang.LanguageIdentifier() != pf.LangIdentifier {
			pf.Flags |= PFF_Error_DuringProcessing
			return false, fmt.Errorf("Compiled translation file “%s” language identifier “%s” does not match", pf.LangIdentifier+compiledFileExt, pf.Lang.LanguageIdentifier())
		}

		//Return success
		pf.Flags |= PFF_Language_SuccessNoFallbackSet
		return true, nil
	}

	//Handle forced dictionary load
	if compiledDictionaryLoadOnly && pf.LangIdentifier == settings.DefaultLanguage {
		//No need to load the dictionary if already loaded
		if !translate.LanguageFile(translate.LF_YAML).HasCurrentDictionary() {
			//Attempt to load the file. If failure, then return error
			if success, err := loadCompiledDictionary(); !success {
				return cond(err == nil, errors.New("Loading compiled dictionary failed"), err)
			}
		}
		pf.Flags = (pf.Flags | PFF_Language_IsDefault | PFF_Load_Compiled | PFF_Language_SuccessfullyLoaded) & ^PFF_Load_NotAttempted
		return nil
	}

	//If there is a newer (or equal timestamp) compiled version of the file use it instead
	if fileInfo, err := os.Stat(settings.InputPath + pf.InputFileName); err != nil || fileInfo.IsDir() {
		return couldNotErr(ea_get, "file info for", pf.InputFileName, nil)
	} else if settings.IgnoreTimestamps {
		//Do not continue if/else chain if we are ignoring timestamps
	} else if compFileInfo, err := os.Stat(settings.CompiledOutputPath + pf.LangIdentifier + compiledFileExt); err == nil && !compFileInfo.IsDir() && !compFileInfo.ModTime().Before(fileInfo.ModTime()) {
		if success, err := loadCompiled(compFileInfo.Name()); err != nil {
			return err
		} else if success {
			return nil
		}
	}

	//Open the file for reading
	{
		var f *os.File
		pf.Flags &= ^PFF_Load_NotAttempted
		if _f, err := os.Open(settings.InputPath + pf.InputFileName); err != nil {
			pf.Flags |= PFF_Load_NotFound
			return couldNotErr(ea_open, eft_lang, pf.InputFileName, err)
		} else {
			f = _f
		}
		defer func() { _ = f.Close() }()

		//Read the language file
		var e error
		switch ext := pf.InputFileName[len(pf.LangIdentifier)+1:]; ext {
		case YAML_Extension:
			pf.Flags |= PFF_Load_YAML
			if pf.LangIdentifier == settings.DefaultLanguage {
				pf.Lang, pf.Warnings, e = translate.LF_YAML.LoadDefault(f, settings.AllowBigStrings)
			} else {
				pf.Lang, pf.Warnings, e = translate.LF_YAML.Load(f, settings.AllowBigStrings)
			}
		case JSON_Extension:
			pf.Flags |= PFF_Load_JSON
			loader := cond(settings.AllowJSONTrailingComma, translate.LF_JSON_AllowTrailingComma, translate.LF_JSON)
			if pf.LangIdentifier == settings.DefaultLanguage {
				pf.Lang, pf.Warnings, e = loader.LoadDefault(f, settings.AllowBigStrings)
			} else {
				pf.Lang, pf.Warnings, e = loader.Load(f, settings.AllowBigStrings)
			}
		default:
			pf.Flags |= PFF_Load_NotFound
			return fmt.Errorf("Extension “%s” for file “%s” must be %s", ext, pf.InputFileName, strings.Join([]string{YAML_Extension, JSON_Extension}, " or "))
		}

		//If there is an error, return it
		if e != nil {
			pf.Flags |= PFF_Error_DuringProcessing
			return couldNotErr(ea_load, eft_lang, pf.InputFileName, e)
		}
	}

	//Make sure the language identifier matches what’s in the file
	if pf.Lang.LanguageIdentifier() != pf.LangIdentifier {
		pf.Flags |= PFF_Error_DuringProcessing
		return fmt.Errorf("Language file “%s” language identifier “%s” does not match", pf.InputFileName, pf.Lang.LanguageIdentifier())
	}

	//Output the resultant files for the default language
	if pf.LangIdentifier == settings.DefaultLanguage {
		if settings.OutputGoDictionary {
			if err, numUpdated := pf.Lang.SaveGoDictionaries(settings.GoOutputPath, settings.GoDictHeader); err != nil {
				return fmt.Errorf("Could not save go dictionaries: %s", err.Error())
			} else if numUpdated > 0 {
				pf.Flags |= PFF_OutputSuccess_GoDictionaries
			}
		}
		if settings.OutputCompiled {
			//The compiled dictionary
			{
				dictFileName := DictionaryFileBase + compiledFileExt
				if fc, err := os.Create(settings.CompiledOutputPath + dictFileName); err != nil {
					return couldNotErr(ea_open, eft_comp_dict, dictFileName, err)
				} else {
					defer func() { _ = fc.Close() }()
					if err := pf.Lang.SaveGTRDict(fc, settings.CompressCompiled); err != nil {
						return couldNotErr(ea_save, eft_comp_dict, dictFileName, err)
					}
				}
			}

			//The compiled variable dictionary
			dictFileName := VarDictionaryFileBase + compiledFileExt
			if fc, err := os.Create(settings.CompiledOutputPath + dictFileName); err != nil {
				return couldNotErr(ea_open, eft_comp_var_dict, dictFileName, err)
			} else {
				defer func() { _ = fc.Close() }()
				if err := pf.Lang.SaveGTRVarsDict(fc, settings.CompressCompiled); err != nil {
					return couldNotErr(ea_save, eft_comp_var_dict, dictFileName, err)
				}
			}

			pf.Flags |= PFF_OutputSuccess_CompiledDictionary
		}
	}

	//Output the compiled translation file
	if settings.OutputCompiled {
		outFileName := pf.LangIdentifier + compiledFileExt
		if fc, err := os.Create(settings.CompiledOutputPath + outFileName); err != nil {
			return couldNotErr(ea_open, eft_comp_lang, outFileName, err)
		} else {
			defer func() { _ = fc.Close() }()
			if err := pf.Lang.SaveGTR(fc, settings.CompressCompiled); err != nil {
				return couldNotErr(ea_save, eft_comp_lang, outFileName, err)
			}
		}
		pf.Flags |= PFF_OutputSuccess_CompiledLanguage
	}

	//Return success
	pf.Flags |= PFF_Language_SuccessNoFallbackSet
	return nil
}

func (settings *ProcessSettings) processLangAndDefault(languageIdentifier string, processFallbacks, forceCompiledDictionaryLoad bool) (loadedLanguages ProcessedFileList, languageLoadOrder []string, err error) {
	//Check and update the settings
	if err := settings.checkSettings(); err != nil {
		return nil, nil, err
	}

	//The list of languages that still need to be processed
	loadedLanguages = make(ProcessedFileList)
	languageLoadOrder = []string{settings.DefaultLanguage}
	if languageIdentifier != settings.DefaultLanguage {
		languageLoadOrder = append(languageLoadOrder, languageIdentifier)
	}

FileLoop:
	for curLangIndex := 0; curLangIndex < len(languageLoadOrder); curLangIndex++ {
		//Add the ProcessedFile to the return list
		curLang := languageLoadOrder[curLangIndex]
		pf := &ProcessedFile{
			LangIdentifier: curLang,
			Flags:          PFF_Load_NotAttempted,
		}
		loadedLanguages[curLang] = pf

		//Attempt to find the language file from the possible translation text file extensions
		for _, ext := range []string{YAML_Extension, JSON_Extension} {
			//Find if there is a matching translation text file extension
			if fInfo, err := os.Stat(settings.InputPath + curLang + "." + ext); err != nil || fInfo.IsDir() {
				continue
			} else {
				pf.InputFileName = fInfo.Name()
			}

			//If forcing a compiled dictionary load on the default language, do so now
			//Only if the requested language is not the default though
			if forceCompiledDictionaryLoad && curLang == settings.DefaultLanguage && languageIdentifier != settings.DefaultLanguage {
				if pf.Err = settings.processFile(pf, true); pf.Err != nil {
					return loadedLanguages, languageLoadOrder, pf.Err
				}
				continue FileLoop
			}

			//Process the language and handle errors
			if pf.Err = settings.processFile(pf, false); pf.Err != nil {
				return loadedLanguages, languageLoadOrder, pf.Err
			}

			//Add the fallback language to be processed
			if !processFallbacks || curLang == settings.DefaultLanguage || pf.Lang.FallbackName() == settings.DefaultLanguage || len(pf.Lang.FallbackName()) == 0 {
				//No language to add
			} else if _, ok := loadedLanguages[pf.Lang.FallbackName()]; ok {
				return loadedLanguages, languageLoadOrder, fmt.Errorf("File “%s” has a fallback loop on “%s” starting from “%s”", pf.InputFileName, pf.Lang.FallbackName(), languageIdentifier)
			} else {
				languageLoadOrder = append(languageLoadOrder, pf.Lang.FallbackName())
			}

			continue FileLoop
		}

		//Return error if file not found
		pf.Flags = (pf.Flags | PFF_Load_NotFound) & ^PFF_Load_NotAttempted
		return loadedLanguages, languageLoadOrder, fmt.Errorf("File for “%s” was not found starting from “%s”", curLang, languageIdentifier)
	}

	//Return success
	return loadedLanguages, languageLoadOrder, nil
}
