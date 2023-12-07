//Read translations from compiled (.gtr) files

package translate

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"io"
	"math"
	"unsafe"
)

// Main storage structures
type storeHeader struct {
	fileType                    [3]byte //GTR
	translationStringByteLength uint8   //4 or 8 for storeTranslationRule16 or storeTranslationRule32
	numRules, numTranslations   uint32
	settingsSize, dataSize      uint32
	hash                        [20]byte
}
type storeTranslationRule16 struct {
	length uint16
	rule   pluralRule
}
type storeTranslationRule32 struct {
	length uint32
	rule   pluralRule
	//2 bytes unused
}
type storeTranslationRuleSlice struct {
	length byte
}

// Dictionary file storage structures
type storeDictHeader struct {
	fileType                       [3]byte //DTR
	numTranslations, numNamespaces uint32
	idsSize, namespacesSize        uint32
}
type storeTranslationIDSize struct {
	length uint16
}
type storeNamespace struct {
	length   [3]byte
	nameSize uint8
}

// getLength : Access the length of the struct, since go does not support accessing a generic struct field x.f
type getLength interface {
	getLength() uint32
}

func (v storeTranslationRule16) getLength() uint32    { return uint32(v.length) }
func (v storeTranslationRule32) getLength() uint32    { return v.length }
func (v storeTranslationRuleSlice) getLength() uint32 { return uint32(v.length) }
func (v storeTranslationIDSize) getLength() uint32    { return uint32(v.length) }
func (v storeNamespace) getLength() uint32 {
	return *p2uint32p(&v.length) & 0xFFFFFF
}

//goland:noinspection GoSnakeCaseUsage
const (
	//Struct sizes
	size_storeTranslationRule16    = uint32(unsafe.Sizeof(storeTranslationRule16{}))
	size_storeTranslationRule32    = uint32(unsafe.Sizeof(storeTranslationRule32{}))
	size_storeTranslationRuleSlice = uint32(unsafe.Sizeof(storeTranslationRuleSlice{}))
	size_storeTranslationIDsSize   = uint32(unsafe.Sizeof(storeTranslationIDSize{}))
	size_storeNamespace            = uint32(unsafe.Sizeof(storeNamespace{}))

	//Soft limits
	softLimit_numNamespaces       = 1_000
	softLimit_idsSize             = 1024 * 1024 * 32
	softLimit_namespacesSize      = 1024 * 1024
	softLimit_numTranslationRules = 1_000_000
	softLimit_numTranslations     = 1_000_000
	softLimit_settingsSize        = 1024 * 1024
	softLimit_dataSize            = uint32(1024 * 1024 * 1024 * 3.5) //3.5GB

	ErrDictionaryDoesNotMatch = "Dictionary does not match"
)

// Get compiled (binary) file sizes
func (header storeHeader) getCompiledFileSize() uint64 {
	return uint64(unsafe.Sizeof(header)) +
		uint64(header.numRules)*uint64(header.translationStringByteLength) +
		uint64(header.numTranslations)*uint64(size_storeTranslationRuleSlice) +
		uint64(header.settingsSize) + uint64(header.dataSize)
}
func (header storeDictHeader) getCompiledFileSize() uint64 {
	return uint64(unsafe.Sizeof(header)) +
		uint64(header.numTranslations)*uint64(size_storeTranslationIDsSize) +
		uint64(header.numNamespaces)*uint64(size_storeNamespace) +
		uint64(header.idsSize) + uint64(header.namespacesSize)
}

// Check soft size caps
func (header storeHeader) checkSoftCaps() error {
	for _, v := range []struct {
		sizePointer uint32
		maxSize     uint32
		varName     string
	}{
		{header.numRules, softLimit_numTranslationRules, "Num Translation Rules"},
		{header.numTranslations, softLimit_numTranslations, "Num Translations"},
		{header.settingsSize, softLimit_settingsSize, "Settings Size"},
		{header.dataSize, softLimit_dataSize, "Data size"},
	} {
		if v.sizePointer > v.maxSize {
			return errors.New(message.NewPrinter(language.English).Sprintf("%s cannot be larger than %d", v.varName, v.maxSize))
		}
	}

	return nil
}
func (header storeDictHeader) checkSoftCaps() error {
	for _, v := range []struct {
		sizePointer uint32
		maxSize     uint32
		varName     string
	}{
		{header.numTranslations, softLimit_numTranslations, "Num Translations"},
		{header.numNamespaces, softLimit_numNamespaces, "Num Namespaces"},
		{header.idsSize, softLimit_idsSize, "IDs Size"},
		{header.namespacesSize, softLimit_namespacesSize, "Namespaces Size"},
	} {
		if v.sizePointer > v.maxSize {
			return errors.New(message.NewPrinter(language.English).Sprintf("%s cannot be larger than %d", v.varName, v.maxSize))
		}
	}

	return nil
}

func init() {
	//Make sure hard limits added together are under 4gb
	if (storeHeader{
		[3]byte{}, uint8(size_storeTranslationRule32), //Assumes 8 byte translation string sizes for safety
		softLimit_numTranslationRules, softLimit_numTranslations,
		softLimit_settingsSize, softLimit_dataSize, [20]byte{},
	}).getCompiledFileSize() > math.MaxUint32 {
		panic("Translation soft limits could overflow to >4GB")
	}
	if (storeDictHeader{
		[3]byte{}, softLimit_numTranslationRules, softLimit_numNamespaces,
		softLimit_idsSize, softLimit_namespacesSize,
	}).getCompiledFileSize() > math.MaxUint32 {
		panic("Translation soft limits could overflow dictionary to >4GB")
	}
}

func (dict *languageDict) fromCompiledFile(r io.Reader) error {
	//Handle reading the binary file
	var numBytesRead, prevBytesRead uint32 = 0, 0
	hashOfFile := sha1.New()
	readBytes := func(bytes []byte) error {
		prevBytesRead = numBytesRead
		numBytesRead += ulen32(bytes)

		if bytesRead, err := r.Read(bytes); err != nil {
			if err == io.EOF {
				//This is not actually an EOF error if bytesRead==len(bytes)
				if bytesRead == len(bytes) {
					hashOfFile.Write(bytes)
					return nil
				}

				return errors.New("File ended early")
			}
			return err
		} else if bytesRead < len(bytes) {
			return errors.New("File ended early")
		}

		hashOfFile.Write(bytes)
		return nil
	}

	//Handle returning errors
	retErrStr := func(err string, Location uint32) error { return fmt.Errorf("@%d %s", Location, err) }
	retErr := func(err error, Location uint32) error { return retErrStr(err.Error(), Location) }

	//Confirm the header and its data
	var header storeDictHeader
	if err := readBytes(any2b(&header)); err != nil {
		return retErr(err, prevBytesRead)
	}
	if b2s(header.fileType[0:3]) != "DTR" {
		return retErrStr("Invalid file header", 0)
	}
	if err := header.checkSoftCaps(); err != nil {
		return retErr(err, prevBytesRead)
	}

	//Create the final structure now that we have sizes
	*dict = languageDict{make(map[string]*namespace, header.numNamespaces), make([]string, header.numNamespaces), nil, false}

	//Make a temporary buffer of the largest size we need to read in all data
	tempBuff := make([]byte, max(
		size_storeTranslationIDsSize*header.numTranslations+header.idsSize,
		size_storeNamespace*header.numNamespaces+header.namespacesSize,
	))

	//Read in translation ids
	translationIDsList := make([]string, 0, header.numTranslations)
	{
		startStrPos := size_storeTranslationIDsSize * header.numTranslations
		if err, errOffset := readDataToStruct(
			header.numTranslations, "translation IDs", tempBuff, header.idsSize, readBytes, false, header.idsSize,
			func(pos uint32, readFrom *storeTranslationIDSize, accum uint32) {
				translationIDsList = append(
					translationIDsList,
					string(tempBuff[startStrPos+accum:startStrPos+accum+uint32(readFrom.length)]),
				)
			},
		); err != nil {
			return retErr(err, prevBytesRead+errOffset)
		}
	}

	//Read in namespaces
	{
		namePosStart := size_storeNamespace * header.numNamespaces
		namePosAccum := uint32(0)
		var nsLenErr error
		if err, errOffset := readDataToStruct(
			header.numNamespaces, "translation ID offsets", tempBuff, header.numTranslations, readBytes, false, header.namespacesSize,
			func(pos uint32, readFrom *storeNamespace, accum uint32) {
				//Get the namespace name
				if namePosAccum+uint32(readFrom.nameSize) > header.namespacesSize {
					if nsLenErr == nil {
						nsLenErr = retErrStr(fmt.Sprintf(
							"Length of accumulated [%s] data read (%d) at index (%d) has exceeded given data length (%d)",
							"namespace names", namePosAccum+uint32(readFrom.nameSize), pos, header.namespacesSize,
						), prevBytesRead+pos*uint32(unsafe.Sizeof(*readFrom))+3)
					}
					return
				}
				namespaceName := string(tempBuff[namePosStart+namePosAccum : namePosStart+namePosAccum+uint32(readFrom.nameSize)])
				namePosAccum += uint32(readFrom.nameSize)
				dict.namespacesInOrder[pos] = namespaceName

				//Create the namespace
				translationsLength := readFrom.getLength()
				myNamespace := namespace{
					namespaceName, uint(pos),
					make(translationIDs, translationsLength), nil,
				}
				dict.namespaces[namespaceName] = &myNamespace

				//Copy in the translation IDs
				for localIndex, translationID := range translationIDsList[accum : accum+translationsLength] {
					myNamespace.ids[translationID] = TransIndex(accum + uint32(localIndex))
				}
			},
		); err != nil {
			return retErr(err, prevBytesRead+errOffset)
		} else if nsLenErr != nil {
			return nsLenErr
		}
	}

	//Make sure we are at the end of the file
	if numBytesRead != uint32(header.getCompiledFileSize()) {
		return retErrStr(fmt.Sprintf("End of file not reached (%d!=%d)", numBytesRead, header.getCompiledFileSize()), numBytesRead)
	}

	//Save the dictionary hash
	dict.hash = hashOfFile.Sum(nil)

	//Return success
	return nil
}

func (dict *languageDict) fromCompiledVarFile(r io.Reader) error {
	//Handle reading the binary file
	var numBytesRead uint32 = 0
	readBytes := func(bytes []byte) error {
		numBytesRead += ulen32(bytes)

		if bytesRead, err := r.Read(bytes); err != nil {
			if err != io.EOF {
				return err
			} else if bytesRead != len(bytes) { //This is not actually an EOF error if bytesRead==len(bytes)
				return errors.New("File ended early")
			}
		} else if bytesRead < len(bytes) {
			return errors.New("File ended early")
		}

		return nil
	}
	readByte := func() (byte, error) {
		var r [1]byte
		if err := readBytes(r[:]); err != nil {
			return 0, err
		}
		return r[0], nil
	}

	//Compiled var files are companions to the compiled dictionary files, so they will have no headers beyond the 3 byte file definition
	{
		var h [3]byte
		if err := readBytes(h[:]); err != nil {
			return errors.New("Could not read header")
		} else if b2s(h[:]) != "VTR" {
			return errors.New("Header invalid")
		}
	}

	//Loop through namespace and translations and read variables
	startTransIndex := TransIndex(0)
	for _, namespaceName := range dict.namespacesInOrder {
		n := dict.namespaces[namespaceName]
		//Fill in translation IDs
		numTranslations := TransIndex(ulen32m(n.ids))
		n.idsInOrder = make([]translationIDNameAndVars, numTranslations)
		for name, index := range n.ids {
			n.idsInOrder[index-startTransIndex].name = name
		}
		startTransIndex += numTranslations

		//Read in variables
		for i := TransIndex(0); i < numTranslations; i++ {
			//Read in the number of variables
			v := &n.idsInOrder[i]
			if numVars, err := readByte(); err != nil {
				return err
			} else if numVars > 0 {
				v.vars = make([]translationIDVar, numVars)
			}

			//Create the variables
			for i2 := range v.vars {
				if nameLen, err := readByte(); err != nil {
					return err
				} else if varType, err := readByte(); err != nil {
					return err
				} else {
					newName := make([]byte, nameLen)
					if err := readBytes(newName); err != nil {
						return err
					}
					v.vars[i2] = translationIDVar{string(newName), variableType(varType)}
				}
			}
		}
	}

	//Return success
	dict.hasVarsLoaded = true
	return nil
}

func (l *Language) fromCompiledFile(r io.Reader, dict *languageDict) error {
	//Handle reading the binary file
	var numBytesRead, prevBytesRead uint32 = 0, 0
	readBytes := func(bytes []byte) error {
		prevBytesRead = numBytesRead
		numBytesRead += ulen32(bytes)

		if bytesRead, err := r.Read(bytes); err != nil {
			if err == io.EOF {
				//This is not actually an EOF error if bytesRead==len(bytes)
				if bytesRead == len(bytes) {
					return nil
				}

				return errors.New("File ended early")
			}
			return err
		} else if bytesRead < len(bytes) {
			return errors.New("File ended early")
		}

		return nil
	}

	//Handle returning errors
	retErrStr := func(err string, Location uint32) error { return fmt.Errorf("@%d %s", Location, err) }
	retErr := func(err error, Location uint32) error { return retErrStr(err.Error(), Location) }

	//Confirm the header and its data
	var header storeHeader
	if err := readBytes(any2b(&header)); err != nil {
		return retErr(err, prevBytesRead)
	} else if b2s(header.fileType[0:3]) != "GTR" {
		return retErrStr("Invalid file header", prevBytesRead)
	} else if header.translationStringByteLength != uint8(size_storeTranslationRule16) && header.translationStringByteLength != uint8(size_storeTranslationRule32) {
		return retErrStr(fmt.Sprintf("Invalid translation string size (%d != (%d || %d))", header.translationStringByteLength, size_storeTranslationRule16, size_storeTranslationRule32), prevBytesRead+uint32(unsafe.Offsetof(header.translationStringByteLength)))
	} else if err := header.checkSoftCaps(); err != nil {
		return retErr(err, prevBytesRead)
	} else if !bytes.Equal(header.hash[:], dict.hash) {
		return retErrStr(ErrDictionaryDoesNotMatch, prevBytesRead+uint32(unsafe.Offsetof(header.hash)))
	}

	//Make sure the number of translations matches the dictionary
	{
		expectedNumTranslations := uint32(0)
		for _, n := range dict.namespaces {
			expectedNumTranslations += ulen32m(n.ids)
		}
		if expectedNumTranslations != header.numTranslations {
			return retErrStr(fmt.Sprintf("Number of translations (%d) does not match number in dictionary (%d)", header.numTranslations, expectedNumTranslations), prevBytesRead)
		}
	}

	//Pull in the settings
	var languageTag language.Tag
	var settingsValues []string
	{
		//Read in the settings section from the file
		settingsStr := make([]byte, header.settingsSize)
		if err := readBytes(settingsStr); err != nil {
			return retErr(err, prevBytesRead)
		}

		//Process the settings
		const numSettings = 4
		const settingLenSize = uint(unsafe.Sizeof(uint16(0)))
		settingsValues = make([]string, numSettings)
		for i, byteLoc := uint(0), uint(0); i < numSettings; i++ {
			//Get the settings string value
			if byteLoc+settingLenSize > ulen(settingsStr) {
				return retErrStr("invalid settings length", prevBytesRead+uint32(byteLoc))
			}
			strLen := uint(*(*uint16)(unsafe.Pointer(&settingsStr[byteLoc])))
			byteLoc += settingLenSize
			if byteLoc+strLen > ulen(settingsStr) {
				return retErrStr("invalid string length", prevBytesRead+uint32(byteLoc))
			}
			settingsValues[i] = string(settingsStr[byteLoc : byteLoc+strLen])
			byteLoc += strLen

			switch i {
			//Handle a language tag
			case 1:
				if _languageTag, err := language.Parse(settingsValues[i]); err != nil {
					return retErrStr("Invalid language tag: "+settingsValues[i], prevBytesRead+uint32(byteLoc-strLen))
				} else {
					languageTag = _languageTag
				}
			//Make sure byteLoc matches len(settingsStr) on last iteration
			case numSettings - 1:
				if byteLoc != ulen(settingsStr) {
					return retErrStr(fmt.Sprintf("Settings length not completely consumed (%d!=%d)", byteLoc, len(settingsStr)), prevBytesRead+uint32(byteLoc))
				}
			}
		}
	}

	//Create the final structure now that we have sizes
	*l = Language{
		stringsData:        make([]byte, header.dataSize),
		rules:              make([]translationRule, header.numRules+1),
		translations:       make([]translationRuleSlice, header.numTranslations+1),
		dict:               dict,
		name:               settingsValues[0],
		languageIdentifier: settingsValues[1],
		fallbackName:       settingsValues[2],
		missingPluralRule:  settingsValues[3],
		languageTag:        languageTag,
	}

	//Make a temporary buffer of the largest size we need to read in all data
	tempBuff := make([]byte, max(
		uint32(header.translationStringByteLength)*header.numRules,
		size_storeTranslationRuleSlice*header.numTranslations,
	))

	//Read in translation rules
	{
		var err error
		var errOffset uint32
		if header.translationStringByteLength == uint8(size_storeTranslationRule16) {
			err, errOffset = readDataToStruct(
				header.numRules, "rules", tempBuff, header.dataSize, readBytes, true, 0,
				func(pos uint32, readFrom *storeTranslationRule16, accum uint32) {
					if readFrom == nil {
						l.rules[pos] = translationRule{accum, pluralRule{cmpAll, 0}}
					} else {
						l.rules[pos] = translationRule{accum, readFrom.rule}
					}
				},
			)
		} else {
			err, errOffset = readDataToStruct(
				header.numRules, "rules", tempBuff, header.dataSize, readBytes, true, 0,
				func(pos uint32, readFrom *storeTranslationRule32, accum uint32) {
					if readFrom == nil {
						l.rules[pos] = translationRule{accum, pluralRule{cmpAll, 0}}
					} else {
						l.rules[pos] = translationRule{accum, readFrom.rule}
					}
				},
			)
		}
		if err != nil {
			return retErr(err, prevBytesRead+errOffset)
		}
	}

	//Read in translation rule slices
	if err, errOffset := readDataToStruct(
		header.numTranslations, "rule slices", tempBuff, header.numRules, readBytes, true, 0,
		func(pos uint32, readFrom *storeTranslationRuleSlice, accum uint32) {
			l.translations[pos] = translationRuleSlice{accum}
		},
	); err != nil {
		return retErr(err, prevBytesRead+errOffset)
	}

	//Pull in stringsData
	if err := readBytes(l.stringsData); err != nil {
		return retErr(err, prevBytesRead)
	}

	//Make sure we are at the end of the file
	if numBytesRead != uint32(header.getCompiledFileSize()) {
		return retErrStr(fmt.Sprintf("End of file not reached (%d!=%d)", numBytesRead, header.getCompiledFileSize()), numBytesRead)
	}

	//Return success
	return nil
}

// Reads binary data into a slice and checks its data against known buffer lengths
func readDataToStruct[
	readType storeTranslationRule16 | storeTranslationRule32 | storeTranslationRuleSlice | storeTranslationIDSize | storeNamespace,
](
	numToReadIntoSlice uint32, readTypeName string, //Info for writing to slice
	tempBuff []byte, expectedReadLen uint32, readBytes func([]byte) error, //Info for reading from buffer
	hasExtraValAtEnd bool, extraDataToReadSize uint32, //Extra control variables
	storeStruct func(pos uint32, readFrom *readType, accum uint32), //Callback to store the read data
) (Error error, ErrorLocationOffset uint32) {
	//Read the bytes into the buffer
	var tempReadStruct readType
	if err := readBytes(tempBuff[0 : uint32(unsafe.Sizeof(tempReadStruct))*numToReadIntoSlice+extraDataToReadSize]); err != nil {
		return fmt.Errorf("Read error [%s] %s", readTypeName, err), 0
	}

	//Process the buffer, converted into a typed slice
	var accum uint32 = 0
	//goland:noinspection GoRedundantConversion
	for i, v := range unsafe.Slice((*readType)(unsafe.Pointer(&tempBuff[0])), numToReadIntoSlice) {
		//Confirm end position is within range
		EndPos := accum + any(v).(getLength).getLength()
		if EndPos > expectedReadLen {
			return fmt.Errorf(
					"Length of accumulated [%s] data read (%d) at index (%d) has exceeded given data length (%d)",
					readTypeName, EndPos, i, expectedReadLen,
				),
				uint32(i) * uint32(unsafe.Sizeof(tempReadStruct))
		}

		//Store the struct
		storeStruct(uint32(i), &v, accum)
		accum = EndPos
	}

	//Make sure the expectedReadLen was properly reached
	if accum != expectedReadLen {
		return fmt.Errorf(
			"Length of accumulated [%s] data read (%d) did not reach the end (%d)",
			readTypeName, accum, expectedReadLen,
		), numToReadIntoSlice * uint32(unsafe.Sizeof(tempReadStruct))
	}

	//If there is an extra value at the end, store it
	if hasExtraValAtEnd {
		storeStruct(numToReadIntoSlice, nil, accum)
	}

	return nil, 0
}
