This document gives the definitions and their specifics for the concepts used by this library.

# The dictionary
The dictionary is created when loading [the default language](#The-default-language) from a [text file](translation_files.md), or when loading a [compiled dictionary file](#Compiled-binary-translation-files).

The dictionary contains the [namespaces](#Namespaces) and [translation IDs](#Translation-IDs) and their respective indexes (**TransIndex**).

All languages use the same dictionary. If the dictionary is changed, all languages must be regenerated.

Loading non-default languages or a [compiled translation file](#Compiled-binary-translation-files) requires that a dictionary already be loaded.

# Namespaces
* Namespaces help keep translation sections separated to help reduce clutter when importing translations into Go.
	* Namespaces each [get their own Go package](using_in_go.md#generated-go-dictionary-files) for importing
* They also allow for duplicate [translation IDs](#Translation-IDs) per namespace.
* Namespaces can only contain alphanumeric and underscore characters. Their first character cannot be a number.

# Translation IDs
* These are defined in the [translation text files](translation_files.md#Text-processing-rules) as the property keys directly under a [namespace](#Namespaces).
* Translation IDs are used to access translations in 2 ways:
	1. They can be used as strings with a [Namespace](#Namespaces) in the [Named: <sub>[Must]</sub>Get<sub>[Plural]</sub>Named()](language_get_functions.md#Named-functions) functions.
	2. They can be used as indexes from a [Generated Go dictionary file](using_in_go.md#generated-go-dictionary-files) in the [Indexed: <sub>[Must]</sub>Get<sub>[Plural]</sub>()](language_get_functions.md#Indexed-functions) functions. Examples:\
	`Language.GetPlural(NameSpaceExample.BorrowedNumberOfBooks, 5)`\
	`Language.Get(NameSpaceExample.WelcomeTitle, "Dakusan", time.Now(), currency.GBP.Amount(1275.98), 1492)`
* There cannot be conflicting IDs in a single [Namespace](#Namespaces).
* Translation IDs must start with an uppercase alphabetic character. Afterward they can contain unicode letters, unicode digits, or underscores.

# Translation strings
* These are defined in the [translation text files](translation_files.md#Text-processing-rules) as the property values of a [plurality rule](translation_files.md#Plurality-rules) or a [Translation ID](#Translation-IDs).
* They can be 64KB long (See [Hard limits](misc.md#Hard-limits) on expanding that).
* See [Parsing translation strings](translation_files.md#Parsing-translation-strings) on how to format them.

# Fallback languages
Each language can contain 1 optional fallback language, set as a [language identifier](#Language-identifiers) in its settings section via <code>[Settings](translation_files.md#Settings).FallbackLanguage</code>.

If a translation was not defined for a language, then its fallback language will be checked.

Fallback languages are checked recursively up to [the default language](#The-default-language).

When [manually loading language files](using_in_go.md#Manually-loading-the-language-files), [Language.SetFallback()](using_in_go.md#Calling-SetFallback) must be used. Fallback languages must use the exact same [dictionary](#The-dictionary).

# The default language
The default language (<code>[global_settings](../README.md#Settings-file).DefaultLanguage</code>) must contain all the [translation IDs](#Translation-IDs). The internals of this module [(the dictionary)](#The-dictionary) are formulated through the default language.

If a [translation ID](#Translation-IDs) is missing from a non-default language, and it does not have a [fallback language](#Fallback-languages), then the translation from the default language is returned.

The default language does not have a [fallback language](#Fallback-languages).

# Compiled binary translation files
One file per language is placed in <code>[global_settings](../README.md#Settings-file).CompiledOutputPath</code>. They are named `$LanguageName.gtr` and have a .gz (gzip compress) suffix added if <code>[global_settings](../README.md#Settings-file).CompressCompiled</code> is turned on.

A special dictionary file containing the [translation IDs](#Translation-IDs) and [namespaces](#Namespaces) is compiled from [the dictionary](#The-dictionary) and uses its ordering. It is saved as `dictionary.gtr`. When [the dictionary](#The-dictionary) is updated, all other translation files may need to be updated. A variables dictionary is also saved to `variables.gtr`, but it is only ever used when processing non-[default language](definitions.md#The-default-language) [translation text files](translation_files.md) and the compiled dictionary is being loaded.

# Language identifiers
The language identifier identifies the i18n locale for formatting dates, currencies, etc. They are the [two-letter ISO 639-1 language code](https://en.wikipedia.org/wiki/ISO_639-1) with an optional dash and a [ISO-3166 country code](https://en.wikipedia.org/wiki/List_of_ISO_3166_country_codes). The full list can be found [here](https://www.fincher.org/Utilities/CountryLanguageList.shtml).

When using the [command line interface](../README.md#Command-line-interface) or the [automated library functions](using_in_go.md#Automatically-saving-and-loading-the-language-files):
* The filenames (without the extension) must match the language identifiers
* Language identifiers are used to identify and link [fallback languages](#Fallback-languages).
