package simple

import (
	"testing"

	"github.com/minio/sha256-simd"
)

func BenchmarkProof(b *testing.B) {
	b.SetBytes(512 << 20)
	for i := 0; i < b.N; i++ {
		if _, err := Prove(1, "/tmp/example", []byte("challenge"), 10001, 2000, 1800); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRaw(b *testing.B) {
	buf := make([]byte, 1)
	b.SetBytes(int64(len(buf)))
	h := sha256.New().(*sha256.Digest)
	d := [32]byte{}
	for i := 0; i < b.N; i++ {
		h.Write(buf)
		h.CheckSumInto(&d)
		h.Reset()
	}
}
