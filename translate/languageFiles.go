//Public functions to load compiled files from io.Reader

package translate

import (
	"compress/gzip"
	"errors"
	"io"
)

// LanguageFile is the base type to load language files
type LanguageFile int

// LanguageBinaryFile is the interface to load .gtr compiled files
type LanguageBinaryFile LanguageFile

//goland:noinspection GoSnakeCaseUsage
const (
	LF_GTR LanguageBinaryFile = iota

	//This is used to make LanguageTextFile enum pick up where this enum left off
	lf_DO_NOT_USE
)

// Holds a copy of the dictionary, which is static for all languages
var remDict *languageDict

// Load loads a .gtr language file. The default language text file or the dictionary must be loaded first.
//
// Note: Fallback languages still need to be assigned through Language.SetFallback()
func (lf LanguageBinaryFile) Load(r io.Reader, isCompressed bool) (*Language, error) {
	//Check if the dictionary is already loaded
	localDict := remDict
	hasDict := localDict != nil
	if !hasDict {
		return nil, errors.New("The dictionary has not been loaded yet. You must first call LanguageTextFile.LoadDefault() or LanguageBinaryFile.LoadDictionary()")
	}

	//Handle compressed files
	if isCompressed {
		if _r, err := gzip.NewReader(r); err != nil {
			return nil, err
		} else {
			r = _r
		}
	}

	//Read the file
	var l Language
	if err := l.fromCompiledFile(r, localDict); err != nil {
		return nil, err
	}
	return &l, nil
}

// LoadDefault loads a .gtr language file. This must be the default language. The dictionary must be loaded first.
func (lf LanguageBinaryFile) LoadDefault(r io.Reader, isCompressed bool) (*Language, error) {
	if l, err := lf.Load(r, isCompressed); err != nil {
		return nil, err
	} else {
		l.fallback = l
		return l, nil
	}
}

// LoadDictionary loads a compiled dictionary file, which must be done before loading any compiled translation file or non-default-language translation text file.
//
// Returns an error if the dictionary was not loaded during this call.
//
// Returns ok=true if the dictionary was read successfully during this or a previous call to this function.
func (lf LanguageBinaryFile) LoadDictionary(r io.Reader, isCompressed bool) (err error, ok bool) {
	//Check if the dictionary is already loaded
	hasDict := remDict != nil

	//Confirm LanguageFile type
	if lf != LF_GTR {
		return errors.New("Only compiled dictionaries can be loaded through this function"), hasDict
	}

	//Throw an error if the dictionary is already loaded
	if hasDict {
		return errors.New("Dictionary already loaded"), hasDict
	}

	//Handle compressed files
	if isCompressed {
		if _r, err := gzip.NewReader(r); err != nil {
			return err, false
		} else {
			r = _r
		}
	}

	//Load and save the dictionary
	var newDict languageDict
	if err := newDict.fromCompiledFile(r); err != nil {
		return err, false
	}

	//Lock the stored dictionary and write it
	//Not worrying about race conditions as dictionaries are not changed after being created and stored
	remDict = &newDict

	//Return success
	return nil, true
}

// LoadDictionaryVars loads a compiled variable dictionary file. This is only used when processing non-default language text files and the compiled dictionary is being loaded.
func (lf LanguageBinaryFile) LoadDictionaryVars(r io.Reader, isCompressed bool) (err error) {
	//Make sure dictionary is already loaded
	if remDict == nil {
		return errors.New("The dictionary has not been loaded yet. You must first call LanguageTextFile.LoadDefault() or LanguageBinaryFile.LoadDictionary()")
	}

	//Handle compressed files
	if isCompressed {
		if _r, err := gzip.NewReader(r); err != nil {
			return err
		} else {
			r = _r
		}
	}

	//Process and return if error
	if err := remDict.fromCompiledVarFile(r); err != nil {
		return err
	}

	//Return success
	initTextProcessing()
	return nil
}

// ClearCurrentDictionary erases the stored dictionary used for LanguageTextFile.Load() and LanguageBinaryFile.Load(). Languages that have mismatched dictionaries are incompatible. Returns if dictionary was already loaded
func (ll LanguageFile) ClearCurrentDictionary() bool {
	hasDict := remDict != nil
	remDict = nil
	return hasDict
}

// HasCurrentDictionary returns if there is a stored dictionary already loaded (for LanguageTextFile.Load() and LanguageBinaryFile.Load())
func (ll LanguageFile) HasCurrentDictionary() bool {
	return remDict != nil
}
