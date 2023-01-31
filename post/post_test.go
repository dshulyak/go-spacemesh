package simple

import (
	"testing"
)

func BenchmarkProof(b *testing.B) {
	b.SetBytes(512 << 20)
	for i := 0; i < b.N; i++ {
		if err := Prove(8, "/tmp/example"); err != nil {
			b.Fatal(err)
		}
	}
}
