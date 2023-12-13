This document describes how the translation text files operate. Specifically, how your translations are read and processed.

# YAML files
The YAML files are to be located in <code>[global_settings](../README.md#Settings-file).InputPath</code> and are to be named `$LanguageIdentifier.yaml`. For example: `en-US.yaml`. See [text processing rules](#Text-processing-rules) for more information.

Unfortunately, YAML processing for very large files (10,000+ translations) can be **slow**. It uses a lot of regular expression reverse lookups and such, and Go’s [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) is **way** slower than [yaml.v2](https://pkg.go.dev/gopkg.in/yaml.v2)! For this reason, [JSON parsing](#JSON-files) is also available.

You can see a [YAML example in the README file](../README.md#YAML-formatting-by-example).

# JSON files
The JSON files are to be located in <code>[global_settings](../README.md#Settings-file).InputPath</code> and are to be named `$LanguageIdentifier.json`. For example: `en-US.json`. See [text processing rules](#Text-processing-rules) for more information.

Unfortunately, JSON trailing commas is not available in any of the fast JSON packages I tried. If you want this option, see <code>[global_settings](../README.md#Settings-file).AllowJSONTrailingComma</code>.

JSON native processing seems to be ~4 times faster than YAML.v2, and ~8 times faster using [valyala/fastjson](https://github.com/valyala/fastjson).

Example: *(Only includes part of the [YAML example](../README.md#YAML-formatting-by-example))*
```json
{
	"Settings":{
		"LanguageName": "English",
		"LanguageIdentifier": "en-US",
		"MissingPluralRule": "A translation rule could not be found for the given plurality",
		"FallbackLanguage": "en-US",
	},
	"NameSpaceExample":{
		"TranslationID": "TranslationValue",
		"Foo天६_": "😭Bar {{*TranslationID}} {{*_animalsGroupNames.Cow}}",
		"BorrowedNumberOfBooks": {
			"\\TestVal": "Test",
			"=0": "You have no books borrowed"
		}
	}
}
```

# Parsing translation strings
Translation strings can have the following special properties:
* [Variables](#Variables) with [Printf format specifiers](#Printf-format-specifiers)
* [Special characters](#Special-characters)
* [Embedded Static Translations](#Embedded-Static-Translations)

## Variables
[Translation strings](definitions.md#Translation-strings) can have variables inside them in the format `{{.VariableName}}`. Example: `{{.BorrowedNumberOfBooks}}`. [Variables must be named](#Variable-Names).

The variable `PluralCount` is always available. It is set accordingly when [Plural functions](language_get_functions.md#Plural-functions) (<code><sub>[Must]</sub>GetPlural<sub>[Named]</sub>(**pluralCount** uint)</code>) are called. Its value defaults to `0xFFFFFFFF` if used in a [non-plural function](language_get_functions.md#Non-Plural-functions) (<code><sub>[Must]</sub>Get<sub>[Named]</sub>()</code>). See [Plurality rules](#Plurality-rules) for more information.

If incorrect argument types for the corresponding translation variables are passed to the [Get() functions](language_get_functions.md#Get-translation-functions), Go’s built-in printf functions will include information about the mismatch in the output string. Some of the special i18n variables types (or missing arguments) return errors if an unexpected type is given.

### Printf format specifiers
Variables inside [translation strings](definitions.md#Translation-strings) can have [printf format specifiers](https://alvinalexander.com/programming/printf-format-cheat-sheet/) appended in the format `{{.VariableName|FormatRules}}`. Example: `{{.BorrowedNumberOfBooks|03}}`.
The following features and flags are supported: width, precision, -, 0
> [!warning]
> do not include the c/s/d/f/etc. type

### Formatting DateTimes
Formatting for date and times uses the [klauspost/lctime](https://github.com/klauspost/lctime) library. See [here](https://pkg.go.dev/github.com/klauspost/lctime?utm_source=godoc#pkg-overview) for format specifiers.

The format string specifier for the date and time is included after the optional [Printf format specifiers](#Printf-format-specifiers), following an exclamation mark. For example: `{{.VariableName!%D %T}}` or `{{.VariableName|10!%x}}`. The string cannot be longer than 255 bytes.

The 3 common formats you will probably need are:
* `%c`: The locale’s appropriate date and time representation
* `%x`: The locale’s appropriate date representation
* `%X`: The locale’s appropriate time representation

## Embedded translations
Other [Translation IDs](definitions.md#Translation-IDs) can be embedded into a translation string for recursive lookups. There are 2 types:
* [Static translations](#Embedded-Static-Translations)
* [Variable Translations](#Embedded-Variable-Translations)

Embedded translations cannot have any [named variables](#Variables) *(besides PluralCount)*.

Embedded translations are looked up according to the following rules:
* A **TransIndex** index to the Translation ID ([Variable Translations only](#Embedded-Variable-Translations))
* The name of the Translation ID (if in the same [namespace](definitions.md#Namespaces) as the translation string it is being embedded into). Example: `{{*TranslationID}}`
* The name of the namespace, a dot, and the name of the Translation ID. Example: `{{*NameSpaceExample.WelcomeTitle}}`

### Embedded Static Translations
Static translations are embedded into [translation strings](#Parsing-translation-strings) at compile time. See [Embedded translations](#Embedded-translations).

They take the format `{{*VariableName}}`. Example: `{{*TranslationID}}`.

### Embedded Variable Translations
These are [named variables](#Variables) which take an [Embedded Translation ID](#Embedded-translations).

## Special characters
| Specifier | Description          | # Ranges | # Required length |
|-----------|----------------------|----------|-------------------|
| \a        | audible alert        |          |                   |
| \b        | backspace            |          |                   |
| \f        | form feed            |          |                   |
| \n        | newline, or linefeed |          |                   |
| \r        | carriage return      |          |                   |
| \t        | tab                  |          |                   |
| \v        | vertical tab         |          |                   |
| \\        | backslash            |          |                   |
| \x##      | hexadecimal char ##  | 0-9, a-f | 2                 |
| \u####    | unicode rune ####    | 0-9, a-f | 2-6               |

> [!warning]
> The text files ([YAML](#YAML-files) or [JSON](#JSON-files)) that you are coming from may have their own internal escaping of backslashes, so you may need to write `\\x80` to get `byte(128)` (as an example).

> [!warning]
> By employing the `\x` character escape with values `>0x7F`, it becomes possible to generate invalid utf8 character strings. The value `0xFF` is reserved by this library and is unusable.

> [!warning]
> When incorporating an escaped Unicode character using `\u`, if it is immediately succeeded by a hexadecimal character (0-9, a-f), that hex character becomes part of the Unicode value. To circumvent this, ensure the Unicode string is prefixed with zeros to achieve a length of six characters.

# Text processing rules
* The `Settings` section is required. See [settings](#Settings) section for its variables
1. All other top level sections are [namespaces](definitions.md#Namespaces).
2. Under namespaces are the list of [Translation IDs](definitions.md#Translation-IDs).
3. Under a Translation ID there can be the following object properties:
	* [Variable Names](#Variable-Names)
	* [Plurality rules](#Plurality-rules)
	* Properties starting with a “\” are ignored
	1) If a Translation ID has only 1 translation and no variables, it can be included on the same line as the Translation ID, in which case it is treated as a `^` rule. Example: `Wolf: Pack`

> [!important]
> The order of variables is the order in which parameters are expected to be given to the [translation functions](language_get_functions.md#Get-translation-functions).<br>
> If variable order or types differ between translation files, then warnings are issued when compiling.

Warnings are generated in the following conditions:
* When creating a non-[default language](definitions.md#The-default-language), if a [namespace](definitions.md#Namespaces) or [translation ID](definitions.md#Translation-IDs) is missing (not in [the dictionary](definitions.md#The-dictionary)), or an extra one exists.
* A variable mismatch is found for a translation between the secondary and the default language. This is to help catch errors if the variable list is changed in the default language.

> [!warning]
> When compiling rules: Goroutines are split off at both the namespace and translation ID levels (this benchmarked the best), so this can easily take up 100% of your CPU if you have 100,000+ translations and do not modify your [GOMAXPROCS](https://pkg.go.dev/runtime#GOMAXPROCS).

## Variable Names
Variable Names must be included as properties under [Translation IDs](definitions.md#Translation-IDs) (See [Text processing rules](#Text-processing-rules)).

Variable Names can contain unicode letters, unicode digits, or underscores. They are case-sensitive (including *PluralCount*).

The values for these are their type as a string. The valid types (case-insensitive) are the following. If the type is directly passed to [printf](https://pkg.go.dev/fmt#hdr-Printing), its printf specifier character [verb] is included.
* *Any type*: Anything `%v`
* *String Types*: String `%s`
  * Accepts any [Stringer interface](https://pkg.go.dev/fmt#Stringer)
* *Number Types*: Integer `%d`, Binary `%b`, Octal `%o`, HexLower `%x`, HexUpper `%X`, Scientific `%e`, Floating `%f`
* *Dates*: DateTime (See [formatting DateTimes](#Formatting-DateTimes))
* *i18n numeric types*: Currency, IntegerWithSymbols, FloatWithSymbols
* *Other*: Bool `%t`
* *Embedded translations*: VariableTranslation (See [Embedded Variable translations](#Embedded-Variable-Translations))

# Settings
These are values that are required in the [text processing](#Text-processing-rules) `Settings` section.
* The `LanguageName` value is required. It is used for reference.
* The `LanguageIdentifier` value is required. See [language identifiers](definitions.md#Language-identifiers).
* The `MissingPluralRule` value is required. It is the translation returned if a matching plurality rule cannot be found during a [plural function](language_get_functions.md#Plural-functions). An error is still returned too in this case for non-[Must functions](language_get_functions.md#Must-functions).
* The `FallbackLanguage` value is optional. See [Fallback languages](definitions.md#Fallback-languages). The fallback for the [default language](definitions.md#The-default-language) is ignored.

# Plurality rules:
* Plurality rules define what translation to use depending upon a given `PluralCount`.
* Rules can take the following operations to compare against `PluralCount`:
	| Name                  | Operator | Example    | Notes              |
	|-----------------------|----------|------------|--------------------|
	| Any                   |  ^       | ^          | Everything matches |
	| Equals                |  =       | =1         |                    |
	| Less Than             |  &lt;    | &lt;2      |                    |
	| Greater Than          |  &gt;    | &gt;3      |                    |
	| Less Than or Equal    |  &lt;=   | &lt;=4     |                    |
	| Greater Than or Equal |  &gt;=   | &gt;= 5    |                    |
	| Between               |  ~       | ~6-7       | Is inclusive       |
	| Ignore                |  \       | \Translator| Line is ignored    |
* Rules are processed in given order
* Whitespace is ignored
* If calling a [Non-Plural functions](language_get_functions.md#Non-Plural-functions) then the `Any` rule is always used. If it does not exist, then the first rule is used.
* If calling a [Plural functions](language_get_functions.md#Plural-functions) and there is no matching rule, then <code>[Settings](#Settings).MissingPluralRule</code> is returned. An error is still returned for non-[Must functions](language_get_functions.md#Must-functions).
* See [Hard Limits](misc.md#Hard-limits) operator notes
