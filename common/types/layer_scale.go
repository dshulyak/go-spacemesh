// Code generated by github.com/spacemeshos/go-scale/scalegen. DO NOT EDIT.

package types

import (
	"github.com/spacemeshos/go-scale"
)

func (t *LayerID) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeCompact32(enc, uint32(t.Value)); err != nil {
		return total, err
	} else { // nolint
		total += n
	}
	return total, nil
}

func (t *LayerID) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if field, n, err := scale.DecodeCompact32(dec); err != nil {
		return total, err
	} else { // nolint
		total += n
		t.Value = uint32(field)
	}
	return total, nil
}
