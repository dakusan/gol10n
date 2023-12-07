//Write out translations to compiled (.gtr) files
//go:build !gol10n_read_compiled_only

package translate

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"strings"
	"unsafe"
)

func (dict *languageDict) toCompiledFile(_w io.Writer) error {
	//Get the number of translations
	numTranslations := uint(0)
	for _, n := range dict.namespaces {
		numTranslations += ulenm(n.ids)
	}

	//Get information from namespaces
	var translationIDsString, namespaceNamesString string
	translationIDSizes := make([]storeTranslationIDSize, numTranslations)
	writeNamespaces := make([]storeNamespace, len(dict.namespaces))
	{
		namespaceNames := make([]string, len(dict.namespaces))
		translationIDs := make([]string, len(translationIDSizes))
		for _, n := range dict.namespaces {
			namespaceNames[n.index] = n.name
			for name, index := range n.ids {
				translationIDs[index] = name
				translationIDSizes[index] = storeTranslationIDSize{uint16(len(name))}
			}
			*p2uint32p(&writeNamespaces[n.index]) = ulen32m(n.ids) & 0xFFFFFF //3 byte uint
			writeNamespaces[n.index].nameSize = uint8(len(n.name))
		}
		const joinWithNoSeparator = ""
		namespaceNamesString = strings.Join(namespaceNames, joinWithNoSeparator)
		translationIDsString = strings.Join(translationIDs, joinWithNoSeparator)
	}

	//Prepare the header for writing
	header := storeDictHeader{
		[3]byte(s2b("DTR")),
		uint32(numTranslations),
		ulen32m(dict.namespaces),
		ulen32s(translationIDsString),
		ulen32s(namespaceNamesString),
	}

	//Grow the file to its needed size (if the writer is an os.File)
	newFileSize := header.getCompiledFileSize()
	if newFileSize > math.MaxUint32 {
		return errors.New("Filesize cannot be greater than 4GB")
	}
	if f, ok := _w.(*os.File); ok {
		if err := f.Truncate(int64(newFileSize)); err != nil {
			return fmt.Errorf("Could not grow file to needed size (%d): %s", newFileSize, err)
		}
	}

	//Write out the parts of the file
	w := &countedHashedWriter{countedWriter{0, _w}, sha1.New(), dict.hash == nil}
	if err := writeDataToFile(w, &header, 1); err != nil { //Write the header
		return err
	} else if err := writeDataToFile(w, &translationIDSizes[0], header.numTranslations); err != nil { //Write out the translation id sizes
		return err
	} else if err := writeBytesToFile(w, s2b(translationIDsString)); err != nil { //Write out the translation ids
		return err
	} else if err := writeDataToFile(w, &writeNamespaces[0], header.numNamespaces); err != nil { //Write out the namespaces
		return err
	} else if err := writeBytesToFile(w, s2b(namespaceNamesString)); err != nil { //Write out the namespace names
		return err
	}

	//Make sure the newFileSize matches
	if uint64(w.bytesWritten) != newFileSize {
		return fmt.Errorf("Output file size (%d) did not match what it should (%d)", w.bytesWritten, newFileSize)
	}

	//Save the dictionary hash
	if dict.hash == nil {
		dict.hash = w.h.Sum(nil)
	}

	//Return success
	return nil
}

func (dict *languageDict) toCompiledVarFile(w io.Writer) error {
	//Can only write out variables if we actually have them
	if !dict.hasVarsLoaded {
		return errors.New("Can only write variable dictionary if the given dictionary has the variables")
	}

	//Create this as a bytes buffer for simplicity
	var b strings.Builder

	//Compiled var files are companions to the compiled dictionary files, so they will have no headers beyond the 3 byte file definition
	b.WriteString("VTR")

	//Loop through namespace and translations and output variables
	for _, namespaceName := range dict.namespacesInOrder {
		for _, t := range dict.namespaces[namespaceName].idsInOrder {
			b.WriteByte(uint8(len(t.vars)))
			for _, v := range t.vars {
				b.WriteByte(uint8(len(v.name)))
				b.WriteByte(uint8(v.varType))
				b.WriteString(v.name)
			}
		}
	}

	//Write out the result and return errors
	if n, err := w.Write(s2b(b.String())); err != nil {
		return fmt.Errorf("Failed to write %d bytes: %s", b.Len(), err.Error())
	} else if n != b.Len() {
		return fmt.Errorf("Only wrote %d of %d bytes", n, b.Len())
	}

	//Return success
	return nil
}

func (l *Language) toCompiledFile(_w io.Writer) error {
	//Check if any translation strings are larger than 64k
	translationStringByteLength := uint8(size_storeTranslationRule16)
	for i, r := range l.rules {
		if i < len(l.rules)-1 && l.rules[i+1].startPos-r.startPos > math.MaxUint16 {
			translationStringByteLength = uint8(size_storeTranslationRule32)
			break
		}
	}

	//Prepare the header for writing
	settingsString := l.getSettingsAsString()
	header := storeHeader{
		[3]byte(s2b("GTR")),
		translationStringByteLength,
		ulen32(l.rules) - 1,
		l.NumTranslations(),
		ulen32(settingsString),
		ulen32(l.stringsData),
		[20]byte(l.dict.hash),
	}

	//Grow the file to its needed size (if the writer is an os.File)
	newFileSize := header.getCompiledFileSize()
	if newFileSize > math.MaxUint32 {
		return errors.New("Filesize cannot be greater than 4GB")
	}
	if f, ok := _w.(*os.File); ok {
		if err := f.Truncate(int64(newFileSize)); err != nil {
			return fmt.Errorf("Could not grow file to needed size (%d): %s", newFileSize, err)
		}
	}

	//Write out the header and settings
	w := &countedWriter{0, _w}
	if err := writeDataToFile(w, &header, 1); err != nil {
		return err
	} else if err := writeBytesToFile(w, settingsString); err != nil {
		return err
	}

	//Write out the rules
	if header.translationStringByteLength == uint8(size_storeTranslationRule16) {
		writeRules := make([]storeTranslationRule16, header.numRules)
		for i := uint(0); i < uint(header.numRules); i++ {
			v := l.rules[i]
			writeRules[i] = storeTranslationRule16{uint16(l.rules[i+1].startPos - v.startPos), v.rule}
		}
		if err := writeDataToFile(w, &writeRules[0], header.numRules); err != nil {
			return err
		}
	} else {
		writeRules := make([]storeTranslationRule32, header.numRules)
		for i := uint(0); i < uint(header.numRules); i++ {
			v := l.rules[i]
			writeRules[i] = storeTranslationRule32{l.rules[i+1].startPos - v.startPos, v.rule}
		}
		if err := writeDataToFile(w, &writeRules[0], header.numRules); err != nil {
			return err
		}
	}

	//Write out the rule slices
	writeRuleSlices := make([]storeTranslationRuleSlice, header.numTranslations)
	for i := uint(0); i < uint(header.numTranslations); i++ {
		writeRuleSlices[i] = storeTranslationRuleSlice{uint8(l.translations[i+1].startIndex - l.translations[i].startIndex)}
	}
	if err := writeDataToFile(w, &writeRuleSlices[0], header.numTranslations); err != nil {
		return err
	}

	//Write stringsData
	if err := writeBytesToFile(w, l.stringsData); err != nil {
		return err
	}

	//Make sure the newFileSize matches
	if uint64(w.bytesWritten) != newFileSize {
		return fmt.Errorf("Output file size (%d) did not match what it should (%d)", w.bytesWritten, newFileSize)
	}

	return nil
}

func (l *Language) getSettingsAsString() []byte {
	//Determine the total length
	settingStrings := []string{
		l.name, l.languageIdentifier, l.fallbackName, l.missingPluralRule,
	}
	totalSize := ulen(settingStrings) * uint(unsafe.Sizeof(uint16(0)))
	for _, s := range settingStrings {
		totalSize += ulens(s)
	}

	//Compile the string
	str := make([]byte, 0, totalSize)
	for _, s := range settingStrings {
		strSize := uint16(len(s))
		str = append(str, any2b(&strSize)...)
		str = append(str, s2b(s)...)
	}

	return str
}

// -----------------------Write structured data to the file----------------------
func writeDataToFile[
	writeType storeHeader | storeDictHeader | storeTranslationRule16 | storeTranslationRule32 | storeTranslationRuleSlice | storeTranslationIDSize | storeNamespace,
](w io.Writer, data *writeType, numToWrite uint32) error {
	return writeBytesToFile(w, any2bLen(data, uint(numToWrite)))
}
func writeBytesToFile(w io.Writer, b []byte) error {
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("Could not write %d bytes: %s", len(b), err)
	}

	return nil
}

// -----Specialized io.Writer structs for counting bytes written and hashing-----
type countedWriter struct {
	bytesWritten uint
	w            io.Writer
}

func (w *countedWriter) Write(b []byte) (int, error) {
	num, err := w.w.Write(b)
	w.bytesWritten += uint(num)
	return num, err
}

type countedHashedWriter struct {
	countedWriter
	h             hash.Hash
	calculateHash bool
}

func (w *countedHashedWriter) Write(b []byte) (int, error) {
	if w.calculateHash {
		w.h.Write(b)
	}
	return w.countedWriter.Write(b)
}
