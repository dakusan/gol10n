//Convert from YAML files
//go:build !gol10n_read_compiled_only

package translate

import (
	"errors"
	"gopkg.in/yaml.v2"
	"strconv"
	"unicode/utf8"
)

type yamlMapSlice yaml.MapSlice
type yamlItem yaml.MapItem

func (ms *yamlMapSlice) getValue(paramName string) (val tpItem, ok bool) {
	for _, v := range *ms {
		if v.Key == paramName {
			return yamlItem(v), true
		}
	}
	return nil, false
}

func (ms *yamlMapSlice) toMap() map[string]tpItem {
	retVal := make(map[string]tpItem, len(*ms))
	for _, v := range *ms {
		name, _ := yamlValToStr(v.Key)
		retVal[name] = yamlItem{Key: name, Value: v.Value}
	}
	return retVal
}

func (ms *yamlMapSlice) toOrdered() []tpItem {
	ret := make([]tpItem, len(*ms))
	for i, v := range *ms {
		ret[i] = yamlItem(v)
	}

	return ret
}

func (ms *yamlMapSlice) getLength() uint {
	return ulen(*ms)
}

func (i yamlItem) getName() string {
	return twoToOne(yamlValToStr(i.Key))
}

func (i yamlItem) getObject() (val tpMap, ok bool) {
	if _val, ok := i.Value.(yamlMapSlice); !ok {
		return nil, false
	} else {
		return &_val, true
	}
}

func (i yamlItem) getString() (val string, ok bool) {
	val, ok = yamlValToStr(i.Value)
	return
}

func yamlValToStr(i interface{}) (val string, ok bool) {
	switch v := i.(type) {
	case string:
		//Note: YAML.v2 returns timestamps as strings
		return v, true
	case int:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	case float64:
		return strconv.FormatFloat(v, 'g', 15, 64), true
	case bool:
		if v {
			return "Yes", true
		}
		return "No", true
	case nil:
		return "", true
	default:
		return returnBlankStrOnErr, false
	}
}

func fromYamlFile(textStr []byte) (yamlItem, error) {
	//Check for valid utf8
	if !utf8.Valid(textStr) {
		return yamlItem{}, errors.New("File is not utf8 valid")
	}

	ms := yamlMapSlice{}
	if err := yaml.Unmarshal(textStr, &ms); err != nil {
		return yamlItem{}, errors.New("Error parsing YAML File: " + err.Error())
	}

	return yamlItem{Key: "TOP", Value: ms}, nil
}
