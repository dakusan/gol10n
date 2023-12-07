//Convert from JSON files
//go:build !gol10n_read_compiled_only

package translate

import (
	"errors"
	"github.com/valyala/fastjson"
	"regexp"
	"unicode/utf8"
)

type jsonMapSlice struct {
	obj *fastjson.Object
}
type jsonItem struct {
	name  string
	value *fastjson.Value
}

func (ms jsonMapSlice) getValue(paramName string) (val tpItem, ok bool) {
	var ret tpItem = nil
	ms.obj.Visit(func(key []byte, v *fastjson.Value) {
		if b2s(key) == paramName {
			ret = jsonItem{paramName, v}
		}
	})
	return ret, ret != nil
}

func (ms jsonMapSlice) toMap() map[string]tpItem {
	retVal := make(map[string]tpItem, ms.obj.Len())
	ms.obj.Visit(func(key []byte, v *fastjson.Value) {
		str := b2s(key)
		retVal[str] = jsonItem{str, v}
	})

	return retVal
}

func (ms jsonMapSlice) toOrdered() []tpItem {
	ret := make([]tpItem, ms.obj.Len())
	i := 0
	ms.obj.Visit(func(key []byte, v *fastjson.Value) {
		ret[i] = jsonItem{b2s(key), v}
		i++
	})

	return ret
}

func (ms jsonMapSlice) getLength() uint {
	return uint(ms.obj.Len())
}

func (i jsonItem) getName() string {
	return i.name
}

func (i jsonItem) getObject() (val tpMap, ok bool) {
	if i.value.Type() != fastjson.TypeObject {
		return nil, false
	} else if getObj, err := i.value.Object(); err != nil {
		return nil, false
	} else {
		return jsonMapSlice{getObj}, true
	}
}

func (i jsonItem) getString() (val string, ok bool) {
	switch i.value.Type() {
	case fastjson.TypeString:
		if getStr, err := i.value.StringBytes(); err != nil {
			return returnBlankStrOnErr, false
		} else {
			return b2s(getStr), true
		}
	case fastjson.TypeNumber, fastjson.TypeTrue, fastjson.TypeFalse, fastjson.TypeNull:
		return i.value.String(), true
	default:
		return returnBlankStrOnErr, false
	}
}

func fromJsonFile(textStr []byte, allowJSONTrailingComma bool) (jsonItem, error) {
	//Check for valid utf8
	if !utf8.Valid(textStr) {
		return jsonItem{}, errors.New("File is not utf8 valid")
	}

	//Remove trailing commas if requested
	if allowJSONTrailingComma {
		textStr = regexp.MustCompile(`,\s*?\n\s*}`).ReplaceAll(textStr, []byte{'}'})
	}

	if ret, err := (&fastjson.Parser{}).ParseBytes(textStr); err != nil {
		return jsonItem{}, errors.New("Error parsing JSON File: " + err.Error())
	} else {
		return jsonItem{"TOP", ret}, nil
	}
}
