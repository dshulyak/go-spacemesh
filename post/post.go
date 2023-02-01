package simple

import (
	"crypto/aes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

func provingDifficulty(numLabels uint64, k1 uint64) uint64 {
	const maxTarget = math.MaxUint64
	x := maxTarget / numLabels
	y := maxTarget % numLabels
	return x*k1 + (y*k1)/numLabels
}

func setup(filename string) (*os.File, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return f, err
	}
	if err := unix.Fadvise(int(f.Fd()), 0, stat.Size(), unix.FADV_SEQUENTIAL); err != nil {
		return f, err
	}
	return f, nil
}

func Prove(cpu int, filename string, challenge []byte, nonce uint32, k1, k2 uint64) ([]uint64, error) {
	f, err := setup(filename)
	if f != nil {
		defer f.Close()
	}
	if err != nil {
		return nil, err
	}
	var (
		proof      = make([]uint64, k2)
		position   uint64
		step       = 1 << 20
		eg         errgroup.Group
		difficulty = provingDifficulty(256<<30, k1)

		mu    sync.Mutex
		index uint64
	)

	for i := 0; i < cpu; i++ {
		eg.Go(func() error {
			buf := make([]byte, step)
			dst := [64]byte{}
			src := [64]byte{}
			kernel, err := aes.NewCipher(make([]byte, 32))
			if err != nil {
				return err
			}
			for {
				mu.Lock()
				n, err := f.Read(buf)
				i := index
				index += uint64(n)
				mu.Unlock()

				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				for _, b := range buf[:n] {
					binary.BigEndian.PutUint32(src[0:], uint32(nonce))
					src[5] = b
					kernel.Encrypt(dst[:16], src[:16])
					if r2 := binary.BigEndian.Uint64(dst[:]); r2 <= difficulty {
						pos := atomic.AddUint64(&position, 1)
						if pos >= k2 {
							return nil
						}
						proof[pos-1] = i
					}
					i++
				}
				if errors.Is(err, io.EOF) {
					return nil
				}
			}
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	// check positition >= k2
	return proof, nil
}
