//Handle plurality rules

package translate

type cmpOp uint8
type pluralRule struct {
	op cmpOp //For cmpBetween the top 5 bits (0-31) are added to i0 for the upper limit of the between comparison. cmpBetweenExtraBit gives (32-63)
	i0 uint8
}

func (pr pluralRule) getOp() cmpOp {
	return cmpOp(uint8(pr.op) & 7)
}

const (
	cmpAll cmpOp = iota
	cmpEquals
	cmpLess
	cmpLessEqual
	cmpGreater
	cmpGreaterEqual
	cmpBetween
	cmpBetweenExtraBit
	_ = 255
)

func (pr pluralRule) cmp(pluralCount uint8) bool {
	switch pr.getOp() {
	case cmpAll:
		return true
	case cmpEquals:
		return pluralCount == pr.i0
	case cmpLess:
		return pluralCount < pr.i0
	case cmpLessEqual:
		return pluralCount <= pr.i0
	case cmpGreater:
		return pluralCount > pr.i0
	case cmpGreaterEqual:
		return pluralCount >= pr.i0
	case cmpBetween:
		return pr.i0 <= pluralCount && (pr.i0+(uint8(pr.op)>>3)) >= pluralCount
	case cmpBetweenExtraBit:
		return pr.i0 <= pluralCount && (pr.i0+(uint8(pr.op)>>3))+32 >= pluralCount
	default:
		return false
	}
}
