package types

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/util"
)

// Bytes returns the BlockID as a byte slice.
func (id BlockID) Bytes() []byte { return id.AsHash32().Bytes() }

// Bytes returns the byte representation of the LayerID, using little endian encoding.
func (l LayerID) Bytes() []byte { return util.Uint32ToBytes(l.Value) }

// BlockIdsToBytes serializes a slice of BlockIDs.
func BlockIdsToBytes(ids []BlockID) ([]byte, error) {
	var w bytes.Buffer
	SortBlockIDs(ids)
	if _, err := Encode(&w, &ids); err != nil {
		return nil, errors.New("error marshalling block ids ")
	}
	return w.Bytes(), nil
}

// BytesToBlockIds deserializes a slice of BlockIDs.
func BytesToBlockIds(blockIds []byte) ([]BlockID, error) {
	var ids []BlockID
	if _, err := Decode(bytes.NewReader(blockIds), &ids); err != nil {
		return nil, fmt.Errorf("error marshaling layer: %v", err)
	}
	return ids, nil
}

// BytesToAtx deserializes an ActivationTx.
func BytesToAtx(b []byte) (*ActivationTx, error) {
	buf := bytes.NewReader(b)
	var atx ActivationTx
	_, err := Decode(buf, &atx)
	if err != nil {
		return nil, err
	}
	return &atx, nil
}

// NIPostChallengeToBytes serializes a NIPostChallenge.
func NIPostChallengeToBytes(challenge *NIPostChallenge) ([]byte, error) {
	var w bytes.Buffer
	if _, err := Encode(&w, challenge); err != nil {
		return nil, fmt.Errorf("error marshalling NIPost Challenge: %v", err)
	}
	return w.Bytes(), nil
}

// BytesToTransaction deserializes a Transaction.
func BytesToTransaction(buf []byte) (*Transaction, error) {
	b := Transaction{}
	_, err := Decode(bytes.NewReader(buf), &b)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// BytesToInterface deserializes any type.
// ⚠️ Pass the interface by reference
func BytesToInterface(buf []byte, i interface{}) error {
	_, err := Decode(bytes.NewReader(buf), i)
	if err != nil {
		return err
	}
	return nil
}

// InterfaceToBytes serializes any type.
// ⚠️ Pass the interface by reference
func InterfaceToBytes(i interface{}) ([]byte, error) {
	var w bytes.Buffer
	if _, err := Encode(&w, &i); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// ATXIdsToBytes serializes a slice of atx ids.
func ATXIdsToBytes(ids []ATXID) ([]byte, error) {
	var w bytes.Buffer
	SortAtxIDs(ids)
	if _, err := Encode(&w, &ids); err != nil {
		return nil, errors.New("error marshalling block ids ")
	}
	return w.Bytes(), nil
}

// BytesToLayerID return uint64 layer IO
func BytesToLayerID(b []byte) LayerID {
	return NewLayerID(util.BytesToUint32(b))
}
