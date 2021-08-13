package types

import (
	"io"

	cbor "github.com/fxamacker/cbor/v2"
)

// Encode ...
func Encode(w io.Writer, value interface{}) (int, error) {
	codec := cbor.NewEncoder(w)
	err := codec.Encode(value)
	return 0, err
}

// Decode ...
func Decode(r io.Reader, value interface{}) (int, error) {
	codec := cbor.NewDecoder(r)
	err := codec.Decode(value)
	return 0, err
}

// Marshal ...
func Marshal(value interface{}) ([]byte, error) {
	return cbor.Marshal(value)
}

// Unmarshal ...
func Unmarshal(buf []byte, value interface{}) error {
	return cbor.Unmarshal(buf, value)
}
