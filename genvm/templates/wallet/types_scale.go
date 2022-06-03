// Code generated by github.com/spacemeshos/go-scale/gen. DO NOT EDIT.

package wallet

import (
	"github.com/spacemeshos/go-scale"
)

func (t *SpendArguments) EncodeScale(enc *scale.Encoder) (total int, err error) {
	// field Destination (0)
	if n, err := t.Destination.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}

	// field Amount (1)
	if n, err := scale.EncodeCompact64(enc, t.Amount); err != nil {
		return total, err
	} else {
		total += n
	}

	return total, nil
}

func (t *SpendArguments) DecodeScale(dec *scale.Decoder) (total int, err error) {
	// field Destination (0)
	if n, err := t.Destination.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}

	// field Amount (1)
	if field, n, err := scale.DecodeCompact64(dec); err != nil {
		return total, err
	} else {
		total += n
		t.Amount = field
	}

	return total, nil
}

func (t *SpawnArguments) EncodeScale(enc *scale.Encoder) (total int, err error) {
	// field PublicKey (0)
	if n, err := t.PublicKey.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}

	return total, nil
}

func (t *SpawnArguments) DecodeScale(dec *scale.Decoder) (total int, err error) {
	// field PublicKey (0)
	if n, err := t.PublicKey.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}

	return total, nil
}

func (t *SpendPayload) EncodeScale(enc *scale.Encoder) (total int, err error) {
	// field Arguments (0)
	if n, err := t.Arguments.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}

	// field Nonce (1)
	if n, err := t.Nonce.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}

	// field GasPrice (2)
	if n, err := scale.EncodeCompact32(enc, t.GasPrice); err != nil {
		return total, err
	} else {
		total += n
	}

	return total, nil
}

func (t *SpendPayload) DecodeScale(dec *scale.Decoder) (total int, err error) {
	// field Arguments (0)
	if n, err := t.Arguments.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}

	// field Nonce (1)
	if n, err := t.Nonce.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}

	// field GasPrice (2)
	if field, n, err := scale.DecodeCompact32(dec); err != nil {
		return total, err
	} else {
		total += n
		t.GasPrice = field
	}

	return total, nil
}


func (t *SpawnPayload) EncodeScale(enc *scale.Encoder) (total int, err error) {
	// field Arguments (0)
	if n, err := t.Arguments.EncodeScale(enc); err != nil {
		return total, err
	} else {
		total += n
	}

	// field GasPrice (1)
	if n, err := scale.EncodeCompact32(enc, t.GasPrice); err != nil {
		return total, err
	} else {
		total += n
	}

	return total, nil
}

func (t *SpawnPayload) DecodeScale(dec *scale.Decoder) (total int, err error) {
	// field Arguments (0)
	if n, err := t.Arguments.DecodeScale(dec); err != nil {
		return total, err
	} else {
		total += n
	}

	// field GasPrice (1)
	if field, n, err := scale.DecodeCompact32(dec); err != nil {
		return total, err
	} else {
		total += n
		t.GasPrice = field
	}

	return total, nil
}
