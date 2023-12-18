//Create a table of the flags of a list of ProcessFiles
//go:build !gol10n_read_compiled_only

package execute

import (
	"bytes"
)

// CreateFlagTable creates an aligned ascii table that shows which flags are set on which ProcessedFiles. The row headers are the ProcessedFileFlagNames.ShortName and the column headers are the ProcessedFile.LangIdentifier.
//
// Format: 1 row per language. 1 column per flag. 4 letter flag names are split over 2 columns
//
// Symbols: | column separator, * values
func (list ProcessedFileList) CreateFlagTable() []string {
	//Get which ProcessFileFlags were used and the maximum length of the language identifiers
	usedPFFlags := make([]bool, len(ProcessedFileFlagNames))
	maxLangLen := 2 //All columns must be at least 2 bytes
	for _, pf := range list {
		if len(pf.LangIdentifier) > maxLangLen {
			maxLangLen = len(pf.LangIdentifier)
		}
		for flagIndex, flagInfo := range ProcessedFileFlagNames {
			if pf.Flags&flagInfo.Flag != 0 {
				usedPFFlags[flagIndex] = true
			}
		}
	}

	//Get the list of flags to use
	flagsList := make([]int, 0, len(usedPFFlags))
	for flagIndex, wasUsed := range usedPFFlags {
		if wasUsed {
			flagsList = append(flagsList, flagIndex)
		}
	}

	//Pull the used flag values and create a byte array of the row format
	const charColSeparator, charSpacer, charFlagSet = '|', ' ', '*'
	const colWidth, colSepWidth = 2, 1 //All column widths are 2 characters
	flagValues := make([]struct {
		flag ProcessedFileFlag
		pos  int
	}, len(flagsList))
	rowBytes := bytes.Repeat([]byte{charSpacer}, colSepWidth*2+maxLangLen+len(flagsList)*(colWidth+colSepWidth))
	rowBytes[0] = charColSeparator
	rowBytes[maxLangLen+colSepWidth] = charColSeparator
	for localIndex, lookupIndex := range flagsList {
		fv := &flagValues[localIndex]
		fv.flag = ProcessedFileFlagNames[lookupIndex].Flag
		fv.pos = maxLangLen + colSepWidth + colSepWidth + localIndex*(colWidth+colSepWidth)
		rowBytes[fv.pos+colWidth] = charColSeparator
	}

	//Create the 2 header rows
	outRows := make([]string, len(list)+2)
	rowNum := 0
	for i := 0; i < 2; i++ {
		for localIndex, lookupIndex := range flagsList {
			fv := flagValues[localIndex]
			name := ProcessedFileFlagNames[lookupIndex].ShortName
			copy(rowBytes[fv.pos:fv.pos+colWidth], name[i*colWidth:(i+1)*colWidth])
		}
		outRows[rowNum] = string(rowBytes)
		rowNum++
	}
	for localIndex := range flagsList {
		rowBytes[flagValues[localIndex].pos+1] = charSpacer
	}

	//Output the rows
	langIdentSpacer := bytes.Repeat([]byte{charSpacer}, maxLangLen)
	for _, v := range list {
		copy(rowBytes[colSepWidth:], langIdentSpacer)
		copy(rowBytes[colSepWidth:], v.LangIdentifier)
		for _, f := range flagValues {
			rowBytes[f.pos] = cond[byte](v.Flags&f.flag == 0, charSpacer, charFlagSet)
		}
		outRows[rowNum] = string(rowBytes)
		rowNum++
	}

	return outRows
}
