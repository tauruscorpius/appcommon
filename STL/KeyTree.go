package STL

// TeBCD KEY-TREE golang implementation
// https://en.wikipedia.org/wiki/Binary-coded_decimal

import (
	"errors"
	"fmt"
)

const (
	MaxKeyDepth int = 15
	KeyCounter  int = 16 // 0-9, a-f||A-F
)

type (
	KeyTree struct {
		next      [KeyCounter]*KeyTree
		data      interface{}
		minDigits int
	}
)

func (t *KeyTree) _getSlotKey(k byte) int8 {
	if k >= '0' && k <= '9' {
		return int8(k - '0')
	}
	if k >= 'a' && k <= 'f' {
		return int8(k - 'a' + 10)
	}
	if k >= 'A' && k <= 'F' {
		return int8(k - 'A' + 10)
	}
	return -1
}

func (t *KeyTree) AddNode(min int, seg []byte, data interface{}) error {
	if len(seg) > MaxKeyDepth {
		return errors.New("invalid key depth by " + string(seg))
	}
	idx := 0
	iter := t
	for idx < len(seg) {
		slot := t._getSlotKey(seg[idx])
		if slot < 0 || slot >= int8(KeyCounter) {
			return errors.New("invalid digit index for digits key tree, @" + string(seg[idx:]))
		}
		if iter.next[slot] == nil {
			iter.next[slot] = &KeyTree{}
		}
		iter = iter.next[slot]
		idx++
	}
	iter.data = data
	iter.minDigits = min
	return nil
}

func (t *KeyTree) GetNode(seg []byte) (error, interface{}) {
	idx := 0
	iter := t
	data := t
	match := 0
	for idx < len(seg) && iter != nil {
		slot := t._getSlotKey(seg[idx])
		if slot < 0 || slot >= int8(KeyCounter) {
			return errors.New("invalid digit index for digits key tree, @" + string(seg[idx:])), nil
		}
		if iter.next[slot] == nil {
			break
		}
		iter = iter.next[slot]
		if iter.data != nil {
			// max length matched effective data
			data = iter
			match = idx + 1
		}
		idx++
	}
	if data == nil || data == t {
		return errors.New("not found by " + string(seg)), nil
	}
	if data.minDigits > 0 && len(seg) < data.minDigits {
		// min digits limited
		return fmt.Errorf("min digits limited, by %s expect %d current %d\n", string(seg), data.minDigits, match), nil
	}
	return nil, data.data
}
