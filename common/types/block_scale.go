// Code generated by github.com/spacemeshos/go-scale/scalegen. DO NOT EDIT.

// nolint
package types

import (
	"github.com/spacemeshos/go-scale"
)

func (t *Block) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := t.InnerBlock.EncodeScale(enc)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *Block) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := t.InnerBlock.DecodeScale(dec)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *BlockHeader) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.ID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.Layer))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact64(enc, uint64(t.Height))
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *BlockHeader) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.ID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Layer = LayerID(field)
	}
	{
		field, n, err := scale.DecodeCompact64(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Height = uint64(field)
	}
	return total, nil
}

func (t *InnerBlock) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.LayerIndex))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact64(enc, uint64(t.TickHeight))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.Rewards, 500)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.TxIDs, 100000)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *InnerBlock) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.LayerIndex = LayerID(field)
	}
	{
		field, n, err := scale.DecodeCompact64(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.TickHeight = uint64(field)
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[AnyReward](dec, 500)
		if err != nil {
			return total, err
		}
		total += n
		t.Rewards = field
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[TransactionID](dec, 100000)
		if err != nil {
			return total, err
		}
		total += n
		t.TxIDs = field
	}
	return total, nil
}

func (t *RatNum) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeCompact64(enc, uint64(t.Num))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact64(enc, uint64(t.Denom))
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *RatNum) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeCompact64(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Num = uint64(field)
	}
	{
		field, n, err := scale.DecodeCompact64(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Denom = uint64(field)
	}
	return total, nil
}

func (t *AnyReward) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.AtxID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := t.Weight.EncodeScale(enc)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *AnyReward) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.AtxID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := t.Weight.DecodeScale(dec)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *Certificate) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteArray(enc, t.BlockID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeStructSliceWithLimit(enc, t.Signatures, 1000)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *Certificate) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := scale.DecodeByteArray(dec, t.BlockID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeStructSliceWithLimit[CertifyMessage](dec, 1000)
		if err != nil {
			return total, err
		}
		total += n
		t.Signatures = field
	}
	return total, nil
}

func (t *CertifyMessage) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := t.CertifyContent.EncodeScale(enc)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.Signature[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.SmesherID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *CertifyMessage) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := t.CertifyContent.DecodeScale(dec)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.DecodeByteArray(dec, t.Signature[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.DecodeByteArray(dec, t.SmesherID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *CertifyContent) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.LayerID))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.BlockID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact16(enc, uint16(t.EligibilityCnt))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.Proof[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *CertifyContent) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.LayerID = LayerID(field)
	}
	{
		n, err := scale.DecodeByteArray(dec, t.BlockID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeCompact16(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.EligibilityCnt = uint16(field)
	}
	{
		n, err := scale.DecodeByteArray(dec, t.Proof[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
