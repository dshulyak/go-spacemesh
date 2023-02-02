package simple

import (
	"testing"
)

func BenchmarkProof(b *testing.B) {
	b.SetBytes(512 << 20)
	for i := 0; i < b.N; i++ {
		if _, err := Prove(1, "/tmp/example", []byte("challenge"), 0, 2000, 1800); err != nil {
			b.Fatal(err)
		}
	}
}
