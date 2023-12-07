//Create plurality rules
//go:build !gol10n_read_compiled_only

package translate

import (
	"errors"
	"math"
)

func createPluralRule(s string) (pluralRule, error) {
	//Consume whitespace
	index := 0
	strLen := len(s)
	consumeWhitespace := func() {
		for index < strLen && s[index] == ' ' {
			index++
		}
	}

	//Get the primary operator
	var myOp cmpOp
	switch s[index] {
	case '^':
		index++
		consumeWhitespace()
		if index != strLen {
			return pluralRule{}, errors.New("^ operator cannot have anything after it")
		}
		return pluralRule{cmpAll, 0}, nil
	case '=':
		myOp = cmpEquals
	case '<':
		myOp = cmpLess
	case '>':
		myOp = cmpGreater
	case '~':
		myOp = cmpBetween
	default:
		return pluralRule{}, errors.New("Must start with one of: ^ = < > ~")
	}
	index++

	//Extend cmpLess and cmpGreater to cmpLessEqual and cmpGreaterEqual
	consumeWhitespace()
	if (myOp == cmpLess || myOp == cmpGreater) && index < strLen && s[index] == '=' {
		if myOp == cmpLess {
			myOp = cmpLessEqual
		} else {
			myOp = cmpGreaterEqual
		}
		index++
		consumeWhitespace()
	}

	//Make sure the operator is followed by an appropriate number
	const maxUint8Digits, base10Shift = 3, 10
	getNum := func() (uint16, uint) {
		numFound := uint16(0)
		digitNumsConsumed := uint(0)
		for index < strLen && s[index] >= '0' && s[index] <= '9' && digitNumsConsumed < maxUint8Digits {
			numFound = numFound*base10Shift + uint16(s[index]-'0')
			digitNumsConsumed++
			index++
		}
		return numFound, digitNumsConsumed
	}
	_numFound, _digitNumsConsumed := getNum()
	if _digitNumsConsumed == 0 || _numFound > math.MaxUint8 {
		return pluralRule{}, errors.New("Operator must be followed by a number between 0 and 255")
	}

	//Handle unary operators
	consumeWhitespace()
	if myOp != cmpBetween {
		if index < strLen {
			return pluralRule{}, errors.New("Nothing can follow the number")
		}
		return pluralRule{myOp, uint8(_numFound)}, nil
	}

	//Handle dual operators (cmpBetween)
	if index >= strLen || s[index] != '-' {
		return pluralRule{}, errors.New("~ operator must have a dash following the first number")
	}
	index++
	consumeWhitespace()
	_numFound2, _digitNumsConsumed2 := getNum()
	const maxBetweenDiff, halfBetweenDiff = 64 - 1, 64 / 2
	numFoundDiff := int(_numFound2) - int(_numFound)
	if _digitNumsConsumed2 == 0 || numFoundDiff < 0 || numFoundDiff > maxBetweenDiff {
		return pluralRule{}, errors.New("The second number of the ~ operator must be followed by a number between 0-63 plus the first number")
	}

	//Finish dual operators
	consumeWhitespace()
	if index < strLen {
		return pluralRule{}, errors.New("Nothing can follow the second number")
	}
	isAboveHalf := uint8(0)
	if numFoundDiff >= halfBetweenDiff {
		isAboveHalf = 1
	}
	return pluralRule{cmpOp(uint8(myOp) + isAboveHalf + (uint8(numFoundDiff)-halfBetweenDiff*isAboveHalf)<<3), uint8(_numFound)}, nil
}
