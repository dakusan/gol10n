This document explains how to use this library inside your Go code.

# Generated Go dictionary files
The Go dictionary files are generated from [the dictionary](definitions.md#The-dictionary) and [the default language](definitions.md#The-default-language) into <code>[global_settings](../README.md#Settings-file).GoOutputPath</code>. They are basically enumeration lists.

Each [namespace](definitions.md#Namespaces) gets its own directory and file in the format `$NamespaceName/TranslationIDs.go`.

While regeneration is done after any changes to the [default language](definitions.md#The-default-language) [translation text file](translation_files.md), the generated go files are only saved when there is a relevant change. A file named `NamespaceHashes.json` is kept in <code>[global_settings](../README.md#Settings-file).GoOutputPath</code> to facilitate this behavior.

The format of the go files looks like the following:<br>
**File: <code>[global_settings](../README.md#Settings-file).GoOutputPath</code>/`NameSpaceExample/TranslationIDs.go`**
```go
package NameSpaceExample

import "github.com/dakusan/gol10n/translate"

//goland:noinspection NonAsciiCharacters,GoSnakeCaseUsage
const (
	//TranslationID = TranslationValue
	TranslationID translate.TransIndex = iota + 0

	//Foo天६_ = 😭Bar {{*TranslationID}} {{*_animalsGroupNames.Cow}}
	Foo天६_

	/*BorrowedNumberOfBooks = You have no books borrowed
	VariableOrder = OtherVar[VariableTranslation]*/
	BorrowedNumberOfBooks

	/*WelcomeTitle = Welcome to our hotel <b>{{.Name|-10}}</b>.\nYour stay is for {{.NumDay天s|08.2}} days. Your checkout is on {{.CheckoutDay!%x %X}} and your cost will be {{.Cost}}
	VariableOrder = Name[String], CheckoutDay[DateTime], Cost[Currency], NumDay天s[IntegerWithSymbols]*/
	WelcomeTitle
)
```
The commented translations always take the first given [Plurality rule](translation_files.md#Plurality-rules).

# Using translations in Go
`Language` objects have a combination of Get() functions to compile translations from either a **TransIndex** or a [Translation ID](definitions.md#Translation-IDs) string (with optional [namespace](definitions.md#Namespaces)).

See [here for the full list of Get() translation functions](language_get_functions.md#Get-translation-functions).

## Example “Get” translation function calls
See [examples in the README](../README.md#Example-get-translation-function-calls).

## Automatically saving and loading the language files
* [ProcessSettings](#ProcessSettings)/[ProcessedFile](#ProcessedFile) are in the `translate.execute` package
* [ReturnData/watch.Execute](#watchReturnData) are in the `translate.watch` package

### ProcessSettings
The `ProcessSettings` struct values are taken from [global_settings](../README.md#Settings-file) and are used to automatically read [translation text files](translation_files.md), [compiled translation files](definitions.md#Compiled-binary-translation-files), and [go dictionary files](#Generated-Go-dictionary-files).

[Compiled files](definitions.md#Compiled-binary-translation-files) are read from (and not written to) if their modification timestamps are newer than their [translation text files](translation_files.md) counterpart, unless `IgnoreTimestamps=true`.

The `ProcessSettings` struct also contains the following flags:

| Name               | Type | Description                                                                               |
|--------------------|------|-------------------------------------------------------------------------------------------|
| OutputGoDictionary | bool | Whether to output [go dictionary files](#Generated-Go-dictionary-files)                   |
| OutputCompiled     | bool | Whether to output compiled [.gtr](definitions.md#Compiled-binary-translation-files) files |
| IgnoreTimestamps   | bool | Whether to force outputting all files, ignoring timestamps                                |

Its functions are:
* `func (settings *ProcessSettings) Directory() (ProcessedFileList, error)`
	* Processes all files in the `InputPath` directory. It also returns the resultant languages.
	* No [ProcessedFiles](#ProcessedFile) are returned if any of the following errors occur: Directory error, language identity used more than once, default language not found
* `func (settings *ProcessSettings) File(languageIdentifier string) (loadedLanguages ProcessedFileList, err error)`
	* Processes a single language and its [fallbacks](definitions.md#Fallback-languages) (and [default](definitions.md#The-default-language)). It returns the resultant languages (fallbacks, default, self).
	* The languages in the fallback chain and the default language are also processed for the returned Language objects.
* `func (settings *ProcessSettings) FileNoReturn(languageIdentifier string) error`
	* Processes a single language.
	* The [default language](definitions.md#The-default-language) will also need to be processed for [the dictionary](definitions.md#The-dictionary), but will only have the dictionary written out for it if it needs updating.
	* The languages in the fallback chain will not be processed. Because of this, there will be no Language objects returned.
* `func (settings *ProcessSettings) FileCompileOnly(languageIdentifier string) error`
	* Processes a single [translation text file](translation_files.md). It does not attempt to look at [fallbacks](definitions.md#Fallback-languages), [default languages](definitions.md#The-default-language), or already-compiled files.
	* This will only work if a [compiled dictionary](definitions.md#Compiled-binary-translation-files) already exists.
* `func watch.Execute(settings *ProcessSettings) <-chan watch.ReturnData`
	* Processes all files in the `InputPath` directory.
	* It continually watches the directory for relevant changes in its own goroutine, and only processes and updates the necessary files when a change is detected. See [watch.ReturnData](#watchReturnData).

### ProcessedFile
Some [ProcessSettings](#ProcessSettings) functions return a `map` of `ProcessedFile` structs keyed to the [language identifier](definitions.md#Language-identifiers), which is the `ProcessedFileList` type.

```go
type ProcessedFile struct {
	LangIdentifier string
	InputFileName  string
	Warnings       []string
	Err            error
	Flags          ProcessedFileFlag
	Lang           *translate.Language //Only filled if Flags.PFF_Language_Success*
}
```

`Flags` is a set of `ProcessedFileFlag`, which are:

| Flag name                              | Short | Flag info                                                                                                                                                                                                                                                                       |
|----------------------------------------|-------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Language object info**               |
| PFF_Language_SuccessfullyLoaded        | SuLD  | If the Language object was successfully loaded and filled into ProcessedFiles and the [fallback](definitions.md#Fallback-languages) was set                                                                                                                                     |
| PFF_Language_SuccessNoFallbackSet      | SuNF  | If the Language object was loaded and filled into ProcessedFiles, but the [fallback](definitions.md#Fallback-languages) was not set                                                                                                                                             |
| PFF_Language_IsDefault                 | Defa  | If this is the [default language](definitions.md#The-default-language)                                                                                                                                                                                                          |
| **Loading state (mutually exclusive)** |
| PFF_Load_NotAttempted                  | LoNA  | File loading was not attempted because other errors occurred first                                                                                                                                                                                                              |
| PFF_Load_NotFound                      | LoNF  | File was not loaded because its [translation text file](translation_files.md) was not found                                                                                                                                                                                     |
| PFF_Load_YAML                          | LoYA  | If this was loaded from a [YAML](translation_files.md#YAML-files) [translation text file](translation_files.md)                                                                                                                                                                 |
| PFF_Load_JSON                          | LoJS  | If this was loaded from a [JSON](translation_files.md#JSON-files) [translation text file](translation_files.md)                                                                                                                                                                 |
| PFF_Load_Compiled                      | LoCo  | If this was loaded from a [.gtr](definitions.md#Compiled-binary-translation-files) file<br><sub>Note: Compression state is assumed from `ProcessSettings.CompressCompiled`</sub>                                                                                                |
| **Error information**                  |
| PFF_Error_DuringProcessing             | Er    | If errors occurred during processing                                                                                                                                                                                                                                            |
| **File output success flags**          |
| PFF_OutputSuccess_CompiledLanguage     | OuCL  | If a [.gtr file](definitions.md#Compiled-binary-translation-files) was successfully output<br><sub>Note: Only when `ProcessSettings.OutputCompiled`<br>Note: Compression state is assumed from `ProcessSettings.CompressCompiled`</sub>                                         |
| PFF_OutputSuccess_CompiledDictionary   | OuCD  | If a [.gtr dictionary file](definitions.md#Compiled-binary-translation-files) was successfully output<br><sub>Note: Only when `ProcessSettings.OutputCompiled` and `PFF_Language_IsDefault`<br>Note: Compression state is assumed from `ProcessSettings.CompressCompiled`</sub> |
| PFF_OutputSuccess_GoDictionaries       | OuGD  | If one or more [go dictionary files](#Generated-Go-dictionary-files) was successfully output<br><sub>Note: Only when `ProcessSettings.OutputGoDictionary` and `PFF_Language_IsDefault`</sub>                                                                                    |

A function is available, `ProcessedFileList.CreateFlagTable() []string` which creates an aligned ascii table that shows which flags are set on which `ProcessedFile`s. The row headers are the `Short` in the above table, and the column headers are the [language identifier](definitions.md#Language-identifiers).

### watch.ReturnData
The `watch.Execute()` function (listed under [ProcessSettings](#ProcessSettings)) returns what’s happening through a channel of `watch.ReturnData` type.
```go
package watch
type ReturnData struct {
	Type    ReturnType
	Files   execute.ProcessedFileList //Only on ReturnType=WR_ProcessedDirectory
	Err     error                     //Only on ReturnType=WR_ProcessedDirectory or WR_ProcessedFile or WR_ErroredOut
	Message string                    //Only on ReturnType=WR_Message or WR_ProcessedFile
}

type ReturnType int
const (
	WR_Message  ReturnType //An informative message is being sent
	WR_ProcessedDirectory  //Directory() was called due to initialization or default language update
	WR_ProcessedFile       //A single file was updated. Message contains the filename. Error is filled on error.
	WR_ErroredOut          //The watch could not be started or has closed
)

func Execute(settings *execute.ProcessSettings) <-chan ReturnData {}
```

## Manually loading the language files
### Load functions
* Translation text files:
	* **LanguageTextFile**: `LF_YAML`, `LF_JSON`, `LF_JSON_AllowTrailingComma`
		* `func (lf LanguageTextFile) Load(r io.Reader, allowBigStrings bool) (retLang *Language, retWarnings []string, retErrors error)`
			* Loads a [text](translation_files.md) language file (either [YAML](translation_files.md#YAML-files) or [JSON](translation_files.md#JSON-files)).
			* [The dictionary](definitions.md#The-dictionary) must be loaded first.
			* `retLang` is still returned when there are warnings but no errors.
			* Note: [Fallback language](definitions.md#Fallback-languages) still need to be assigned through [Language.SetFallback()](#Calling-SetFallback).
		* `func (lf LanguageTextFile) LoadDefault(r io.Reader, allowBigStrings bool) (retLang *Language, retWarnings []string, retErrors error)`
			* Loads [the default language](definitions.md#The-default-language) text file and [the dictionary](definitions.md#The-dictionary).
			* `retLang` is still returned when there are warnings but no errors.
* Compiled binary files:
	* **LanguageBinaryFile**: `LF_GTR`
		* `func (lf LanguageBinaryFile) Load(r io.Reader, isCompressed bool) (*Language, error)`
			* Loads a [.gtr](definitions.md#Compiled-binary-translation-files) language file.
			* [The dictionary](definitions.md#The-dictionary) must be loaded first.
			* Note: [Fallback language](definitions.md#Fallback-languages) still need to be assigned through [Language.SetFallback()](#Calling-SetFallback).
		* `func (lf LanguageBinaryFile) LoadDefault(r io.Reader, isCompressed bool) (*Language, error)`
			* Loads a [.gtr](definitions.md#Compiled-binary-translation-files) language file.
			* This must be [the default language](definitions.md#The-default-language).
			* [The dictionary](definitions.md#The-dictionary) must be loaded first.
		* `func (lf LanguageBinaryFile) LoadDictionary(r io.Reader, isCompressed bool) (err error, ok bool)`
			* Loads [the dictionary](definitions.md#The-dictionary) via a [compiled dictionary file](definitions.md#Compiled-binary-translation-files), which must be done before loading any compiled translation file or non-default translation text file.
			* Returns an error if the dictionary was not loaded during this call.
			* Returns `ok=true` if the dictionary was read successfully during this or a previous call to this function.
* **LanguageFile**:
	* Both **LanguageTextFile** and **LanguageBinaryFile** are of type **LanguageFile**
	* Both `LanguageTextFile.Load()` and `LanguageBinaryFile.Load()` require that a [dictionary](definitions.md#The-dictionary) already be loaded. The following 2 functions interact with that stored dictionary.
		* `func (LanguageFile) ClearCurrentDictionary() error`
			* Erases the stored dictionary so a new dictionary can be loaded.
			* Returns if dictionary was already loaded.
		* `func (LanguageFile) HasCurrentDictionary() bool`
			* Returns if there is a stored dictionary already loaded
	* Languages that have mismatched dictionaries are incompatible.

### Calling SetFallback
Calling `Language.SetFallback(fallbackLanguage *Language) error` is required after calling `LanguageTextFile.Load()` or `LanguageBinaryFile.Load()`.
* Stores the [fallback language](definitions.md#Fallback-languages).
* If <code>[Settings](translation_files.md#Settings).FallbackLanguage</code> was given for the parent language, the <code>[Settings](translation_files.md#Settings).LanguageIdentifier</code> of the given fallbackLanguage must match. If it was not given, fallbackLanguage becomes the [default language](definitions.md#The-default-language).
* A language cannot have itself set as its fallback. That only occurs naturally for the default language.
* The fallback language being set must already have its fallback language set. This is required so fallback language loops cannot occur.

## Manually loading compiled files with fallbacks
These functions read in languages from [compiled files](definitions.md#Compiled-binary-translation-files) with just the [language identifier](definitions.md#Language-identifiers) given. They are primarily here for when the [gol10n_read_compiled_only build tag](misc.md#Build-optimizations) is specified, as they handle the same kind of shortcut functionality as the [automatic functions](#Automatically-saving-and-loading-the-language-files), which are not included when `gol10n_read_compiled_only` build tag is specified.
They are in the `translate.load_compiled` package.
* `LoadDefault(compiledDirectoryPath string, defaultLanguageIdentifier string, isCompressed bool) (*translate.Language, error)`
	* Loads the [compiled dictionary](definitions.md#Compiled-binary-translation-files) and [default language](definitions.md#The-default-language).
* `Load(compiledDirectoryPath string, langIdentifier string, isCompressed bool, defaultLanguage *translate.Language) (*translate.Language, error)`
	* Loads the language and its [fallbacks](definitions.md#Fallback-languages). The [dictionary](definitions.md#The-dictionary) must be loaded first (Through `LoadDefault()`)
## Manually saving the language files
* `func (l *Language) SaveGTR(w io.Writer, isCompressed bool) error`
	* Saves a [.gtr](definitions.md#Compiled-binary-translation-files) language file
* `func (l *Language) SaveGTRDict(w io.Writer, isCompressed bool) error`
	* Saves a [.gtr](definitions.md#Compiled-binary-translation-files) dictionary file
* `func (l *Language) SaveGTRVarsDict(w io.Writer, isCompressed bool) error`
	* Saves a [.gtr](definitions.md#Compiled-binary-translation-files) variable dictionary file
* `func (l *Language) SaveGoDictionaries(outputDirectory string, GoDictHeader string) (err error, numUpdated uint)`
	* Saves the [*.go dictionary files](#generated-go-dictionary-files) from the language to `$outputDirectory/$NamespaceName/TranslationIDs.go`
	* The `GoDictHeader` is inserted just before the `const` declaration

## Other Language getters
These are the other functions under the `Language` class
* `NumTranslations() uint32`
* `Name() string`
* `LanguageIdentifier() string`
* `LanguageTag() language.Tag`
* `FallbackName() string`
* `MessagePrinter() *message.Printer`
* `TimeLocalizer() (*lctime.Localizer, error)`
* `TranslationIDLookup(index TransIndex) (val string, ok bool)`
	* Returns the [namespace name](definitions.md#Namespaces) and [Translation ID](definitions.md#Translation-IDs) name from a **TransIndex**, separated by a dot.
	* As this is only used for debugging purposes, this is not optimized and has to search through all of a [namespace’s](definitions.md#Namespaces) translations to find a match (only when read from a [compiled dictionary file without the variable dictionary loaded](definitions.md#Compiled-binary-translation-files)).
