// Code generated by github.com/spacemeshos/go-scale/scalegen. DO NOT EDIT.

// nolint
package types

import (
	"github.com/spacemeshos/go-scale"
)

func (t *NIPostChallenge) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.PublishEpoch))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact64(enc, uint64(t.Sequence))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.PrevATXID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.PositioningATX[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.CommitmentATX)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteSliceWithLimit(enc, t.InitialPostIndices, 8000)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *NIPostChallenge) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.PublishEpoch = EpochID(field)
	}
	{
		field, n, err := scale.DecodeCompact64(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Sequence = uint64(field)
	}
	{
		n, err := scale.DecodeByteArray(dec, t.PrevATXID[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.DecodeByteArray(dec, t.PositioningATX[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		field, n, err := scale.DecodeOption[ATXID](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.CommitmentATX = field
	}
	{
		field, n, err := scale.DecodeByteSliceWithLimit(dec, 8000)
		if err != nil {
			return total, err
		}
		total += n
		t.InitialPostIndices = field
	}
	return total, nil
}

func (t *InnerActivationTx) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := t.NIPostChallenge.EncodeScale(enc)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.Coinbase[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.NumUnits))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.NIPost)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.InitialPost)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.NodeID)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.VRFNonce)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *InnerActivationTx) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := t.NIPostChallenge.DecodeScale(dec)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.DecodeByteArray(dec, t.Coinbase[:])
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
		t.NumUnits = uint32(field)
	}
	{
		field, n, err := scale.DecodeOption[NIPost](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.NIPost = field
	}
	{
		field, n, err := scale.DecodeOption[Post](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.InitialPost = field
	}
	{
		field, n, err := scale.DecodeOption[NodeID](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.NodeID = field
	}
	{
		field, n, err := scale.DecodeOption[VRFPostIndex](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.VRFNonce = field
	}
	return total, nil
}

func (t *ATXMetadata) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeCompact32(enc, uint32(t.PublishEpoch))
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeByteArray(enc, t.MsgHash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *ATXMetadata) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeCompact32(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.PublishEpoch = EpochID(field)
	}
	{
		n, err := scale.DecodeByteArray(dec, t.MsgHash[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *ActivationTx) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := t.InnerActivationTx.EncodeScale(enc)
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
	{
		n, err := scale.EncodeByteArray(enc, t.Signature[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *ActivationTx) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		n, err := t.InnerActivationTx.DecodeScale(dec)
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
	{
		n, err := scale.DecodeByteArray(dec, t.Signature[:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *NIPost) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeOption(enc, t.Challenge)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.Post)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeOption(enc, t.PostMetadata)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *NIPost) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeOption[Hash32](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Challenge = field
	}
	{
		field, n, err := scale.DecodeOption[Post](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.Post = field
	}
	{
		field, n, err := scale.DecodeOption[PostMetadata](dec)
		if err != nil {
			return total, err
		}
		total += n
		t.PostMetadata = field
	}
	return total, nil
}

func (t *PostMetadata) EncodeScale(enc *scale.Encoder) (total int, err error) {
	{
		n, err := scale.EncodeByteSliceWithLimit(enc, t.Challenge, 32)
		if err != nil {
			return total, err
		}
		total += n
	}
	{
		n, err := scale.EncodeCompact64(enc, uint64(t.LabelsPerUnit))
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (t *PostMetadata) DecodeScale(dec *scale.Decoder) (total int, err error) {
	{
		field, n, err := scale.DecodeByteSliceWithLimit(dec, 32)
		if err != nil {
			return total, err
		}
		total += n
		t.Challenge = field
	}
	{
		field, n, err := scale.DecodeCompact64(dec)
		if err != nil {
			return total, err
		}
		total += n
		t.LabelsPerUnit = uint64(field)
	}
	return total, nil
}
