//Public functions to load text files from io.Reader
//go:build !gol10n_read_compiled_only

package translate

import (
	"errors"
	"io"
	"strings"
)

// LanguageTextFile is the interface to load translation text files
type LanguageTextFile LanguageFile

//goland:noinspection GoSnakeCaseUsage
const (
	LF_YAML = iota + LanguageTextFile(lf_DO_NOT_USE)
	LF_JSON
	LF_JSON_AllowTrailingComma
)

// Load loads (yaml or json) a language text file. The default language or the dictionary must be loaded first. retLang is still returned when there are warnings but no errors.
//
// Note: Fallback languages still need to be assigned through Language.SetFallback()
func (lf LanguageTextFile) Load(r io.Reader, allowBigStrings bool) (retLang *Language, retWarnings []string, retErrors error) {
	//Check if the dictionary is already loaded
	localDict := remDict
	hasDict := localDict != nil
	if !hasDict {
		return nil, nil, errors.New("The dictionary has not been loaded yet. You must first call LanguageTextFile.LoadDefault() or LanguageBinaryFile.LoadDictionary()")
	}

	//Load and return the language
	return lf.loadReal(r, localDict, allowBigStrings)
}

// LoadDefault loads (yaml or json) the default language text file (and the dictionary). This must be called before reading other languages (unless LanguageBinaryFile.LoadDictionary was already called). retLang is still returned when there are warnings but no errors.
func (lf LanguageTextFile) LoadDefault(r io.Reader, allowBigStrings bool) (retLang *Language, retWarnings []string, retErrors error) {
	//Check if the dictionary is already loaded
	if remDict != nil {
		return nil, nil, errors.New("The dictionary was already loaded. You can load this language through LanguageTextFile.Load()")
	}

	//Load the language
	var l *Language
	var warn []string
	if _l, _warn, err := lf.loadReal(r, nil, allowBigStrings); err != nil {
		return nil, _warn, err
	} else {
		l, warn = _l, _warn
	}

	//Write the stored dictionary
	//Not worrying about race conditions as dictionaries are not changed after being created and stored
	remDict = l.dict

	//Return success
	l.fallback = l //Set self as the fallback
	return l, warn, nil
}

func (lf LanguageTextFile) loadReal(r io.Reader, dict *languageDict, allowBigStrings bool) (retLang *Language, retWarnings []string, retErrors error) {
	//Load the full structure from the translation text file
	var topItem tpItem
	switch lf {
	case LF_YAML:
		if b, err := io.ReadAll(r); err != nil {
			return nil, nil, errors.New("Error reading the file: " + err.Error())
		} else if y, err := fromYamlFile(b); err != nil {
			return nil, nil, errors.New("Error reading the file: " + err.Error())
		} else {
			topItem = &y
		}
	case LF_JSON, LF_JSON_AllowTrailingComma:
		if b, err := io.ReadAll(r); err != nil {
			return nil, nil, errors.New("Error reading the file: " + err.Error())
		} else if y, err := fromJsonFile(b, lf == LF_JSON_AllowTrailingComma); err != nil {
			return nil, nil, errors.New("Error reading the file: " + err.Error())
		} else {
			topItem = &y
		}
	default:
		return nil, nil, errors.New("Invalid LanguageTextFile type given")
	}

	//Load and return the language
	var l Language
	initTextProcessing()
	errs, warnings := l.fromTextFile(topItem, dict, allowBigStrings)
	if len(errs) > 0 {
		return nil, warnings, errors.New(strings.Join(errs, "\n"))
	}
	return &l, warnings, nil
}
