//Write go dictionary files
//go:build !gol10n_read_compiled_only

package translate

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

func (l *Language) toGoDictionaries(outputDirectory string) (_ error, numUpdated uint) {
	//Constants
	const (
		namespaceHashesJson      = "NamespaceHashes.json"
		translationsIDOutputFile = "TranslationIDs.go"
	)

	//Make sure this is the default language
	if l.fallback == nil {
		return errors.New("Language.SetFallback() was not called on this language"), 0
	}
	if l.fallback != l {
		return errors.New("Only the default language file can be used to write go dictionary files"), 0
	}

	//Make sure the languages were read from text (not compiled) files
	if !l.dict.hasVarsLoaded {
		return errors.New("Can only compile to go dictionaries when default language was read from translation text files"), 0
	}

	//Make sure output directory has a trailing slash
	if !strings.HasSuffix(outputDirectory, "/") {
		outputDirectory = outputDirectory + "/"
	}

	//Namespaces wait until we have read the hash file to write their output
	numNamespaces := ulenm(l.dict.namespaces)
	waitForHashes := make(chan bool)
	savedHashes := make(map[string]string, numNamespaces) //This is read after the namespace loop

	//Compile and write the different namespace
	changedNamespaceHashes := make([]string, numNamespaces) //Empty if none changed
	namespaceErrors := make(chan string)
	waitForNamespaces := sync.WaitGroup{}
	for _namespaceIndex := uint(0); _namespaceIndex < numNamespaces; _namespaceIndex++ {
		waitForNamespaces.Add(1)
		go func(namespaceIndex uint) {
			//Mark namespace as complete when function is done
			defer waitForNamespaces.Done()

			//Add the header to the namespace file
			namespaceName := l.dict.namespacesInOrder[namespaceIndex]
			builder := bytes.Buffer{}
			_, _ = fmt.Fprintf(&builder, "package %s\n\nimport \"github.com/dakusan/gol10n/translate\"\n\n//goland:noinspection NonAsciiCharacters\nconst (\n", namespaceName)

			//Write Translation IDs for this namespace
			n := l.dict.namespaces[namespaceName]
			if len(n.idsInOrder) > 0 {
				n.createGoFileConstants(l, &builder, namespaceName)
			}

			//Write the footer
			builder.Write([]byte{')', '\n'})

			//Stringify the result and get the hash
			resultStr := builder.Bytes()
			hashSumBytes := sha1.Sum(resultStr)
			hashSumString := hex.EncodeToString(hashSumBytes[:])

			//If the hash has not changed then nothing left to do
			<-waitForHashes
			if savedHashes[namespaceName] == hashSumString {
				return
			}

			//Create/confirm the directory
			outDir := outputDirectory + namespaceName + "/"
			if dirInfo, err := os.Stat(outDir); os.IsNotExist(err) {
				if err := os.Mkdir(outDir, 0644); err != nil {
					namespaceErrors <- fmt.Sprintf("Error creating namespace directory %s: %s", namespaceName, err.Error())
					return
				}
			} else if err != nil {
				namespaceErrors <- fmt.Sprintf("Error accessing namespace directory %s: %s", namespaceName, err.Error())
				return
			} else if !dirInfo.IsDir() {
				namespaceErrors <- fmt.Sprintf("Namespace directory %s: Is not a directory", namespaceName)
				return
			}

			//Write the file
			if err := os.WriteFile(outDir+translationsIDOutputFile, resultStr, 0644); err != nil {
				namespaceErrors <- fmt.Sprintf("Error writing %s for %s: %s", translationsIDOutputFile, namespaceName, err.Error())
				return
			}

			//Store the changed hash
			changedNamespaceHashes[namespaceIndex] = hashSumString
		}(_namespaceIndex)
	}

	//Get namespace hashes from namespaceHashesJson
	if getHashes, err := os.ReadFile(outputDirectory + namespaceHashesJson); err != nil {
		//If an error occurs assume we have no hashes
	} else if err := json.Unmarshal(getHashes, &savedHashes); err != nil {
		//If an error occurs assume we have no hashes
	}
	close(waitForHashes)

	//Wait for errors or all namespaces to finish
	doneWithNamespaces := make(chan struct{})
	go func() {
		waitForNamespaces.Wait()
		close(doneWithNamespaces)
	}()
	var errs []string
	for continueLoop := true; continueLoop; {
		select {
		case err := <-namespaceErrors:
			errs = append(errs, err)
		case <-doneWithNamespaces:
			continueLoop = false
		}
	}

	//Make sure error channels are exhausted
	for continueLoop := true; continueLoop; {
		select {
		case err := <-namespaceErrors:
			errs = append(errs, err)
		default:
			continueLoop = false
		}
	}

	//Update hashes on changed namespaces
	for index, namespaceName := range l.dict.namespacesInOrder {
		if len(changedNamespaceHashes[index]) != 0 && changedNamespaceHashes[index] != savedHashes[namespaceName] {
			numUpdated++
			savedHashes[namespaceName] = changedNamespaceHashes[index]
		}
	}

	//If there are changed namespaces...
	if numUpdated > 0 {
		//Write the new namespaceHashesJson file
		file, err := os.Create(outputDirectory + namespaceHashesJson)
		defer func() { _ = file.Close() }()
		if err != nil {
			errs = append(errs, "Error opening hash file for writing: "+err.Error())
		} else {
			newEncoder := json.NewEncoder(file)
			newEncoder.SetIndent("", "\t")
			if err := newEncoder.Encode(savedHashes); err != nil {
				errs = append(errs, "Error encoding to hash file: "+err.Error())
			}
		}
	}

	//Return the errors or success
	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n")), numUpdated
	} else {
		return nil, numUpdated
	}
}

func (n *namespace) createGoFileConstants(l *Language, builder *bytes.Buffer, namespaceName string) {
	//Process the translation IDs in the namespace
	firstIndex := uint(n.ids[n.idsInOrder[0].name])
	for index, translationIDAndVars := range n.idsInOrder {
		//If there are variables use a multiline comment
		hasVariables := len(translationIDAndVars.vars) > 0
		if hasVariables {
			builder.Write([]byte{'\t', '/', '*'})
		} else {
			builder.Write([]byte{'\t', '/', '/'})
		}

		//Inspections expect the constant name to be at the beginning of the comment
		builder.WriteString(translationIDAndVars.name)
		builder.WriteString(" = ")

		//Write the first translation rule as the comment
		ruleIndex := l.translations[firstIndex+uint(index)].startIndex
		builder.Write(translationIDAndVars.getTranslationWithVarsAsString(
			l.stringsData[l.rules[ruleIndex].startPos:l.rules[ruleIndex+1].startPos],
			l.dict, namespaceName,
		))

		//Write the variable names if available
		if hasVariables {
			builder.WriteString("\n\tVariableOrder = ")
			for i, v := range translationIDAndVars.vars {
				builder.WriteString(v.name)
				builder.WriteByte('[')
				builder.WriteString(variableTypeMapReverse[v.varType])
				builder.WriteByte(']')
				if i < len(translationIDAndVars.vars)-1 {
					builder.Write([]byte{',', ' '})
				}
			}

			builder.Write([]byte{'*', '/'})
		}

		//Write out the Translation ID as the constants name
		builder.Write([]byte{'\n', '\t'})
		builder.WriteString(translationIDAndVars.name)

		//Write out the iota for the first string
		if index == 0 {
			builder.WriteString(" translate.TransIndex = iota + " + strconv.FormatUint(uint64(firstIndex), 10))
		}

		//Start the next line
		builder.WriteByte('\n')

		//Add an extra line between translations
		if index != len(n.idsInOrder)-1 {
			builder.WriteByte('\n')
		}
	}
}
