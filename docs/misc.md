# Limits:
## Hard limits:
* The [compiled binary translation files](definitions.md#Compiled-binary-translation-files) cannot be larger than 4GB
* Translations:
	* [YAML](translation_files.md#YAML-files) and [JSON](translation_files.md#JSON-files) files must be valid utf8
	* A [translation string](definitions.md#Translation-strings) cannot be larger than 64KB unless <code>[global_settings](../README.md#Settings-file).AllowBigStrings</code> is true
	* A [Translation ID](definitions.md#Translation-IDs) cannot be larger than 64KB
	* A [Namespace name](definitions.md#Namespaces) cannot be longer than 255 bytes
	* Variables:
		* Names cannot be longer than 255 bytes
		* Cannot have more than 255 variables per [translation string](definitions.md#Translation-strings)
		* [Printf format specifiers](translation_files.md#Printf-format-specifiers) numbers cannot be larger than 255
	* [Plural function](language_get_functions.md#Plural-functions) Operators:
		* Numbers following operators can be between 0 and 255
		* The second number of the *Between* operator can be at most 63 higher than the first number
		* There cannot be more than 255 operators on a translation

## Soft limits:
These limits have been introduced to protect systems from badly formed translation files, but they can be changed in the source code
* The total length of all the [translation strings](definitions.md#Translation-strings) together is capped at 3.5GB
* The total number of [Plural function](language_get_functions.md#Plural-functions) operators is capped at 1 million
* The total number of namespaces is capped at 1,000
* The total number of translations is capped at 1 million
* The total length of all the [translation IDs](definitions.md#Translation-IDs) together is capped at 32MB
* The total length of all the [namespace names](definitions.md#Namespaces) together is capped at 1MB
* [Embedded Static Translations](translation_files.md#Embedded-Static-Translations) cannot recurse more than 100 times

# Build optimizations
* When building with this library:
	* If you include `-tags gol10n_read_compiled_only`, then only the functionality to read compiled files is included. This cuts 100KB-150KB from your executable.
	* If you include `-ldflags "-s"` this will decrease your executable size by stripping the symbol table.

# Contributing to this project
* Make sure files are ran through [gofmt -s](https://pkg.go.dev/cmd/gofmt) before submitting pull requests. I’m trying to keep the [![Go Report Card](https://goreportcard.com/badge/github.com/dakusan/gol10n)](https://goreportcard.com/report/github.com/dakusan/gol10n) at 100%.
* This project is licensed under the 3-clause BSD