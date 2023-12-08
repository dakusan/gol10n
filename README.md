[![Go Report Card](https://goreportcard.com/badge/github.com/dakusan/gol10n)](https://goreportcard.com/report/github.com/dakusan/gol10n)
[![GoDoc](https://godoc.org/github.com/dakusan/gol10n?status.svg)](https://godoc.org/github.com/dakusan/gol10n)

# Description
This is a highly space and memory optimized l10n (localization) library for Go (GoLang) pronounced “Goal Ten”.<br>
[Translation strings](docs/definitions.md#Translation-strings) are held, per language, in [text files](docs/translation_files.md) (either [YAML](docs/translation_files.md#YAML-files) or [JSON](docs/translation_files.md#JSON-files)), and compile into [.gtr](docs/definitions.md#Compiled-binary-translation-files) or .gtr.gz (gzip compressed) files.

Translations can be [referenced in Go code](docs/using_in_go.md#Using-translations-in-Go) either by an index, or a [namespace](docs/definitions.md#Namespaces) and [translation ID](docs/definitions.md#Translation-IDs).
Referencing by index is the fastest, most efficient, and what this library was built for. Indexes are stored as constants in [generated Go dictionary files](docs/using_in_go.md#generated-go-dictionary-files) by [namespace](docs/definitions.md#Namespaces), and are also held in [the dictionary](docs/definitions.md#The-dictionary).

Features:
* Translations are created in [YAML](docs/translation_files.md#YAML-files) or [JSON](docs/translation_files.md#JSON-files) [config files](docs/translation_files.md)
* Translations are compiled into [optimized binary files](docs/definitions.md#Compiled-binary-translation-files) for super-fast and space-efficient loading and use
* [Go [enum]](docs/using_in_go.md#generated-go-dictionary-files) [dictionary](docs/definitions.md#The-dictionary) files are created so translations can be accessed by constant index within [namespaces](docs/definitions.md#Namespaces)
* [Command line interface](#Command-line-interface) and [golang library level access](docs/using_in_go.md#Automatically-saving-and-loading-the-language-files) are both available
* [Translations](docs/definitions.md#Translation-IDs) can be separated into [namespaces](docs/definitions.md#Namespaces)
* [Typed variables](docs/translation_files.md#Variables) inside [translation strings](docs/definitions.md#Translation-strings)
* [Fallback languages](docs/definitions.md#Fallback-languages)
* [Printf type formatters](docs/translation_files.md#Printf-format-specifiers) are available and also contain i18n outputs
* [Plurality rules](docs/translation_files.md#Plurality-rules)
* [Embedded translations](docs/translation_files.md#Embedded-translations)

Translation data and rules are stored in optimized blobs similar to how Go’s native i18n package stores its data.

Instead of trying to extract your in-source-code translations though a tool, this lets you handle them manually. I find that keeping translation strings outside of source files and instead using constants is much cleaner.

Gol10n is available under the same style of BSD license as the Go language, which can be found in the LICENSE file.

# Installation
Gol10n is available using the standard go get command.

Install by running:

```
go get github.com/dakusan/gol10n
```

# YAML formatting by example
> [!note]
> Parts of this example will be used in other documentation examples
```yaml
Settings:
    LanguageName: English
    LanguageIdentifier: en-US
    MissingPluralRule: A translation rule could not be found for the given plurality
    FallbackLanguage: en-US #Ignored on the default language

NameSpaceExample:
    TranslationID: TranslationValue
    Foo天६_: 😭Bar {{*TranslationID}} {{*_animalsGroupNames.Cow}}
    BorrowedNumberOfBooks:
        \TestVal: Test
        =0: You have no books borrowed
        =1: You have one book and are encouraged to borrow more
        <=10: You have {{.PluralCount}} books and are within your borrowing limit. If you were a cow, this would be called a {{.OtherVar}}
        ^: You have {{.PluralCount}} books and are over your limit of 10 books
        OtherVar: VariableTranslation
        \Translator: Dakusan
    WelcomeTitle:
        ^: Welcome to our hotel <b>{{.Name|-10}}</b>.\nYour stay is for {{.NumDay天s|08.2}} days. Your checkout is on {{.CheckoutDay!%x %X}} and your cost will be {{.Cost}}
        Name: String #Inlined printf rules. Left justify with spaces, width of 10
        CheckoutDay: DateTime
        Cost: Currency
        NumDay天s: IntegerWithSymbols #Inlined printf rules. Right justify with '0', Total width of 8. Include 2 digits of decimal

_animalsGroupNames:
    Wolf: Pack
    Cow:
        =1: Lonely
        <=12: Herd
        ^: Flink
    Crow:
        <10: Murder
        <100: Genocide
        ~101-164: Too many
        ^: Run for your life
```

[JSON](docs/translation_files.md#JSON-files) parsing is also available.

# Command line interface
```
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
```

There are also [automatic](docs/using_in_go.md#Automatically-saving-and-loading-the-language-files) and [manual](docs/using_in_go.md#Manually-loading-the-language-files) library functions available that duplicate all command line functionality.

# Example “Get” translation function calls
[Indexed functions](docs/language_get_functions.md#Indexed-functions) examples:<br>
> ```golang
> str, err := Language.Get(_animalsGroupNames.Wolf)
> ```
> Yields
> ```golang
> str := "Pack"
> err := nil
> ```

> ```golang
> str := Language.MustGetPlural(NameSpaceExample.BorrowedNumberOfBooks, 9, _animalsGroupNames.Cow)
> ```
> Yields
> ```golang
> str := "You have 9 books and are within your borrowing limit. If you were a cow, this would be called a Herd"
> ```

[Named functions](docs/language_get_functions.md#Named-functions) example:<br>
> [!important]
> The parameter order is based upon the order in the [translation text file](docs/translation_files.md#Text-processing-rules), **NOT** the order in the translation string

> ```golang
> const numDays = 1001
> str := Language.MustGetNamed(
>     "NameSpaceExample", "WelcomeTitle",   //Namespace, TranslationID
>     "Frodo",                              //Variable: Name
>     time.Now().Add(time.Hour*24*numDays), //Variable: CheckoutDay
>     currency.GBP.Amount(1275.98),         //Variable: Cost
>     numDays,                              //Variable: NumDay天s
> )
> ```
> Yields
> ```golang
> str := "Welcome to our hotel <b>Frodo     </b>.\nYour stay is for    1,001 days. Your checkout is on 12/25/2123 09:18:24 PM and your cost will be £ 1,275.98"
> ```

# Settings file
The settings file, `gol10n-settings.yaml`, requires the following variables:
* **DefaultLanguage**: The [identifier](docs/definitions.md#Language-identifiers) for the [default language](docs/definitions.md#The-default-language).
* **InputPath**: The directory with the [translation text files](docs/translation_files.md).
* **GoOutputPath**: The directory to output the [generated Go dictionary files](docs/using_in_go.md#generated-go-dictionary-files) to. Each [namespace](docs/definitions.md#Namespaces) gets its own directory and file in the format `$NamespaceName/translationIDs.go`.
* **CompiledOutputPath**: The directory to output the [compiled binary translation files](docs/definitions.md#Compiled-binary-translation-files) to. Each language gets its own .gtr or .gtr.gz (gzip compressed) file.
* **CompressCompiled**: A boolean specifying whether the [compiled binary translation files](docs/definitions.md#Compiled-binary-translation-files) are saved as .gtr or .gtr.gz (gzip compressed).
* **AllowBigStrings**: A boolean that specifies if translation strings can be larger than 64KB. If true, and a large translation string is found, then [compiled binary translation files](docs/definitions.md#Compiled-binary-translation-files) will become larger.
* **AllowJSONTrailingComma**: A boolean that specifies if [JSON](docs/translation_files.md#JSON-files) files can have trailing commas. If true, a sanitization process is ran over the JSON that changes the regular expression `,\s*\n\s*}` to just `}`.

These settings are only used when using this library in a [command line interface](#Command-line-interface). When calling the go functions [automatically](docs/using_in_go.md#Automatically-saving-and-loading-the-language-files) or [manually](docs/using_in_go.md#Manually-loading-the-language-files), these settings are part of the function parameters.

# Additional reading:
* [Definitions](docs/definitions.md)
* [Translation Files](docs/translation_files.md)
* [Using in Go](docs/using_in_go.md)
* [Language Get() Functions](docs/language_get_functions.md)
* [Misc](docs/misc.md) (Hard and soft limits and TODO section)