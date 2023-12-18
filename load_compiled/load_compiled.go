// Package load_compiled loads compiled files (and fallbacks) from the language identifier
package load_compiled

import (
	"fmt"
	"github.com/dakusan/gol10n/execute"
	"github.com/dakusan/gol10n/translate"
	"os"
)

// LoadDefault loads the compiled dictionary and default language
func LoadDefault(compiledDirectoryPath, defaultLanguageIdentifier string, isCompressed bool) (*translate.Language, error) {
	//Get the fixed compiled path and the file extensions
	compiledDirectoryPath = addSlash(compiledDirectoryPath)
	fileExt := execute.GTR_Extension_Uncompressed
	if isCompressed {
		fileExt = execute.GTR_Extension_Compressed
	}

	//Return the part that the error occurred at
	tError := func(partName string, err error) error {
		return fmt.Errorf("%s error: %s", partName, err.Error())
	}

	//Load the dictionary
	if f, err := os.Open(compiledDirectoryPath + execute.DictionaryFileBase + fileExt); err != nil {
		return nil, tError("Translation dictionary", err)
	} else {
		defer func() { _ = f.Close() }()
		if err, ok := translate.LF_GTR.LoadDictionary(f, isCompressed); !ok {
			return nil, tError("Translation dictionary", err)
		}
	}

	//Load the default language
	if l, err := loadLanguage(compiledDirectoryPath+defaultLanguageIdentifier+fileExt, true, isCompressed); err != nil {
		return nil, tError("Default language", err)
	} else {
		return l, nil
	}
}

// Load loads the language and its fallbacks. Dictionary must be loaded first (Through LoadDefault())
func Load(compiledDirectoryPath, langIdentifier string, isCompressed bool, defaultLanguage *translate.Language) (*translate.Language, error) {
	//If the requested language is also the default language, then nothing to do
	if langIdentifier == defaultLanguage.LanguageIdentifier() {
		return defaultLanguage, nil
	}

	//Get the fixed compiled path and the file extensions
	compiledDirectoryPath = addSlash(compiledDirectoryPath)
	fileExt := execute.GTR_Extension_Uncompressed
	if isCompressed {
		fileExt = execute.GTR_Extension_Compressed
	}

	//Iterate over current and fallback languages
	var loadedLanguages []*translate.Language
	curLang := langIdentifier
	for {
		//Get the next language in the fallback chain
		var l *translate.Language
		if _l, err := loadLanguage(compiledDirectoryPath+curLang+fileExt, false, isCompressed); err != nil {
			return nil, fmt.Errorf("Error loading “%s” (under language “%s”): %s", curLang, langIdentifier, err.Error())
		} else {
			l = _l
		}
		loadedLanguages = append(loadedLanguages, l)

		//If the fallback for this language is not defined, or is set as the default language, then stop here
		curLang = l.FallbackName()
		if len(curLang) == 0 || curLang == defaultLanguage.LanguageIdentifier() {
			break
		}

		//Make sure the language is not already in the fallback chain
		for _, ll := range loadedLanguages {
			if ll.LanguageIdentifier() == curLang {
				return nil, fmt.Errorf("Error loading “%s” (under language “%s”): %s", curLang, langIdentifier, "fallback loop detected")
			}
		}
	}

	//Set the fallbacks on each of the languages in the chain
	loadedLanguages = append(loadedLanguages, defaultLanguage)
	for i := len(loadedLanguages) - 2; i >= 0; i-- {
		if err := loadedLanguages[i].SetFallback(loadedLanguages[i+1]); err != nil {
			return nil, fmt.Errorf(
				"Error setting fallback “%s” on “%s” (under default language “%s”): %s",
				loadedLanguages[i+1].LanguageIdentifier(),
				loadedLanguages[i].LanguageIdentifier(),
				langIdentifier, err.Error(),
			)
		}
	}

	//Store the language and return success
	return loadedLanguages[0], nil
}

func loadLanguage(path string, isDefault, isCompressed bool) (*translate.Language, error) {
	//Open the file
	var f *os.File
	var err error
	if f, err = os.Open(path); err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	//If default language
	if isDefault {
		if l, err := translate.LF_GTR.LoadDefault(f, isCompressed); err != nil {
			return nil, err
		} else {
			return l, nil
		}
	}

	//If not default language
	if l, err := translate.LF_GTR.Load(f, isCompressed); err != nil {
		return nil, err
	} else {
		return l, nil
	}
}

func addSlash(path string) string {
	if len(path) == 0 || (path[len(path)-1] != '/' && path[len(path)-1] != '\\') {
		path = path + "/"
	}
	return path
}

// Remove warnings about unused functions
func init() {
	_, _ = LoadDefault, Load
}
