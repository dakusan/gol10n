//Public functions to save files from io.Writer
//go:build !gol10n_read_compiled_only

package translate

import (
	"compress/gzip"
	"io"
)

// SaveGTR saves a .gtr language file
func (l *Language) SaveGTR(w io.Writer, isCompressed bool) error {
	if isCompressed {
		_w := gzip.NewWriter(w)
		defer func() { _ = _w.Close() }()
		w = _w
	}
	return l.toCompiledFile(w)
}

// SaveGTRDict saves a .gtr dictionary file
func (l *Language) SaveGTRDict(w io.Writer, isCompressed bool) error {
	if isCompressed {
		_w := gzip.NewWriter(w)
		defer func() { _ = _w.Close() }()
		w = _w
	}
	return l.dict.toCompiledFile(w)
}

// SaveGTRVarsDict saves a .gtr variable dictionary file
func (l *Language) SaveGTRVarsDict(w io.Writer, isCompressed bool) error {
	if isCompressed {
		_w := gzip.NewWriter(w)
		defer func() { _ = _w.Close() }()
		w = _w
	}
	return l.dict.toCompiledVarFile(w)
}

// SaveGoDictionaries saves the *.go files from the language to $outputDirectory/$namespaceName/TranslationIDs.go.
// The GoDictHeader is inserted just before the `const` declaration
func (l *Language) SaveGoDictionaries(outputDirectory, GoDictHeader string) (err error, numUpdated uint) {
	return l.toGoDictionaries(outputDirectory, GoDictHeader)
}
