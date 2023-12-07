//Convert from translation text (YAML or JSON) files
//go:build !gol10n_read_compiled_only

package translate

import (
	"errors"
	"fmt"
	"golang.org/x/text/language"
	"io"
	"math"
	"regexp"
	"strings"
	"sync"
)

func (l *Language) fromTextFile(topItem tpItem, dict *languageDict, allowBigStrings bool) (errors, warnings []string) {
	//Handle errors and warnings
	//Returns errors+warnings so call to this can be used as return in parent
	addErrStr := func(err string) ([]string, []string) {
		errors = append(errors, err)

		return errors, warnings
	}
	addWarnStr := func(warn string, args ...interface{}) {
		if len(args) != 0 {
			warnings = append(warnings, fmt.Sprintf(warn, args...))
		} else {
			warnings = append(warnings, warn)
		}
	}

	//Get the top level object
	var topObj tpMap
	if _topObj, ok := topItem.getObject(); !ok {
		return addErrStr("Top level item is not an object")
	} else {
		topObj = _topObj
	}

	//Read the settings object
	isDefaultLanguage := dict == nil
	{
		var langName, missingPluralRule, fallbackLanguage, langIdentStr string
		var langIdent language.Tag
		if settingsObjInterface, ok := topObj.getValue("Settings"); !ok {
			addErrStr("Could not find settings")
		} else if settingsObj, ok := settingsObjInterface.getObject(); !ok {
			addErrStr("Settings is invalid type")
		} else {
			//Handle the LanguageIdentifier
			if _langIdent, err := getSetting(settingsObj, "LanguageIdentifier"); err != nil {
				addErrStr(err.Error())
			} else if _langIdentTag, err := language.Parse(_langIdent); err != nil {
				addErrStr("Settings.LanguageIdentifier is not valid")
			} else {
				langIdentStr = _langIdent
				langIdent = _langIdentTag
			}

			//Handle the LanguageName
			if _langName, err := getSetting(settingsObj, "LanguageName"); err != nil {
				addErrStr(err.Error())
			} else {
				langName = _langName
			}

			//Handle the MissingPluralRule
			if _missingPluralRule, err := getSetting(settingsObj, "MissingPluralRule"); err != nil {
				addErrStr(err.Error())
			} else {
				missingPluralRule = _missingPluralRule
			}

			//Handle the fallback language
			if _fallbackLanguage, err := getSetting(settingsObj, "FallbackLanguage"); err != nil {
				//Ignore error on optional variables
			} else {
				fallbackLanguage = _fallbackLanguage
			}
		}

		//If a default language does not exist then this is the default language and the dictionary needs to be created
		if !isDefaultLanguage {
			if !dict.hasVarsLoaded {
				return addErrStr("Given dictionary must have been created through translation text file")
			}
		} else {
			numNamespaces := topObj.getLength() - 1
			dict = &languageDict{make(map[string]*namespace, numNamespaces), make([]string, 0, numNamespaces), nil, true}
			if myErrors := dict.fromTextFile(topObj); len(myErrors) > 0 {
				errors = append(errors, myErrors...)
				return
			}
		}

		//Create the language for processing
		*l = Language{
			rules:              []translationRule{{0, pluralRule{cmpAll, 0}}},
			translations:       []translationRuleSlice{{0}},
			dict:               dict,
			name:               langName,
			languageIdentifier: langIdentStr,
			fallbackName:       fallbackLanguage,
			missingPluralRule:  missingPluralRule,
			languageTag:        langIdent,
		}
	}

	//Get the data from the namespaces
	namespaceReturnData := make([]struct {
		stringsData  [][][]byte
		pluralRules  [][]pluralRule
		embeddedTIDs [][]TransIndex
	}, len(l.dict.namespacesInOrder))
	{
		//Prepare to return errors and warnings from namespace go function
		errChan := make(chan string, 5)
		warnChan := make(chan string, 5)
		goAddErrStr := func(err string, args ...interface{}) {
			errChan <- fmt.Sprintf(err, args...)
		}
		goAddWarnStr := func(warn string, args ...interface{}) {
			warnChan <- fmt.Sprintf(warn, args...)
		}

		//Iterate over namespaces
		readNamespaces := topObj.toMap()
		delete(readNamespaces, "Settings")
		var waitForNamespaces sync.WaitGroup
		for namespaceIndex, _namespaceName := range l.dict.namespacesInOrder {
			//Create the namespace output structures
			namespaceName := _namespaceName
			myNamespaceReturnData := &namespaceReturnData[namespaceIndex]
			idsInOrderPointer := &l.dict.namespaces[namespaceName].idsInOrder
			myNamespaceReturnData.stringsData = make([][][]byte, len(*idsInOrderPointer))
			myNamespaceReturnData.pluralRules = make([][]pluralRule, len(*idsInOrderPointer))
			myNamespaceReturnData.embeddedTIDs = make([][]TransIndex, len(*idsInOrderPointer))

			//Get the list of translations from the namespace (and confirm the namespace name)
			var readNamespace tpMap = nil
			if getCurNamespace, ok := readNamespaces[_namespaceName]; !ok {
				addWarnStr("Namespace “%s” not found in language file", _namespaceName)
				continue
			} else if curNamespaceSlice, ok := getCurNamespace.getObject(); !ok {
				addWarnStr("Namespace “%s” could not be read", _namespaceName)
				continue
			} else {
				readNamespace = curNamespaceSlice
			}

			//Delete from the read list so that we can make sure later that all the namespaces were used
			delete(readNamespaces, _namespaceName)

			//Run each namespace processing in its own goroutine
			waitForNamespaces.Add(1)
			go func() {
				//Prepare to mark as done in namespace wait group
				defer waitForNamespaces.Done()

				//Prepare wait group for the Translation IDs
				var waitForTranslationIDs sync.WaitGroup

				//Process the translation IDs
				readTranslations := readNamespace.toMap()
				for _translationIDIndex, translationID := range *idsInOrderPointer {
					//Get the value. If it does not exist then a translation with no rules will be written
					var val tpItem = nil
					if _val, ok := readTranslations[translationID.name]; ok {
						val = _val

						//Delete from the list so that we can make sure later that all the translations were used
						delete(readTranslations, translationID.name)
					} else if isDefaultLanguage {
						goAddWarnStr("%s.%s: Default language is somehow missing namespace translation", namespaceName, translationID.name)
						continue
					} else if readNamespace != nil {
						goAddWarnStr("%s.%s: Translation is missing from namespace", namespaceName, translationID.name)
						continue
					}

					//Run each Translation ID processing in its own goroutine
					waitForTranslationIDs.Add(1)
					go func(translationIDIndex uint, translationIDName string) {
						//Mark as done in wait group
						defer waitForTranslationIDs.Done()

						//Get the properties of the Translation ID
						varProps := make([]string, 0, 2)
						if strVal, ok := val.getString(); ok {
							varProps = append(varProps, "^", strVal)
						} else if mapVal, ok := val.getObject(); ok {
							for _, mapItemVal := range mapVal.toOrdered() {
								propName := mapItemVal.getName()
								if propVal, ok := mapItemVal.getString(); !ok {
									goAddErrStr("%s.%s.%s: Must be a string", namespaceName, translationIDName, propName)
								} else {
									varProps = append(varProps, propName, propVal)
								}
							}
						} else {
							goAddErrStr("%s.%s: Invalid type: Must be a string or dictionary", namespaceName, translationIDName)
							return
						}

						//Compile the translations and store its errors, warnings, strings, and rules
						translationErrors, translationWarnings, retStrings, retPluralRules, retEmbeddedTIDs := addTranslationIDFromTextFile(varProps, namespaceName, l.dict, &(*idsInOrderPointer)[translationIDIndex], allowBigStrings)
						myNamespaceReturnData.stringsData[translationIDIndex] = retStrings
						myNamespaceReturnData.pluralRules[translationIDIndex] = retPluralRules
						myNamespaceReturnData.embeddedTIDs[translationIDIndex] = retEmbeddedTIDs
						for _, err := range translationErrors {
							goAddErrStr("%s.%s: %s", namespaceName, translationIDName, err)
						}
						for _, warn := range translationWarnings {
							goAddWarnStr("%s.%s: %s", namespaceName, translationIDName, warn)
						}

						//Add error if there are 0 rules
						if len(retPluralRules) == 0 {
							goAddErrStr("%s.%s: Translation has no rules", namespaceName, translationIDName)
						}
					}(uint(_translationIDIndex), translationID.name)
				}

				//Wait for Translation IDs go routines to complete
				waitForTranslationIDs.Wait()

				//Add warnings about extra translation IDs
				for translationID := range readTranslations {
					goAddWarnStr("%s.%s: Extra translation in namespace", namespaceName, translationID)
				}
			}()
		}

		//Add warnings about extra namespaces
		for namespaceName := range readNamespaces {
			addWarnStr("%s: Extra namespace", namespaceName)
		}

		//Wait for namespace go routines to complete, and store errors and warnings
		doneWithNamespaces := make(chan struct{})
		go func() {
			waitForNamespaces.Wait()
			close(doneWithNamespaces)
		}()
		for continueLoop := true; continueLoop; {
			select {
			case err := <-errChan:
				addErrStr(err)
			case warn := <-warnChan:
				addWarnStr(warn)
			case <-doneWithNamespaces:
				continueLoop = false
			}
		}

		//Make sure error and warning channels are exhausted
		for continueLoop := true; continueLoop; {
			select {
			case err := <-errChan:
				addErrStr(err)
			case warn := <-warnChan:
				addWarnStr(warn)
			default:
				continueLoop = false
			}
		}
	}

	//Grow the language slices to their needed sizes
	{
		totalStrLen, totalTranslations, totalRules := uint64(0), uint(0), uint(0)
		for _, nsRetData := range namespaceReturnData {
			totalTranslations += ulen(nsRetData.pluralRules)
			for translationIndex, rules := range nsRetData.pluralRules {
				totalRules += ulen(rules)
				for _, str := range nsRetData.stringsData[translationIndex] {
					totalStrLen += uint64(len(str))
				}
			}
		}
		l.stringsData = make([]byte, totalStrLen)
		l.rules = make([]translationRule, totalRules+1)
		l.translations = make([]translationRuleSlice, totalTranslations+1)
	}

	//Compile the data from the namespaces into the language
	{
		curStrIndex, curTranslationIndex, curRuleIndex := uint32(0), uint32(1), uint32(1)
		for _, nsRetData := range namespaceReturnData {
			for translationIndex, rules := range nsRetData.pluralRules {
				for ruleIndex, rule := range rules {
					//Save the string to the strings data list
					newStr := nsRetData.stringsData[translationIndex][ruleIndex]
					newStrLen := ulen32(newStr)
					copy(l.stringsData[curStrIndex:curStrIndex+newStrLen], newStr)
					curStrIndex += newStrLen

					//Store the rule
					l.rules[curRuleIndex-1].rule = rule
					l.rules[curRuleIndex].startPos = curStrIndex
					curRuleIndex++
				}

				//Store the translation rule slice
				l.translations[curTranslationIndex].startIndex = curRuleIndex - 1
				curTranslationIndex++
			}
		}
	}

	//Iterate over all translation strings with embedded static translations for looped recursion
	{
		//Build a map of embedded TIDs in Translation IDs
		embeddedTIDs := make(map[TransIndex][]TransIndex)
		_TIDNames := make(map[TransIndex]string)
		for _, n := range l.dict.namespaces {
			//Skip empty namespaces
			if len(n.idsInOrder) == 0 {
				continue
			}

			startTID := n.ids[n.idsInOrder[0].name]
			for indexTID, listTID := range namespaceReturnData[n.index].embeddedTIDs {
				if listTID != nil {
					embeddedTIDs[startTID+TransIndex(indexTID)] = listTID
					_TIDNames[startTID+TransIndex(indexTID)] = n.name + "." + n.idsInOrder[indexTID].name
				}
			}
		}

		//Recurse through embeddedTID links to find looped recursion
		var recurseTIDs func(curTID TransIndex, curList []TransIndex, newList []TransIndex) (errList []TransIndex)
		recurseTIDs = func(curTID TransIndex, curList []TransIndex, newList []TransIndex) (errList []TransIndex) {
			//Create a list that includes the current item
			myList := make([]TransIndex, len(curList)+1)
			copy(myList, curList)
			myList[len(curList)] = curTID

			//Check if curTID is already in curList, or >maxEmbeddedCount, and return error if so
			if len(myList) > maxEmbeddedCount || arrayIn(curList, curTID) {
				return myList
			}

			//Iterate over the new list
			for _, newTID := range newList {
				if getNewList, ok := embeddedTIDs[newTID]; ok {
					if _errList := recurseTIDs(newTID, myList, getNewList); _errList != nil {
						return _errList
					}
				}
			}

			return nil
		}

		//Check each translation for a loop recursion
		for curTID, listTID := range embeddedTIDs {
			if _errList := recurseTIDs(curTID, nil, listTID); _errList != nil {
				//Build the list of TID names that have loop recursion
				names := make([]string, len(_errList))
				for i, s := range _errList {
					names[i] = _TIDNames[s]
				}

				//Return the proper error
				if len(_errList) > maxEmbeddedCount {
					return addErrStr(fmt.Sprintf("Max embedded translation nested level (%d) reached: %s", maxEmbeddedCount, strings.Join(names, " -> ")))
				} else {
					return addErrStr("Found embedded translation loop: " + strings.Join(names, " -> "))
				}
			}
		}
	}

	//Check if any of the translation strings require storeTranslationRule32
	translationStringByteLength := size_storeTranslationRule16
	for i := 0; i < len(l.rules)-1; i++ {
		if l.rules[i+1].startPos-l.rules[i].startPos > math.MaxUint16 {
			translationStringByteLength = size_storeTranslationRule32
			break
		}
	}

	//Check the soft caps
	settingsStringLen := len(l.getSettingsAsString())
	if err := checkFor32BitOverflow(len(l.rules), len(l.translations), settingsStringLen, len(l.stringsData)); err != nil {
		return addErrStr(err.Error())
	}
	header := storeHeader{
		[3]byte{}, uint8(translationStringByteLength),
		ulen32(l.rules) - 1, l.NumTranslations(),
		uint32(settingsStringLen), ulen32(l.stringsData), [20]byte{},
	}
	if err := header.checkSoftCaps(); err != nil {
		addErrStr(err.Error())
	}

	//Make sure the resultant golang file won't be too large
	if header.getCompiledFileSize() > math.MaxUint32 {
		return addErrStr("Final file size cannot be larger than 4GB")
	}

	return
}

func (dict *languageDict) fromTextFile(readNamespaces tpMap) (errors []string) {
	//Handle errors
	addErrStr := func(err string, args ...interface{}) {
		if len(args) != 0 {
			errors = append(errors, fmt.Sprintf(err, args...))
		} else {
			errors = append(errors, err)
		}
	}

	//Read the namespaces
	var numTranslations, namespacesSize, idsSize uint64
	regexMatchNamespaceName := regexp.MustCompile(`^\w+$`)
	regexMatchTranslationID := regexp.MustCompile(`^[A-Z][\pL\pN_]*$`)
	for _, itemVal := range readNamespaces.toOrdered() {
		//Check the namespace
		namespaceName := itemVal.getName()
		if namespaceName == "Settings" {
			continue
		} else if len(namespaceName) > math.MaxUint8 {
			addErrStr("Namespace “%s” cannot be longer than 255 bytes", namespaceName)
			continue
		} else if !regexMatchNamespaceName.MatchString(namespaceName) {
			addErrStr("Namespace “%s” can only contain alphanumeric and underscore characters", namespaceName)
			continue
		} else if namespaceName[0] >= '0' && namespaceName[0] <= '9' {
			addErrStr("Namespace “%s” cannot start with a digit", namespaceName)
			continue
		} else if _, ok := dict.namespaces[namespaceName]; ok {
			addErrStr("Namespace “%s” used more than once", namespaceName)
			continue
		}

		//Get the list of translation IDs
		var idsList tpMap
		if _translationIDs, ok := itemVal.getObject(); !ok {
			addErrStr("Namespace “%s” is not a dictionary", namespaceName)
			continue
		} else {
			idsList = _translationIDs
		}

		//Create the namespace
		myNamespace := namespace{
			namespaceName,
			ulenm(dict.namespaces),
			make(translationIDs, idsList.getLength()),
			make([]translationIDNameAndVars, 0, idsList.getLength()),
		}
		dict.namespaces[namespaceName] = &myNamespace
		dict.namespacesInOrder = append(dict.namespacesInOrder, namespaceName)
		namespacesSize += uint64(len(namespaceName))

		//Store the indexes for each Translation ID
		for _, val := range idsList.toOrdered() {
			//Check the Translation ID
			translationID := val.getName()
			if len(translationID) > math.MaxUint16 {
				addErrStr("%s.%s: Must be smaller than 64KB", namespaceName, translationID)
			} else if translationID[0] < 'A' || translationID[0] > 'Z' {
				addErrStr("%s.%s: Must start with an upper case character (A-Z)", namespaceName, translationID)
			} else if !regexMatchTranslationID.MatchString(translationID) {
				addErrStr("%s.%s: Can only contain unicode letters, unicode numbers, and underscores", namespaceName, translationID)
			} else if _, ok := myNamespace.ids[translationID]; ok {
				addErrStr("%s.%s: Used more than once", namespaceName, translationID)
			} else {
				//Store the index
				myNamespace.ids[translationID] = TransIndex(numTranslations)
				myNamespace.idsInOrder = append(myNamespace.idsInOrder, translationIDNameAndVars{translationID, nil})
				numTranslations++
				idsSize += uint64(len(translationID))
			}
		}
	}

	//Check soft and hard caps
	header := storeDictHeader{
		[3]byte{}, uint32(numTranslations),
		ulen32m(dict.namespaces), uint32(idsSize), uint32(namespacesSize),
	}
	if err := checkFor32BitOverflow(numTranslations, uint64(len(dict.namespaces)), namespacesSize, idsSize); err != nil {
		addErrStr(err.Error())
	} else if err := header.checkSoftCaps(); err != nil {
		addErrStr(err.Error())
	} else if header.getCompiledFileSize() > math.MaxUint32 {
		addErrStr("Final dictionary file size cannot be larger than 4GB")
	} else {
		//Get the dictionary hash
		_ = dict.toCompiledFile(io.Discard)
	}

	return
}

// -------------------Interface to access text processing maps-------------------
type tpMap interface {
	getValue(string) (val tpItem, ok bool)
	toMap() map[string]tpItem
	toOrdered() []tpItem
	getLength() uint
}
type tpItem interface {
	getName() string
	getObject() (val tpMap, ok bool)
	getString() (val string, ok bool)
}

// ----------------------------Other helper functions----------------------------
func getSetting(settingsObj tpMap, settingName string) (string, error) {
	if settingInterface, ok := settingsObj.getValue(settingName); !ok {
		return retErrWithStr(fmt.Errorf("Settings.%s %s", settingName, "is missing"))
	} else if strVal, ok := settingInterface.getString(); !ok {
		return retErrWithStr(fmt.Errorf("Settings.%s %s", settingName, "is not a string"))
	} else if len(strVal) == 0 {
		return retErrWithStr(fmt.Errorf("Settings.%s %s", settingName, "is blank"))
	} else {
		return strVal, nil
	}
}

func checkFor32BitOverflow[T int | uint64](values ...T) error {
	for _, v := range values {
		if v > math.MaxUint32 {
			return errors.New("Uint32 overflow occurred")
		}
	}
	return nil
}
