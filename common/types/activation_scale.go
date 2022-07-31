// Code generated by github.com/spacemeshos/go-scale/scalegen. DO NOT EDIT.

package types

import (
	"github.com/spacemeshos/go-scale"
)

func (t *ActivationTxHeader) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := t.NIPostChallenge.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeByteArray(enc, t.Coinbase[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact(enc, uint(t.NumUnits)); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *ActivationTxHeader) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if n, err := t.NIPostChallenge.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.DecodeByteArray(dec, t.Coinbase[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if field, n, err := scale.DecodeCompact(dec); err != nil {
		return total, err
	} else {
		total += n
		t.NumUnits = uint(field)
	}
	return total, nil
}

func (t *NIPostChallenge) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeByteArray(enc, t.NodeID[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact64(enc, uint64(t.Sequence)); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeByteArray(enc, t.PrevATXID[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := t.PubLayerID.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact64(enc, uint64(t.StartTick)); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact64(enc, uint64(t.EndTick)); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeByteArray(enc, t.PositioningATX[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeByteSlice(enc, t.InitialPostIndices); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *NIPostChallenge) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if n, err := scale.DecodeByteArray(dec, t.NodeID[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if field, n, err := scale.DecodeCompact64(dec); err != nil {
		return total, err
	} else {
		total += n
		t.Sequence = uint64(field)
	}
	if n, err := scale.DecodeByteArray(dec, t.PrevATXID[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := t.PubLayerID.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}
	if field, n, err := scale.DecodeCompact64(dec); err != nil {
		return total, err
	} else {
		total += n
		t.StartTick = uint64(field)
	}
	if field, n, err := scale.DecodeCompact64(dec); err != nil {
		return total, err
	} else {
		total += n
		t.EndTick = uint64(field)
	}
	if n, err := scale.DecodeByteArray(dec, t.PositioningATX[:]); err != nil {
		return total, err
	} else {
		total += n
	}
	if field, n, err := scale.DecodeByteSlice(dec); err != nil {
		return total, err
	} else {
		total += n
		t.InitialPostIndices = field
	}
	return total, nil
}

func (t *InnerActivationTx) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeOption(enc, t.ActivationTxHeader); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeOption(enc, t.NIPost); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeOption(enc, t.InitialPost); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *InnerActivationTx) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if field, n, err := scale.DecodeOption[ActivationTxHeader](dec); err != nil {
		return total, err
	} else {
		total += n
		t.ActivationTxHeader = field
	}
	if field, n, err := scale.DecodeOption[NIPost](dec); err != nil {
		return total, err
	} else {
		total += n
		t.NIPost = field
	}
	if field, n, err := scale.DecodeOption[Post](dec); err != nil {
		return total, err
	} else {
		total += n
		t.InitialPost = field
	}
	return total, nil
}

func (t *ActivationTx) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeOption(enc, t.InnerActivationTx); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeByteSlice(enc, t.Sig); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *ActivationTx) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if field, n, err := scale.DecodeOption[InnerActivationTx](dec); err != nil {
		return total, err
	} else {
		total += n
		t.InnerActivationTx = field
	}
	if field, n, err := scale.DecodeByteSlice(dec); err != nil {
		return total, err
	} else {
		total += n
		t.Sig = field
	}
	return total, nil
}

func (t *PoetProof) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := t.MerkleProof.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeSliceOfByteSlice(enc, t.Members); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact64(enc, uint64(t.LeafCount)); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *PoetProof) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if n, err := t.MerkleProof.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}
	if field, n, err := scale.DecodeSliceOfByteSlice(dec); err != nil {
		return total, err
	} else {
		total += n
		t.Members = field
	}
	if field, n, err := scale.DecodeCompact64(dec); err != nil {
		return total, err
	} else {
		total += n
		t.LeafCount = uint64(field)
	}
	return total, nil
}

func (t *PoetProofMessage) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := t.PoetProof.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeByteSlice(enc, t.PoetServiceID); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeString(enc, t.RoundID); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeByteSlice(enc, t.Signature); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *PoetProofMessage) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if n, err := t.PoetProof.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}
	if field, n, err := scale.DecodeByteSlice(dec); err != nil {
		return total, err
	} else {
		total += n
		t.PoetServiceID = field
	}
	if field, n, err := scale.DecodeString(dec); err != nil {
		return total, err
	} else {
		total += n
		t.RoundID = field
	}
	if field, n, err := scale.DecodeByteSlice(dec); err != nil {
		return total, err
	} else {
		total += n
		t.Signature = field
	}
	return total, nil
}

func (t *PoetRound) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeString(enc, t.ID); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *PoetRound) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if field, n, err := scale.DecodeString(dec); err != nil {
		return total, err
	} else {
		total += n
		t.ID = field
	}
	return total, nil
}

func (t *NIPost) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeOption(enc, t.Challenge); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeOption(enc, t.Post); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeOption(enc, t.PostMetadata); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *NIPost) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if field, n, err := scale.DecodeOption[Hash32](dec); err != nil {
		return total, err
	} else {
		total += n
		t.Challenge = field
	}
	if field, n, err := scale.DecodeOption[Post](dec); err != nil {
		return total, err
	} else {
		total += n
		t.Post = field
	}
	if field, n, err := scale.DecodeOption[PostMetadata](dec); err != nil {
		return total, err
	} else {
		total += n
		t.PostMetadata = field
	}
	return total, nil
}

func (t *PostMetadata) EncodeScale(enc *scale.Encoder) (total int, err error) {
	if n, err := scale.EncodeByteSlice(enc, t.Challenge); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact(enc, uint(t.BitsPerLabel)); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact(enc, uint(t.LabelsPerUnit)); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact(enc, uint(t.K1)); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := scale.EncodeCompact(enc, uint(t.K2)); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (t *PostMetadata) DecodeScale(dec *scale.Decoder) (total int, err error) {
	if field, n, err := scale.DecodeByteSlice(dec); err != nil {
		return total, err
	} else {
		total += n
		t.Challenge = field
	}
	if field, n, err := scale.DecodeCompact(dec); err != nil {
		return total, err
	} else {
		total += n
		t.BitsPerLabel = uint(field)
	}
	if field, n, err := scale.DecodeCompact(dec); err != nil {
		return total, err
	} else {
		total += n
		t.LabelsPerUnit = uint(field)
	}
	if field, n, err := scale.DecodeCompact(dec); err != nil {
		return total, err
	} else {
		total += n
		t.K1 = uint(field)
	}
	if field, n, err := scale.DecodeCompact(dec); err != nil {
		return total, err
	} else {
		total += n
		t.K2 = uint(field)
	}
	return total, nil
}
