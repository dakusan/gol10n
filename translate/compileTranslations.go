//Compile translation strings
//go:build !gol10n_read_compiled_only

package translate

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// These are filled in on first use
var variableTypeMap map[string]variableType
var variableTypeMapReverse []string
var regexMatchVariableName, regexReplaceVariables, regexVariableFlags, regexSpecialCharacters, regexMatchEmbeddedStaticVariable *regexp.Regexp

// Note: Initializing this is not concurrency safe
func initTextProcessing() {
	if variableTypeMap != nil {
		return
	}

	//Fill in variable type maps
	variableTypeMapValues := []variableType{vtAnything, vtString, vtInteger, vtBinary, vtOctal, vtHexLower, vtHexUpper, vtScientific, vtFloating, vtBool, vtDateTime, vtCurrency, vtIntegerWithSymbols, vtFloatWithSymbols, vtStaticTranslation, vtVariableTranslation}
	variableTypeMapNames := []string{"Anything", "String", "Integer", "Binary", "Octal", "HexLower", "HexUpper", "Scientific", "Floating", "Bool", "DateTime", "Currency", "IntegerWithSymbols", "FloatWithSymbols", "StaticTranslation", "VariableTranslation"}
	variableTypeMap = make(map[string]variableType, len(variableTypeMapValues))
	variableTypeMapReverse = make([]string, len(variableTypeMapValues))
	for i, v := range variableTypeMapValues {
		variableTypeMap[strings.ToUpper(variableTypeMapNames[i])] = v
		variableTypeMapReverse[v] = variableTypeMapNames[i]
	}

	//Regular expressions
	regexMatchVariableName = regexp.MustCompile(`^[\pL\pN_]+$`)
	regexReplaceVariables = regexp.MustCompile(`\{\{\.\s*([\pL\pN_]+)\s*(?:\|\s*(.*?))?\s*(?:!\s*(.*?))?\s*}}`)
	regexVariableFlags = regexp.MustCompile(`^(-?)\s*(0?)\s*(\d{0,8})\s*(?:\.\s*(\d{1,8}))?\s*$`)
	//goland:noinspection SpellCheckingInspection
	regexSpecialCharacters = regexp.MustCompile(`(?i)\\(?:[abfnrtv\\]|x[0-9a-f]{2}|u[0-9a-f]{2,6})`)
	regexMatchEmbeddedStaticVariable = regexp.MustCompile(`\{\{\*\s*([\pL\pN_]+)\s*(?:\.\s*([\pL\pN_]+))?\s*}}`)
}

func addTranslationIDFromTextFile(props []string, namespaceName string, dict *languageDict, vars *translationIDNameAndVars, allowBigStrings bool) (errors []string, warnings []string, retStrings [][]byte, retPluralRules []pluralRule, retEmbeddedTIDs []TransIndex) {
	//Handle errors and warnings
	addErrStr := func(err string, args ...interface{}) {
		if len(args) != 0 {
			errors = append(errors, fmt.Sprintf(err, args...))
		} else {
			errors = append(errors, err)
		}
	}
	addWarnStr := func(warn string, args ...interface{}) {
		warnings = append(warnings, fmt.Sprintf(warn, args...))
	}

	//Property variables
	type varProp struct {
		index  uint8
		myType variableType
	}
	varProps := map[string]varProp{
		"PluralCount": {0, vtIntegerWithSymbols}, //PluralCount is always variable #0
	}

	//Plural Rules
	type tempPluralRule struct {
		value string
		rule  pluralRule
	}
	myRules := make([]tempPluralRule, 0, 1)

	{
		//Process the properties
		isDefaultLanguage := vars.vars == nil
		tooManyPlural, numVars := false, 0 //Only show overflow errors once
		for i := 0; i < len(props); i += 2 {
			//Properties are stored in tuples
			propName, propVal := props[i], props[i+1]

			//Properties are determined by their first character
			switch propName[0] {
			//Ignore \ properties
			case '\\':

			//Parse as operator
			case '^', '=', '<', '>', '~':
				//Get the rule
				var rule pluralRule
				if _rule, err := createPluralRule(propName); err != nil {
					addErrStr("“%s”: %s", propName, err)
					continue
				} else {
					rule = _rule
				}

				//Make sure there aren't too many rules
				if len(myRules) >= 255 {
					if !tooManyPlural {
						addErrStr("Cannot have more than 255 plural rules")
					}
					tooManyPlural = true
					continue
				}

				//Store the rule for processing
				myRules = append(myRules, tempPluralRule{propVal, rule})
			//Parse as variable
			default:
				//Confirm there are not too many variables
				numVars++
				if numVars == 256 {
					addErrStr("Cannot have more than 255 variables")
					continue
				}

				//Confirm variable validity
				if !regexMatchVariableName.MatchString(propName) {
					addErrStr("“%s” is not a valid variable name", propName)
					continue
				} else if len(propName) > 255 {
					addErrStr("“%s” variable name cannot be longer than 255 bytes", propName)
					continue
				} else if _, exists := varProps[propName]; exists {
					addErrStr("“%s” was declared more than once", propName)
					continue
				}

				//Get variable type
				var varType variableType
				if _varType, ok := variableTypeMap[strings.ToUpper(propVal)]; !ok || _varType == vtStaticTranslation {
					addErrStr("“%s” has an invalid variable type “%s”", propName, propVal)
					continue
				} else {
					varType = _varType
				}

				//Save the variable to the list
				varIndex := len(varProps)
				varProps[propName] = varProp{uint8(varIndex), varType}

				//If the default language then add the variable to the list. If not, then make sure the variable names and types match
				if isDefaultLanguage {
					vars.vars = append(vars.vars, translationIDVar{propName, varType})
				} else if varIndex > len(vars.vars) {
					addWarnStr("Variable #%d does not exist in the default language", varIndex)
				} else if defaultVar := vars.vars[varIndex-1]; defaultVar.name != propName || defaultVar.varType != varType {
					addWarnStr("Variable #%d does not match the default language", varIndex)
				}
			}
		}

		//Issue a warning if the number of variables does not match the default language
		if !isDefaultLanguage && numVars != len(vars.vars) {
			addWarnStr("Number of variables (%d) does not match the default language (%d)", numVars, len(vars.vars))
		}
	}

	//Process the found plural rules
	for ruleNum, ruleVal := range myRules {
		//Replace {{.VarName}} variables with binary format
		var ruleErrors []string
		varNum := 0
		finalStr := regexReplaceVariables.ReplaceAllFunc(s2b(ruleVal.value), func(varVal []byte) []byte {
			//Pull the regex group indexes
			parts := regexReplaceVariables.FindSubmatchIndex(varVal)
			varName := b2s(varVal[parts[2]:parts[3]])
			var varFlags []byte
			if parts[4] != -1 {
				varFlags = varVal[parts[4]:parts[5]]
			}

			//Add an error to return
			varNum++
			addRuleErrStr := func(err string, args ...interface{}) {
				ruleErrors = append(ruleErrors, fmt.Sprintf("Var #%d “%s”: "+err, append([]interface{}{varNum, varName}, args...)...))
			}

			//Get the variable info
			var varInfo varProp
			if _varInfo, ok := varProps[varName]; !ok {
				addRuleErrStr("Unknown variable found")
			} else {
				varInfo = _varInfo
			}

			//Get the flags
			flags := regexVariableFlags.FindSubmatchIndex(varFlags)
			if flags == nil {
				addRuleErrStr("Flags “%s” are invalid", varFlags)
				flags = []int{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
			}
			hasPadRight := flags[2] != flags[3]
			hasPad0 := flags[4] != flags[5]
			var optWidth, optPrecision string
			if flags[6] != flags[7] {
				optWidth = b2s(varFlags[flags[6]:flags[7]])
			}
			if flags[8] != flags[9] {
				optPrecision = b2s(varFlags[flags[8]:flags[9]])
			}

			//Build the output byte struct
			outputRet := make([]byte, 3, 5)
			flagsByte := uint8(varInfo.myType)
			outputRet[0] = varReplacementChar
			outputRet[1] = varInfo.index

			//Process the flags
			if hasPadRight {
				flagsByte |= fmtPadRight
			}
			if hasPad0 {
				flagsByte |= fmtPad0
			}
			checkWidth := func(val, varName string, flagByte uint8) {
				if len(val) == 0 {
					return
				}

				if width, err := strconv.Atoi(val); err != nil {
					addRuleErrStr("Has an invalid %s: %s", varName, val)
				} else if width > 255 {
					addRuleErrStr("The %s (%d) cannot be greater than 255", varName, width)
				} else {
					flagsByte |= flagByte
					outputRet = append(outputRet, byte(width))
				}
			}
			checkWidth(optWidth, "width", fmtHasWidth)
			checkWidth(optPrecision, "precision", fmtHasPrecision)
			outputRet[2] = flagsByte

			//For DateTimes, save the specifier after the colon
			if varInfo.myType == vtDateTime {
				//Confirm the specifier
				if parts[6] == parts[7] {
					addRuleErrStr("This variable type (%s) requires a specifier (a value after an exclamation mark)", variableTypeMapReverse[varInfo.myType])
				} else if parts[7]-parts[6] > 255 {
					addRuleErrStr("This variable type (%s) specifier cannot be more than 255 bytes", variableTypeMapReverse[varInfo.myType])
				} else {
					//Write the specifier length and string
					outputRet = append(outputRet, byte(parts[7]-parts[6]))
					outputRet = append(outputRet, varVal[parts[6]:parts[7]]...)
				}
			}

			return outputRet
		})

		//Replace special characters
		finalStr = regexSpecialCharacters.ReplaceAllFunc(finalStr, func(varVal []byte) []byte {
			var newChar byte
			switch bytes.ToLower(varVal[1:2])[0] {
			case 'a':
				newChar = '\a'
			case 'b':
				newChar = '\b'
			case 'f':
				newChar = '\f'
			case 'n':
				newChar = '\n'
			case 'r':
				newChar = '\r'
			case 't':
				newChar = '\t'
			case 'v':
				newChar = '\v'
			case '\\':
				newChar = '\\'
			//Handle 2 digit hex character
			case 'x':
				newCharCode, _ := strconv.ParseInt(b2s(varVal[2:4]), 16, 0)
				if newCharCode == varReplacementChar {
					ruleErrors = append(ruleErrors, "Cannot use \\xFF as it is a special character in this library")
					return nil
				}

				return []byte{byte(newCharCode)}
			//Handle 4 digit unicode character
			case 'u':
				//Confirm character is within the valid unicode range
				newCharCode, _ := strconv.ParseInt(b2s(varVal[2:]), 16, 0)
				if newCharCode > unicode.MaxRune {
					ruleErrors = append(ruleErrors, "Invalid unicode character found. Is >unicode.MaxRune: 0x"+string(varVal[2:]))
					return nil
				}

				//Confirm the unicode character is a valid code point
				if !utf8.ValidRune(rune(newCharCode)) {
					ruleErrors = append(ruleErrors, "Invalid unicode character found: 0x"+string(varVal[2:]))
					return nil
				}

				//Return the valid character
				return []byte(string(rune(newCharCode)))
			default:
				ruleErrors = append(ruleErrors, "Invalid escaped character found after slash: "+string(varVal[1:]))
				return nil
			}

			return []byte{newChar}
		})

		//Replace static translations
		finalStr = regexMatchEmbeddedStaticVariable.ReplaceAllFunc(finalStr, func(varVal []byte) []byte {
			//If there is a second specifier then consider the first specifier the namespace
			parts := regexMatchEmbeddedStaticVariable.FindSubmatchIndex(varVal)
			myNamespaceName := namespaceName
			translationID := b2s(varVal[parts[2]:parts[3]])
			if parts[4] != parts[5] {
				myNamespaceName = translationID
				translationID = b2s(varVal[parts[4]:parts[5]])
			}

			//Lookup the index for the Translation ID
			if n, ok := dict.namespaces[myNamespaceName]; !ok {
				ruleErrors = append(ruleErrors, fmt.Sprintf("Invalid namespace for specifier %s.%s", myNamespaceName, translationID))
			} else if translationIDIndex, ok := n.ids[translationID]; !ok {
				ruleErrors = append(ruleErrors, fmt.Sprintf("Invalid Translation ID in namespace for specifier %s.%s", myNamespaceName, translationID))
			} else {
				//Store as a variable
				ret := []byte{
					varReplacementChar, 0, byte(vtStaticTranslation), //varReplacementChar, varIndex=0 (Unused), Type
					0, 0, 0, 0, //4 byte Translation ID Index
				}
				*(*TransIndex)(p2uint32p(&ret[3])) = translationIDIndex

				//Add to embedded translation IDs if not already in it
				if !arrayIn(retEmbeddedTIDs, translationIDIndex) {
					retEmbeddedTIDs = append(retEmbeddedTIDs, translationIDIndex)
				}

				return ret
			}

			//Return nothing on error
			return nil
		})

		//Propagate rule errors to parent
		for _, err := range ruleErrors {
			addErrStr("Rule #%d %s", ruleNum+1, err)
		}

		//Make sure the string length is not too long
		if (!allowBigStrings && len(finalStr) > math.MaxUint16) || len(finalStr) > math.MaxUint32 {
			addErrStr("Rule #%d is too long", ruleNum+1)
			finalStr = []byte{}
		}

		//Save the data for return
		retStrings = append(retStrings, finalStr)
		retPluralRules = append(retPluralRules, ruleVal.rule)
	}

	return
}

func (tv *translationIDNameAndVars) getTranslationWithVarsAsString(startStr []byte, dict *languageDict, namespaceName string) []byte {
	//Consume 1 or more bytes
	var outStr bytes.Buffer
	startStrLen := ulen(startStr)
	startStrPos := uint(0)
	consumeByte := func() (consumedByte byte, error []byte) {
		if startStrPos >= startStrLen {
			return 0, s2b("STRING_ENDED_EARLY")
		}
		consumedByte = startStr[startStrPos]
		startStrPos++
		return
	}
	consumeBytes := func(numBytes uint) (retVal []byte, retErr []byte) {
		if startStrPos+numBytes-1 >= startStrLen {
			retErr = s2b("STRING_ENDED_EARLY")
		} else {
			retVal = startStr[startStrPos : startStrPos+numBytes]
		}
		startStrPos += numBytes
		return
	}

	//Loop until no more bytes to consume
	for {
		//Consume a character
		if startStrPos >= startStrLen {
			break
		}
		firstChar, _ := consumeByte()

		//Escape characters <' ' (32)
		if firstChar < ' ' {
			var newChar byte
			switch firstChar {
			case '\a':
				newChar = 'a'
			case '\b':
				newChar = 'b'
			case '\f':
				newChar = 'f'
			case '\n':
				newChar = 'n'
			case '\r':
				newChar = 'r'
			case '\t':
				newChar = 't'
			case '\v':
				newChar = 'v'
			default:
				outStr.WriteString(fmt.Sprintf("\\x%02x", firstChar))
				continue
			}
			outStr.Write([]byte{'\\', newChar})
			continue
		}

		//If a slash followed by a (possibly) escapable character, escape the slash
		if firstChar == '\\' && startStrPos < startStrLen && bytes.IndexByte([]byte{'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', 'x', 'u'}, startStr[startStrPos]) != -1 {
			nextByte, _ := consumeByte()
			outStr.Write([]byte{firstChar, firstChar, nextByte})
			if nextByte == '\\' {
				outStr.WriteByte(nextByte)
			}
			continue
		}

		//If not a variable, just output the character
		if firstChar != varReplacementChar {
			outStr.WriteByte(firstChar)
			continue
		}

		//Handle static translations
		if startStrPos+2 <= startStrLen && variableType(startStr[startStrPos+1]&0xF) == vtStaticTranslation {
			//Write the insertion header
			outStr.Write([]byte{'{', '{', '*'})

			//Get the Translation ID
			startStrPos += 2 //Skip the variable name index and variableType
			if _translationID, err := consumeBytes(4); err != nil {
				outStr.Write(err)
			} else {
				//Search the namespaces for the Translation ID
				translationID := TransIndex(*p2uint32p(&_translationID[0]))
				staticTranslationName := "NOT_FOUND"
				if nsName, translationIDName, ok := dict.translationIDLookup(translationID); ok {
					if nsName != namespaceName {
						staticTranslationName = nsName + "." + translationIDName
					} else {
						staticTranslationName = translationIDName
					}
				}

				//Write out the found name
				outStr.WriteString(staticTranslationName)
			}

			//Write variable insertion footer
			outStr.Write([]byte{'}', '}'})
			continue
		}

		//Write the variable insertion header
		outStr.Write([]byte{'{', '{', '.'})

		//Write the variable name from its index
		if varIndex, err := consumeByte(); err != nil {
			outStr.Write(err)
		} else if varIndex == 0 {
			outStr.WriteString("PluralCount")
		} else if uint(varIndex)-1 < ulen(tv.vars) {
			outStr.WriteString(tv.vars[varIndex-1].name)
		} else {
			outStr.WriteString("ERROR_BAD_VAR_INDEX")
		}

		//Get the flag byte
		var flagByte byte
		if _flagByte, err := consumeByte(); err != nil {
			outStr.Write(err)
		} else {
			flagByte = _flagByte
		}

		//Handle flags
		if flagByte&0xF0 > 0 {
			outStr.WriteByte('|')
			if flagByte&fmtPadRight > 0 {
				outStr.WriteByte('-')
			}
			if flagByte&fmtPad0 > 0 {
				outStr.WriteByte('0')
			}
			if flagByte&fmtHasWidth > 0 {
				if width, err := consumeByte(); err != nil {
					outStr.Write(err)
				} else {
					outStr.WriteString(strconv.FormatUint(uint64(width), 10))
				}
			}
			if flagByte&fmtHasPrecision > 0 {
				if precision, err := consumeByte(); err != nil {
					outStr.Write(err)
				} else {
					outStr.WriteString("." + strconv.FormatUint(uint64(precision), 10))
				}
			}
		}

		//Handle DateTime
		if variableType(flagByte&0xF) == vtDateTime {
			//Pull and write the specifier
			if specifierLen, err := consumeByte(); err != nil {
				outStr.Write(err)
			} else if specifier, err := consumeBytes(uint(specifierLen)); err != nil {
				outStr.Write(err)
			} else {
				outStr.WriteByte('!')
				outStr.Write(specifier)
			}
		}

		//Write variable insertion footer
		outStr.Write([]byte{'}', '}'})
	}

	return outStr.Bytes()
}
