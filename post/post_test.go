package simple

import (
	"testing"
)

func BenchmarkProof(b *testing.B) {
	b.SetBytes(512 << 20)
	for i := 0; i < b.N; i++ {
		if _, err := Prove(4, "/tmp/example", []byte("challenge"), 10001, 2000, 1800); err != nil {
			b.Fatal(err)
		}
	}
}
