//Process translation strings

package translate

import (
	"bytes"
	"fmt"
	"golang.org/x/text/currency"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Variable types
type variableType uint8 //4 bits

const ( //Note: Cannot have more than 16 types
	//Anything (Passes as %v)
	vtAnything variableType = iota

	//Strings
	vtString

	//Integers
	vtInteger
	vtBinary
	vtOctal
	vtHexLower
	vtHexUpper

	//Decimals
	vtScientific
	vtFloating

	//Other
	vtBool

	//Locale specific
	vtDateTime
	vtCurrency
	vtIntegerWithSymbols
	vtFloatWithSymbols

	//Recursive translations
	vtStaticTranslation
	vtVariableTranslation
)

// Formatting flags
const (
	fmtHasWidth     = 1 << 4
	fmtHasPrecision = 1 << 5
	fmtPadRight     = 1 << 6
	fmtPad0         = 1 << 7
)

const (
	varReplacementChar = 0xFF //Chosen because this is normally an invalid character in UTF8
)

func (l *Language) processTranslation(translation []byte, pluralCount int64, translationIDIndex TransIndex, embeddedCount uint, args []interface{}) (string, error) {
	//Create a buffer the same size as the current translation (in preparation for translations with no variables)
	var newString strings.Builder
	newString.Grow(len(translation))

	//Prepare to return errors
	insertedVarNum := 1
	varErr := func(err string, args ...interface{}) (string, error) {
		return retErrWithStr(fmt.Errorf("Inserted variable placement #"+strconv.Itoa(insertedVarNum)+" is "+err, args...))
	}

	//Consume a byte from []translation
	transLen := ulen(translation)
	translationIndex := uint(0)
	consumeByte := func(err string) (byte, error) {
		if translationIndex >= transLen {
			_, newErr := varErr(err)
			return 0, newErr
		}
		val := translation[translationIndex]
		translationIndex++
		return val, nil
	}

	for {
		//If translation is completely consumed, then stop here
		if translationIndex >= transLen {
			break
		}

		//Find the next instance of a variable. If there isn't one, then write out the rest of the string and exit
		{
			nextVarIndex := bytes.IndexByte(translation[translationIndex:], varReplacementChar)
			if nextVarIndex == -1 {
				newString.Write(translation[translationIndex:])
				break
			}

			//If there is 1 or more bytes before the variable then write them out
			if nextVarIndex > 0 {
				newString.Write(translation[translationIndex : translationIndex+uint(nextVarIndex)])
				translationIndex += uint(nextVarIndex)
			}

			//Increment to the varNum
			translationIndex++
		}

		//Get the variable index number
		var varNum uint
		if b, err := consumeByte("missing variable number specifier"); err != nil {
			return retErrWithStr(err)
		} else {
			varNum = uint(b)
		}
		if varNum > ulen(args) { //varNum is 1 based since 0 is PluralCount
			return varErr("missing variable #%d", varNum)
		}

		//Get the type+flags char
		var typeFlags uint8
		if b, err := consumeByte("missing typeFlags"); err != nil {
			return retErrWithStr(err)
		} else {
			typeFlags = b
		}

		//Compile the printf flags
		printfFlags := "%"
		if typeFlags&fmtPadRight != 0 {
			printfFlags += "-"
		}
		if typeFlags&fmtPad0 != 0 {
			printfFlags += "0"
		}

		//Set the width and precision (if given)
		if typeFlags&fmtHasWidth != 0 {
			if b, err := consumeByte("missing width"); err != nil {
				return retErrWithStr(err)
			} else if b != 0 {
				printfFlags += strconv.FormatUint(uint64(b), 10)
			}
		}
		if typeFlags&fmtHasPrecision != 0 {
			if b, err := consumeByte("missing precision"); err != nil {
				return retErrWithStr(err)
			} else {
				printfFlags += "." + strconv.FormatUint(uint64(b), 10)
			}
		}

		//Get the value for the variable
		var val interface{}
		if varNum == 0 {
			val = uint32(pluralCount)
		} else {
			val = args[varNum-1]
		}

		//Get the printf variable type
		//goland:noinspection SpellCheckingInspection
		const varTypePrintfChar = "vsdboxXeft------"
		vt := varTypePrintfChar[typeFlags&0xF]

		//If a non-special type, use sprintf to add it to our string
		if vt != '-' {
			_, _ = fmt.Fprintf(&newString, printfFlags+string(vt), val)
			insertedVarNum++
			continue
		}

		//Consume bytes from []translation
		consumeBytes := func(numBytes uint, err string) ([]byte, error) {
			if translationIndex+numBytes-1 >= transLen {
				_, newErr := varErr(err)
				return nil, newErr
			}
			val := translation[translationIndex : translationIndex+numBytes]
			translationIndex += numBytes
			return val, nil
		}

		//Handle date/times
		if variableType(typeFlags&0xF) == vtDateTime {
			//Make sure the time localizer already exists
			if _, err := l.TimeLocalizer(); err != nil {
				return varErr("date/time. Error localizing: " + err.Error())
			}

			//Get the specifier string
			var specifierStr string
			if specifierLen, err := consumeByte("missing DateTime.specifierLen"); err != nil {
				return retErrWithStr(err)
			} else if _specifierStr, err := consumeBytes(uint(specifierLen), "missing DateTime.specifier"); err != nil {
				return retErrWithStr(err)
			} else {
				specifierStr = b2s(_specifierStr)
			}

			//Localize the time
			if t, ok := val.(time.Time); !ok {
				return varErr("date/time. Variable require a time.Time object")
			} else {
				newString.WriteString((*l.timeLocalizer).Strftime(specifierStr, t))
				insertedVarNum++
				continue
			}
		}

		//Handle numeric i18n types
		var printerType byte
		switch variableType(typeFlags & 0xF) {
		case vtCurrency:
			if curVal, ok := val.(currency.Amount); !ok {
				return varErr("currency. Variable require a golang.org/x/text/currency.Amount object")
			} else {
				val = currency.Symbol(curVal)
				printerType = 'd'
			}
		case vtIntegerWithSymbols:
			printerType = 'd'
		case vtFloatWithSymbols:
			printerType = 'f'
		default:
			//Other types will be taken care of below
		}
		if printerType != 0 {
			//Localize the number
			_, _ = l.MessagePrinter().Fprintf(&newString, printfFlags+string(printerType), val)
			insertedVarNum++
			continue
		}

		//Handle embedded translations
		var newTranslationIDIndex TransIndex
		switch variableType(typeFlags & 0xF) {
		case vtStaticTranslation:
			//Get the newTranslationIDIndex
			if b, err := consumeBytes(4, "missing static translation index"); err != nil {
				return retErrWithStr(err)
			} else {
				newTranslationIDIndex = TransIndex(*p2uint32p(&b[0]))
				if uint32(newTranslationIDIndex) >= l.NumTranslations() {
					return varErr("static translation with invalid index")
				}
			}
		case vtVariableTranslation:
			//Get the newTranslationIDIndex
			switch v := val.(type) {
			case TransIndex:
				if uint32(v) >= l.NumTranslations() {
					return varErr("variable translation with invalid index")
				} else {
					newTranslationIDIndex = v
				}
			case string:
				//Split the specifier around the period into the namespace and TranslationID. If namespace is not given, assume the current namespace
				var myNamespaceName string
				translationID := s2b(v)
				if dotLoc := bytes.IndexByte(translationID, '.'); dotLoc != -1 {
					myNamespaceName = b2s(translationID[0:dotLoc])
					translationID = translationID[dotLoc+1:]
				} else {
					//Lookup the current namespace
					myNamespaceName, _, _ = l.dict.translationIDLookupNS(translationIDIndex)
				}

				//Lookup the index for the Translation ID
				if n, ok := l.dict.namespaces[myNamespaceName]; !ok {
					return varErr("variable translation with invalid namespace: %s.%s", myNamespaceName, b2s(translationID))
				} else if translationIDIndex, ok := n.ids[b2s(translationID)]; !ok {
					return varErr("variable translation with invalid Translation ID in namespace: %s.%s", myNamespaceName, b2s(translationID))
				} else {
					newTranslationIDIndex = translationIDIndex
				}
			case int, int8, int16, int32, int64:
				_v := TransIndex(reflect.ValueOf(v).Int())
				if _v < 0 || uint32(_v) >= l.NumTranslations() {
					return varErr("variable translation with invalid index")
				} else {
					newTranslationIDIndex = _v
				}
			case uint, uint8, uint16, uint32, uint64:
				_v := TransIndex(reflect.ValueOf(v).Uint())
				if uint32(_v) >= l.NumTranslations() {
					return varErr("variable translation with invalid index")
				} else {
					newTranslationIDIndex = _v
				}
			default:
				return varErr("variable translation with invalid type “%s” (must be TransIndex or string)", reflect.TypeOf(val).Name())
			}
		default:
			return varErr("unknown variable type")
		}

		//Add the translation from the index
		if embeddedStr, err := l.getReal(newTranslationIDIndex, pluralCount, embeddedCount+1, nil); err != nil {
			return varErr(
				"variable translation “%s”->“%s”:\n%s",
				twoToOne(l.TranslationIDLookup(translationIDIndex)),
				twoToOne(l.TranslationIDLookup(newTranslationIDIndex)),
				err.Error(),
			)
		} else {
			newString.WriteString(embeddedStr)
			insertedVarNum++
		}
	}

	//Return the final value. If cap-len>maxCapDiff then copy the string so cap=size
	const maxCapDiff = 1024
	if newString.Cap()-newString.Len() > maxCapDiff {
		return strings.Clone(newString.String()), nil
	}
	return newString.String(), nil
}

// As this is only used for debugging purposes, this is not optimized and has to search through all of a namespace’s translations to find a match (only when read from a compiled file).
func (dict *languageDict) translationIDLookup(index TransIndex) (namespaceName string, translationID string, ok bool) {
	//Get the namespace of the translation ID
	nsName, nsStartIndex, ok := dict.translationIDLookupNS(index)
	if !ok {
		return returnBlankStrOnErr, returnBlankStrOnErr, false
	}

	//If dict.hasVarsLoaded then this info can be looked up quickly
	if dict.hasVarsLoaded {
		return nsName, dict.namespaces[nsName].idsInOrder[uint(index)-nsStartIndex].name, true
	}

	//Search through the namespace for the translation ID
	for name, matchIndex := range dict.namespaces[nsName].ids {
		if matchIndex == index {
			return nsName, name, true
		}
	}

	return returnBlankStrOnErr, returnBlankStrOnErr, false
}

func (dict *languageDict) translationIDLookupNS(index TransIndex) (namespaceName string, namespaceStartIndex uint, ok bool) {
	//Determine the namespace by using number of translations in each namespace
	namespaceStartIndex = 0
	for _, nsName := range dict.namespacesInOrder {
		namespaceLen := ulenm(dict.namespaces[nsName].ids)
		if uint(index) < namespaceStartIndex+namespaceLen {
			return nsName, namespaceStartIndex, true
		}
		namespaceStartIndex += namespaceLen
	}
	return returnBlankStrOnErr, 0, false
}
