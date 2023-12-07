//Command line interface
//go:build !gol10n_read_compiled_only

/*
Package main is the command line interface to the “translate” package, which is highly space and memory optimized l10n (localization) library.

gol10n.exe [Mode] [flags]:

Modes (Mutually exclusive):

	 Directory mode: [No arguments given]
	    Processes all files in the “InputPath” directory
	    Can be used in conjunction with -w
	 File mode: [arg1=language identifier]
	    Processes a single language file
	    The default language will need to be processed if a compiled dictionary does not exist
	    Can be used in conjunction with -s or -f

	-s, --single-file               Mode=File. The default language will not be processed
	                                This will only work if a compiled dictionary already exists
	-f, --fallbacks                 Mode=File. Also process the language’s fallback files
	-w, --watch                     Mode=Directory. Continually watches the directory for relevant changes
	                                Only processes and updates the necessary files when a change is detected
	    --create-settings           Create the default settings-gol10n.json file
	-h, --help                      This help prompt

File flags (Modify how non-translation-text-files are interacted with):

	-d, --go-dictionary[=false]     Output the go dictionary files when processing the default language (default true)
	-c, --output-compiled[=false]   Output the compiled translation files and dictionary (default true)
	-i, --ignore-timestamps         Always read from translation text files [ignore compiled files even if they are newer]

The following are for overriding settings from settings-gol10n.json. If not given, the values from the settings file will be used:

	-l, --default-language string   The identifier for the default language
	-p, --input-path string         The directory with the translation text files
	-g, --go-path string            The directory to output the generated Go dictionary files to
	                                Each namespace gets its own directory and file in the format “$NamespaceName/translationIDs.go”
	-o, --output-path string        The directory to output the compiled binary translation files to
	                                Each language gets its own .gtr or .gtr.gz (gzip compressed) file
	-m, --compress-compiled         Whether the compiled binary translation files are saved as .gtr or .gtr.gz (gzip compressed)
	-b, --allow-big-strings         If translation strings can be larger than 64KB
	                                If true, and a large translation is found, then compiled binary files will become larger
	-j, --allow-json-comma          If JSON files can have trailing commas. If true, a sanitization process is ran over the JSON

Command line display modifiers:
See using_in_go.md#ProcessedFile  Mode=Directory, -w, -f

	-t, --table[=false]             Output an ascii table of the processed languages and their flags (default true)
	-v, --verbose                   Output a list of processed files and their processing flags
	-x, --warnings[=false]          Output a list of warnings when processing non-default language translation files (default true)
*/
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dakusan/gol10n/translate/execute"
	"github.com/dakusan/gol10n/translate/watch"
	"github.com/spf13/pflag"
	"os"
	"regexp"
	"strings"
)

func main() {
	//I wish there was a way to let the process naturally return an error code without forcing the exit with os.Exit()
	//Because of this I decided to not return success values
	mainWrapper()
}

// Returns if successful
func mainWrapper() bool {
	//Mode flags
	flagSingleFile := pflag.BoolP("single-file", "s", false, "Mode=File. The default language will not be processed\nThis will only work if a compiled dictionary already exists")
	flagFallbackFiles := pflag.BoolP("fallbacks", "f", false, "Mode=File. Also process the language’s fallback files")
	flagWatchFiles := pflag.BoolP("watch", "w", false, "Mode=Directory. Continually watches the directory for relevant changes\nOnly processes and updates the necessary files when a change is detected")
	flagCreateSettingsFile := pflag.Bool("create-settings", false, "Create the default "+execute.SettingsFileName+" file")
	flagShowHelp := pflag.BoolP("help", "h", false, "This help prompt")

	//Settings receiver with defaults
	settings := execute.ProcessSettings{
		DefaultLanguage:    "en-US",
		InputPath:          "translations",
		GoOutputPath:       "const",
		CompiledOutputPath: "compiled",
		CompressCompiled:   true,
		OutputGoDictionary: true,
		OutputCompiled:     true,
	}

	//Settings overrides
	type settingInfo struct {
		name, fixedName string
		flagValue       any
		settingPointer  any
	}
	settingOverrides := make(map[string]settingInfo)
	addSetting := func(shortLetter byte, name string, settingPointer any, usageText string) {
		//Lower case name and add dash where there were upper case letters
		const upperToLowerOrOp = 32
		fixedName := []byte(name)
		fixedName[0] = fixedName[0] | upperToLowerOrOp
		fixedName = regexp.MustCompile(`[a-z][A-Z]`).ReplaceAllFunc(fixedName, func(b []byte) []byte {
			return []byte(fmt.Sprintf("%c-%c", b[0], b[1]|upperToLowerOrOp))
		})
		sFixedName := string(fixedName)

		myInfo := settingInfo{name, sFixedName, nil, settingPointer}
		switch settingPointer.(type) {
		case *bool:
			myInfo.flagValue = pflag.BoolP(sFixedName, string(shortLetter), false, usageText)
		case *string:
			myInfo.flagValue = pflag.StringP(sFixedName, string(shortLetter), "", usageText)
		default:
			panic("Unreachable code")
		}

		settingOverrides[name] = myInfo
	}

	//Create file flags
	addSetting('d', "GoDictionary", &settings.OutputGoDictionary, "Output the go dictionary files when processing the default language")
	addSetting('c', "OutputCompiled", &settings.OutputCompiled, "Output the compiled translation files and dictionary")
	addSetting('i', "IgnoreTimestamps", &settings.IgnoreTimestamps, "Always read from translation text files [ignore compiled files even if they are newer]")

	//Settings flags
	addSetting('l', "DefaultLanguage", &settings.DefaultLanguage, "The identifier for the default language")
	addSetting('p', "InputPath", &settings.InputPath, "The directory with the translation text files")
	addSetting('g', "GoPath", &settings.GoOutputPath, "The directory to output the generated Go dictionary files to\nEach namespace gets its own directory and file in the format “$NamespaceName/translationIDs.go”")
	addSetting('o', "OutputPath", &settings.CompiledOutputPath, "The directory to output the compiled binary translation files to\nEach language gets its own .gtr or .gtr.gz (gzip compressed) file")
	addSetting('m', "CompressCompiled", &settings.CompressCompiled, "Whether the compiled binary translation files are saved as .gtr or .gtr.gz (gzip compressed)")
	addSetting('b', "AllowBigStrings", &settings.AllowBigStrings, "If translation strings can be larger than 64KB\nIf true, and a large translation is found, then compiled binary files will become larger")
	addSetting('j', "AllowJsonComma", &settings.AllowJSONTrailingComma, "If JSON files can have trailing commas. If true, a sanitization process is ran over the JSON")

	//Output flags
	flagShowTable := pflag.BoolP("table", "t", true, "Output an ascii table of the processed languages and their flags")
	flagShowProcessedFlags := pflag.BoolP("verbose", "v", false, "Output a list of processed files and their processing flags")
	flagShowProcessedWarnings := pflag.BoolP("warnings", "x", true, "Output a list of warnings when processing non-default language translation files")
	for _, flagName := range []string{"go-dictionary", "output-compiled", "table", "warnings"} {
		pflag.Lookup(flagName).NoOptDefVal = "false"
		pflag.Lookup(flagName).DefValue = "true"

	}

	//Set up help prompt
	stdErr := func(str string) bool {
		_, _ = fmt.Fprintln(os.Stderr, str)
		return false
	}
	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		//Add title above a flag
		flagsSection := pflag.CommandLine.FlagUsages()
		titleFlagSection := func(shortLetter byte, titleLine string, args ...interface{}) {
			flagsSection = regexp.MustCompile(`(?m)^\s*-`+string(shortLetter)).ReplaceAllStringFunc(flagsSection, func(str string) string {
				return fmt.Sprintf("\n"+titleLine+"\n%s", append(args, str)...)
			})
		}

		//Add the titles
		titleFlagSection('d', "File flags (Modify how non-translation-text-files are interacted with):")
		titleFlagSection('l', "The following are for overriding settings from %s. If not given, the values from the settings file will be used:", execute.SettingsFileName)
		titleFlagSection('t', "Command line display modifiers:\nSee using_in_go.md#ProcessedFile  Mode=Directory, -w, -f\n")

		//Modes information
		modesStrings := []string{
			"   Directory mode: [No arguments given]\n      Processes all files in the “InputPath” directory\n      Can be used in conjunction with -w",
			"   File mode: [arg1=language identifier]\n      Processes a single language file\n      The default language will need to be processed if a compiled dictionary does not exist\n      Can be used in conjunction with -s or -f",
		}

		FullMessage := fmt.Sprintf(
			"%s [Mode] [flags]:\n\nModes (Mutually exclusive):\n%s\n\n%s",
			regexp.MustCompile(`^.*[/\\]`).ReplaceAllString(os.Args[0], ""),
			strings.Join(modesStrings, "\n"),
			flagsSection,
		)

		stdErr(FullMessage)
	}
	pflag.ErrHelp = errors.New("")

	//Run flags parsing
	pflag.Parse()

	//If help is requested
	if *flagShowHelp {
		pflag.Usage()
		return false
	}

	//If settings file creation is requested
	if *flagCreateSettingsFile {
		var f *os.File
		var err error
		if f, err = os.Create(execute.SettingsFileName); err != nil {
			return stdErr(fmt.Sprintf("Error opening %s: %s", execute.SettingsFileName, err.Error()))
		}
		defer func() { _ = f.Close() }()
		e := json.NewEncoder(f)
		e.SetIndent("", "\t")
		if err := e.Encode(settings); err != nil {
			return stdErr(fmt.Sprintf("Error compiling settings to %s: %s", execute.SettingsFileName, err.Error()))
		}
		return stdErr("Settings file created")
	}

	//Read the settings file
	if settingsText, err := os.ReadFile(execute.SettingsFileName); err != nil {
		return stdErr(fmt.Sprintf("Could not read settings file “%s”: %s", execute.SettingsFileName, err.Error()))
	} else if err := json.Unmarshal(settingsText, &settings); err != nil {
		return stdErr(fmt.Sprintf("Could not read settings file “%s”: %s", execute.SettingsFileName, err.Error()))
	}

	//Read the flags into settings
	for _, s := range settingOverrides {
		if !pflag.Lookup(s.fixedName).Changed {
			continue
		}
		switch v := s.settingPointer.(type) {
		case *bool:
			*v = *s.flagValue.(*bool)
		case *string:
			*v = *s.flagValue.(*string)
		default:
			panic("Unreachable code")
		}
	}

	//Make sure no mode mutually exclusive flags are set together
	{
		count := 0
		for _, b := range []*bool{flagSingleFile, flagFallbackFiles, flagWatchFiles} {
			if *b {
				count++
			}
		}
		if count > 1 {
			return stdErr(fmt.Sprintf("-s -f -w flags cannot be used together"))
		}
	}

	//Make sure we are in the proper mode for the mode flags
	hasLangIdentifier := pflag.NArg() > 0
	if hasLangIdentifier && *flagWatchFiles {
		return stdErr(fmt.Sprintf("-w flag cannot be used in mode=File"))
	} else if !hasLangIdentifier && (*flagSingleFile || *flagFallbackFiles) {
		return stdErr(fmt.Sprintf("-s and -f flags cannot be used in mode=Directory"))
	}

	//Run the requested mode
	languageIdentifier := pflag.Arg(0)
	switch {
	case *flagSingleFile:
		if err := settings.FileCompileOnly(languageIdentifier); err != nil {
			fmt.Println(err.Error())
			return false
		} else {
			fmt.Println("Success")
			return true
		}
	case *flagFallbackFiles:
		dirData, err := settings.File(languageIdentifier)
		outputDirData(dirData, err, *flagShowTable, *flagShowProcessedFlags, *flagShowProcessedWarnings)
		return err == nil
	case *flagWatchFiles:
		ret := watch.Execute(&settings)
		for msg := range ret {
			switch msg.Type {
			case watch.WR_Message:
				fmt.Println(msg.Message)
			case watch.WR_ProcessedFile:
				if msg.Err != nil {
					fmt.Printf("Processing file “%s”: %s\n", msg.Message, msg.Err.Error())
				} else {
					fmt.Printf("Processing file “%s”: %s\n", msg.Message, "Success")
				}
			case watch.WR_ProcessedDirectory:
				fmt.Println("Finished processing input directory")
				outputDirData(msg.Files, msg.Err, *flagShowTable, *flagShowProcessedFlags, *flagShowProcessedWarnings)
			case watch.WR_ErroredOut:
				fmt.Printf("Fatal error, exiting: %s\n", msg.Err)
				return true
			case watch.WR_CloseRequested:
				fmt.Println("Exiting watch")
				return true
			}
		}
		panic("Unreachable code")
	case hasLangIdentifier:
		if err := settings.FileNoReturn(languageIdentifier); err != nil {
			fmt.Println(err.Error())
			return false
		} else {
			fmt.Println("Success")
			return true
		}
	case !hasLangIdentifier:
		dirData, err := settings.Directory()
		outputDirData(dirData, err, *flagShowTable, *flagShowProcessedFlags, *flagShowProcessedWarnings)
		return err == nil
	default:
		panic("Unreachable code")
	}
}

func outputDirData(ret execute.ProcessedFileList, err error, showTable, showProcessedFlags, showWarnings bool) {
	//Output errors
	if err != nil {
		fmt.Println("Errors: " + err.Error())
		for _, pf := range ret {
			if pf.Err != nil {
				fmt.Printf("Lang “%s”: %s\n", pf.LangIdentifier, pf.Err.Error())
			}
		}
		fmt.Println(strings.Repeat("-", 80))
	} else {
		fmt.Println("Success")
	}

	//Print the flag table
	if len(ret) != 0 && showTable {
		fmt.Println(strings.Join(ret.CreateFlagTable(), "\n"))
	}

	//Print the processed flags
	if len(ret) != 0 && showProcessedFlags {
		for _, pf := range ret {
			getFlags := make([]string, 0, len(execute.ProcessedFileFlagNames))
			for _, f := range execute.ProcessedFileFlagNames {
				if pf.Flags&f.Flag != 0 {
					getFlags = append(getFlags, f.Name)
				}
			}
			fmt.Printf("%s: %s\n", pf.LangIdentifier, strings.Join(getFlags, ", "))
		}
	}

	//Print warnings
	if showWarnings {
		isFirstWarning := true
		for _, pf := range ret {
			if len(pf.Warnings) == 0 {
				continue
			}
			if isFirstWarning {
				fmt.Println(strings.Repeat("-", 80))
				fmt.Println("Warnings:")
				isFirstWarning = false
			}

			fmt.Printf("Lang “%s”: %s\n", pf.LangIdentifier, strings.Join(pf.Warnings, "\n"))
		}
	}
}
