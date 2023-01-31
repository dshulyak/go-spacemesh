package simple

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"

	"github.com/minio/sha256-simd"
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

func Prove(cpu int, filename string, k1, k2 uint64) error {
	f, err := setup(filename)
	if f != nil {
		defer f.Close()
	}
	if err != nil {
		return err
	}
	var (
		proof      = make(chan uint64, k2)
		eg         errgroup.Group
		share      = 80 / cpu
		difficulty = provingDifficulty(512<<20, k1)
	)
	for i := 0; i < cpu; i++ {
		i := 0
		eg.Go(func() error {
			start := share * i
			end := share * (i + 1)
			buf := make([]byte, 1<<20)
			input := make([]byte, 37)
			copy(input, "any challenge")
			for {
				n, err := f.Read(buf)
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				for nonce := start; nonce <= end; nonce++ {
					// one byte label
					for _, b := range buf[:n] {
						binary.BigEndian.PutUint32(input[32:], uint32(nonce))
						input[36] = b
						r := sha256.Sum256(input)
						if r2 := binary.LittleEndian.Uint64(r[:]); r2 <= difficulty {
							select {
							case proof <- r2:
							default:
								return nil
							}
						}
					}
				}
				if errors.Is(err, io.EOF) {
					return nil
				}
			}
		})
	}
	return eg.Wait()
}
