This document lists the translation Get() functions for [use in go](using_in_go.md#Using-translations-in-Go).

# “Get” translation functions
The base function is `Language.Get(index TransIndex, ...args) (string, error)`. It takes a **TransIndex** (*uint32*) and a list of arguments, and returns either the result string or an error. All other functions are variations of this function as defined in the following function subsections.

If a [translation ID](definitions.md#Translation-IDs) is missing from a non-[default language](definitions.md#The-default-language), then the translation from a [fallback language](definitions.md#Fallback-languages) is returned.

The complete list of functions under the `Language` class is:
| Function | Arguments | Return |
| -------- | --------- | ------ |
| $${\color{DarkOrchid}Get\color{black}\}$$ | $$\{\color{Green}index\ \textbf\{TransIndex\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string, error |
| $${\color{DarkOrchid}Get\color{black}\color{Chocolate}Plural\color{black}\}$$ | $$\{\color{Green}index\ \textbf\{TransIndex\}\color{black},\ \color{Chocolate}pluralCount\ \textbf\{uint\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string, error |
| $${\color{Red}Must\color{black}\color{DarkOrchid}Get\color{black}\}$$ | $$\{\color{Green}index\ \textbf\{TransIndex\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string |
| $${\color{Red}Must\color{black}\color{DarkOrchid}Get\color{black}\color{Chocolate}Plural\color{black}\}$$ | $$\{\color{Green}index\ \textbf\{TransIndex\}\color{black},\ \color{Chocolate}pluralCount\ \textbf\{uint\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string |
| $${\color{DarkOrchid}Get\color{black}\color{Magenta}Named\color{black}\}$$ | $$\{\color{Green}nameSpace\ \textbf\{string\},\ translationID\ \textbf\{string\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string, error |
| $${\color{DarkOrchid}Get\color{black}\color{Chocolate}Plural\color{black}\color{Magenta}Named\color{black}\}$$ | $$\{\color{Green}nameSpace\ \textbf\{string\},\ translationID\ \textbf\{string\}\color{black},\ \color{Chocolate}pluralCount\ \textbf\{uint\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string, error |
| $${\color{Red}Must\color{black}\color{DarkOrchid}Get\color{black}\color{Magenta}Named\color{black}\}$$ | $$\{\color{Green}nameSpace\ \textbf\{string\},\ translationID\ \textbf\{string\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string |
| $${\color{Red}Must\color{black}\color{DarkOrchid}Get\color{black}\color{Chocolate}Plural\color{black}\color{Magenta}Named\color{black}\}$$ | $$\{\color{Green}nameSpace\ \textbf\{string\},\ translationID\ \textbf\{string\}\color{black},\ \color{Chocolate}pluralCount\ \textbf\{uint\}\color{black},\ \color{LightSeaGreen}...args\color{black}\ \color{DeepSkyBlue}\}$$ | string |

## Non-Plural functions
Non-Plural functions are of the format <sub>[Must]</sub>Get<sub>[Named]</sub>. These functions **DO NOT** take a `pluralCount uint` parameter. See [Plurality rules](translation_files.md#Plurality-rules) for more information.

* Get(index **TransIndex**, ...args) (**string**, **error**)
* MustGet(index **TransIndex**, ...args) (**string**)
* GetNamed(nameSpace **string**, translationID **string**, ...args) (**string**, **error**)
* MustGetNamed(nameSpace **string**, translationID **string**, ...args) (**string**)

## Plural functions
Plural functions are of the format <sub>[Must]</sub>Get**Plural**<sub>[Named]</sub>. These functions take a `pluralCount uint` parameter. See [Plurality rules](translation_files.md#Plurality-rules) for more information.

* Get`Plural`(index **TransIndex**, `pluralCount` **uint**, ...args) (**string**, **error**)
* MustGet`Plural`(index **TransIndex**, `pluralCount` **uint**, ...args) (**string**)
* Get`Plural`Named(nameSpace **string**, translationID **string**, `pluralCount` **uint**, ...args) (**string**, **error**)
* MustGet`Plural`Named(nameSpace **string**, translationID **string**, `pluralCount` **uint**, ...args) (**string**)

## Must functions
Must functions are of the format **Must**Get<sub>[Plural]</sub><sub>[Named]</sub>. They return empty strings if an error occurs.

* `Must`Get(index **TransIndex**, ...args) (**string**)
* `Must`GetPlural(index **TransIndex**, pluralCount **uint**, ...args) (**string**)
* `Must`GetNamed(nameSpace **string**, translationID **string**, ...args) (**string**)
* `Must`GetPluralNamed(nameSpace **string**, translationID **string**, pluralCount **uint**, ...args) (**string**)

## Indexed functions
Indexed functions take an index (**TransIndex**) to reference the [Translation ID](definitions.md#Translation-IDs).

* Get(`index` **TransIndex**, ...args) (**string**, **error**)
* GetPlural(`index` **TransIndex**, pluralCount **uint**, ...args) (**string**, **error**)
* MustGet(`index` **TransIndex**, ...args) (**string**)
* MustGetPlural(`index` **TransIndex**, pluralCount **uint**, ...args) (**string**)

## Named functions
Named functions take a namespace and translation ID to reference the [Translation ID](definitions.md#Translation-IDs).

* Get`Named`(`nameSpace` **string**, `translationID` **string**, ...args) (**string**, **error**)
* GetPlural`Named`(`nameSpace` **string**, `translationID` **string**, pluralCount **uint**, ...args) (**string**, **error**)
* MustGet`Named`(`nameSpace` **string**, `translationID` **string**, ...args) (**string**)
* MustGetPlural`Named`(`nameSpace` **string**, `translationID` **string**, pluralCount **uint**, ...args) (**string**)
