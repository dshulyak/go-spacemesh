package simple

import (
	"testing"
)

func BenchmarkProof(b *testing.B) {
	b.SetBytes(512 << 20)
	for i := 0; i < b.N; i++ {
		if err := Prove(10, "/tmp/example", 10001, 2000, 1800); err != nil {
			b.Fatal(err)
		}
	}
}
