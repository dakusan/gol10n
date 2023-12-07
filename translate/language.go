//Primary data structures and retrieval functions

/*
Package translate is a highly space and memory optimized l10n (localization) library.

Translation strings are held, per language, in text files (either YAML or JSON), and compile into .gtr or .gtr.gz (gzip compressed) files.

Translations can be referenced in Go code either by an index, or a namespace and translation ID.

Referencing by index is the fastest, most efficient, and what this library was built for. Indexes are stored as constants in generated Go files by namespace.

Translation data and rules are stored in optimized blobs similar to how Go’s native i18n package stores its data.
*/
package translate

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/klauspost/lctime"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"strings"
)

type translationRule struct {
	startPos uint32 //Location in Language.stringsData. endPos is calculated by using the startPos of the next rule
	rule     pluralRule
	//2 extra bytes are available here
}
type translationRuleSlice struct {
	startIndex uint32 //Location in Language.translationRule. endIndex is calculated by using the startIndex of the next rule
}
type languageDict struct {
	namespaces        map[string]*namespace
	namespacesInOrder []string
	hash              []byte //A dictionary hash to make sure language files are compatible
	hasVarsLoaded     bool   //If namespaces.idsInOrder is filled in
}
type namespace struct {
	name       string
	index      uint
	ids        translationIDs             //Translation ID index lookup
	idsInOrder []translationIDNameAndVars //The Translation IDs in order for this namespace. This is only filled/used when reading from translation text files (or the variable dictionary file)
}

// Used for the goWriter and confirming variables in non-default text files
type translationIDNameAndVars struct {
	name string
	vars []translationIDVar
}
type translationIDVar struct {
	name    string
	varType variableType
}

// TransIndex is the type used for quick Translation ID lookup. Their values are stored as constants in generated Go files by namespace.
type TransIndex uint32

type translationIDs map[string]TransIndex

// Language is the primary structure for this library that holds all the namespaces and translations
type Language struct {
	stringsData        []byte                 //All translation strings concatenated into a single array
	rules              []translationRule      //All translation rules concatenated into a single array. There is always 1 extra so the last endPos can be calculated
	translations       []translationRuleSlice //Translation rules per translation. There is always 1 extra so the last endIndex can be calculated
	dict               *languageDict          //This is the same value in all language objects
	fallback           *Language              //If fallbackName is not given, then this is set to the default language. This is itself for the default language.
	name               string
	fallbackName       string
	missingPluralRule  string
	languageIdentifier string
	languageTag        language.Tag //Pulled from the languageIdentifier
	messagePrinter     *message.Printer
	timeLocalizer      *lctime.Localizer
}

const (
	errNoPluralRuleMatches = "no plural rule matches"
	maxEmbeddedCount       = 100
)

//-----------------------------Main Get() functions-----------------------------

// All Get...() functions call this
func (l *Language) getReal(index TransIndex, pluralCount int64, embeddedCount uint, args []interface{}) (string, error) {
	//Confirm index is valid
	if uint32(index) >= l.NumTranslations() {
		return retErrWithStr(fmt.Errorf("Invalid index location: %d", index))
	}

	//If embeddedCount has exceeded maxEmbeddedCount return an error
	if embeddedCount > maxEmbeddedCount {
		return retErrWithStr(fmt.Errorf("Cannot have more than %d embedded translation levels", maxEmbeddedCount))
	}

	//Find the [fallback] language that has the translation
	var curLang, prevLang *Language
	var sliceIndex, sliceLength uint32
	for curLang = l; curLang != prevLang && curLang != nil; curLang = curLang.fallback {
		sliceIndex = curLang.translations[index].startIndex
		sliceLength = curLang.translations[index+1].startIndex - sliceIndex
		if sliceLength != 0 {
			break
		}
		prevLang = curLang
	}
	if curLang == nil {
		return retErrWithStr(errors.New("Fallback language was not set"))
	}
	if curLang == prevLang {
		return retErrWithStr(errors.New("No rules found for translation"))
	}

	//If a non-plural function then the 0th rule will match if there is no cmpAll rule
	matchingRuleIndex := int64(-1)
	isPluralFunc := pluralCount >= 0
	if !isPluralFunc {
		matchingRuleIndex = int64(sliceIndex)
	}

	//Search for a matching rule
	if !isPluralFunc {
		for i, r := range curLang.rules[sliceIndex : sliceIndex+sliceLength] {
			if r.rule.getOp() == cmpAll {
				matchingRuleIndex = int64(sliceIndex) + int64(i)
				break
			}
		}
	} else {
		for i, r := range curLang.rules[sliceIndex : sliceIndex+sliceLength] {
			if r.rule.cmp(uint8(pluralCount)) {
				matchingRuleIndex = int64(sliceIndex) + int64(i)
				break
			}
		}
	}

	//If there is not a matching rule then return error
	if matchingRuleIndex == -1 {
		return curLang.missingPluralRule, errors.New(errNoPluralRuleMatches)
	}

	//Process the translation
	return l.processTranslation(
		curLang.stringsData[curLang.rules[matchingRuleIndex].startPos:curLang.rules[matchingRuleIndex+1].startPos],
		pluralCount, index, embeddedCount, args,
	)
}

// All Get...Named...() functions call this
func (l *Language) getRealNamed(namespace, translationID string, pluralCount int64, args []interface{}) (string, error) {
	if n, ok := l.dict.namespaces[namespace]; !ok {
		return retErrWithStr(errors.New("Invalid namespace"))
	} else if index, ok := n.ids[translationID]; !ok {
		return retErrWithStr(errors.New("Invalid Translation ID"))
	} else {
		return l.getReal(index, pluralCount, 0, args)
	}
}

//------------------Wrappers for getReal() [and getRealNamed()]-----------------

// Get retrieves a non-plural translation with a TransIndex.
//
// It uses either a “^” plurality rule if found, and the first plurality rule otherwise.
func (l *Language) Get(index TransIndex, args ...interface{}) (string, error) {
	return l.getReal(index, -1, 0, args)
}

// GetPlural retrieves a plural translation with a TransIndex.
//
// CurLang.MissingPluralRule is returned if a plurality rule match is not found.
func (l *Language) GetPlural(index TransIndex, pluralCount uint, args ...interface{}) (string, error) {
	return l.getReal(index, int64(pluralCount), 0, args)
}

// MustGet retrieves a non-plural translation with a TransIndex. It returns a blank string when errored.
//
// It uses either a “^” plurality rule if found, and the first plurality rule otherwise.
func (l *Language) MustGet(index TransIndex, args ...interface{}) string {
	return twoToOne(l.getReal(index, -1, 0, args))
}

// MustGetPlural retrieves a plural translation with a TransIndex. It returns a blank string when errored.
//
// CurLang.MissingPluralRule is returned if a plurality rule match is not found.
func (l *Language) MustGetPlural(index TransIndex, pluralCount uint, args ...interface{}) string {
	return twoToOne(l.getReal(index, int64(pluralCount), 0, args))
}

// GetNamed retrieves a non-plural translation with a namespace and Translation ID.
//
// It uses either a “^” plurality rule if found, and the first plurality rule otherwise.
func (l *Language) GetNamed(namespace, translationID string, args ...interface{}) (string, error) {
	return l.getRealNamed(namespace, translationID, -1, args)
}

// GetPluralNamed retrieves a plural translation with a namespace and Translation ID.
//
// CurLang.MissingPluralRule is returned if a plurality rule match is not found.
func (l *Language) GetPluralNamed(namespace string, translationID string, pluralCount uint, args ...interface{}) (string, error) {
	return l.getRealNamed(namespace, translationID, int64(pluralCount), args)
}

// MustGetNamed retrieves a non-plural translation with a namespace and Translation ID. It returns a blank string when errored.
//
// It uses either a “^” plurality rule if found, and the first plurality rule otherwise.
func (l *Language) MustGetNamed(namespace string, translationID string, args ...interface{}) string {
	return twoToOne(l.getRealNamed(namespace, translationID, -1, args))
}

// MustGetPluralNamed retrieves a plural translation with a namespace and Translation ID. It returns a blank string when errored.
//
// CurLang.MissingPluralRule is returned if a plurality rule match is not found.
func (l *Language) MustGetPluralNamed(namespace string, translationID string, pluralCount uint, args ...interface{}) string {
	return twoToOne(l.getRealNamed(namespace, translationID, int64(pluralCount), args))
}

//------------------------------------Getters-----------------------------------

// NumTranslations returns the number of translations in the language’s dictionary
func (l *Language) NumTranslations() uint32 {
	return ulen32(l.translations) - 1
}

// Name returns the name of the language
func (l *Language) Name() string {
	return l.name
}

// LanguageIdentifier returns the language identifier
func (l *Language) LanguageIdentifier() string {
	return l.languageIdentifier
}

// LanguageTag returns the LanguageTag
func (l *Language) LanguageTag() language.Tag {
	return l.languageTag
}

// FallbackName returns the fallback language identifier
func (l *Language) FallbackName() string {
	return l.fallbackName
}

// MessagePrinter returns the MessagePrinter
func (l *Language) MessagePrinter() *message.Printer {
	//Make sure the message printer already exists
	if l.messagePrinter == nil {
		l.messagePrinter = message.NewPrinter(l.languageTag)
	}

	return l.messagePrinter
}

// TimeLocalizer returns the TimeLocalizer
func (l *Language) TimeLocalizer() (*lctime.Localizer, error) {
	//Make sure the time localizer already exists
	if l.timeLocalizer == nil {
		if loc, err := lctime.NewLocalizer(strings.Replace(l.languageTag.String(), "-", "_", -1)); err != nil {
			return nil, err
		} else {
			l.timeLocalizer = &loc
		}
	}

	return l.timeLocalizer, nil
}

// TranslationIDLookup returns the Namespace name and Translation ID name from a TransIndex, separated by a dot.
//
// As this is only used for debugging purposes, this is not optimized and has to search through all of a namespace’s translations to find a match (only when read from a compiled dictionary file without the variable dictionary loaded).
func (l *Language) TranslationIDLookup(index TransIndex) (val string, ok bool) {
	if nsName, translationIDName, ok := l.dict.translationIDLookup(index); ok {
		return nsName + "." + translationIDName, true
	} else {
		return returnBlankStrOnErr, false
	}
}

//------------------------Assign a fallback to a language-----------------------

// SetFallback stores the fallback language and is required after (LanguageTextFile|LanguageBinaryFile).Load() operations.
//
// If “Settings.FallbackLanguage” was given for the parent language, the “Settings.LanguageIdentifier” of the given fallbackLanguage must match. If it was not given, fallbackLanguage must be the default language.
//
// A language cannot have itself set as its fallback. That only occurs naturally for the default language.
//
// The fallback language being set must already have its fallback language set. This is required so fallback language loops cannot occur.
func (l *Language) SetFallback(fallbackLanguage *Language) error {
	//Check for errors
	if l.fallback != nil {
		return errors.New("Fallback language already set")
	} else if fallbackLanguage == nil {
		return errors.New("Fallback language cannot be nil")
	} else if l == fallbackLanguage {
		return errors.New("Fallback language and parent language cannot be the same")
	} else if fallbackLanguage.fallback == nil {
		return fmt.Errorf("Fallback language “%s” must already have its fallback language set", fallbackLanguage.languageIdentifier)
	} else if l.dict != fallbackLanguage.dict && !bytes.Equal(l.dict.hash, fallbackLanguage.dict.hash) {
		return errors.New("Dictionaries of the two languages do not match")
	} else if l.fallbackName != fallbackLanguage.languageIdentifier {
		if l.fallbackName != "" {
			return fmt.Errorf("Fallback language identifier “%s” and parent language “%s” fallback language “%s” must match", fallbackLanguage.languageIdentifier, l.languageIdentifier, l.fallbackName)
		} else if fallbackLanguage.fallback != fallbackLanguage {
			return fmt.Errorf("Fallback language is not the default language")
		}
	}

	//Return success
	l.fallback = fallbackLanguage
	return nil
}
