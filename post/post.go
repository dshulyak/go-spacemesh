package simple

import (
	"encoding/binary"
	"errors"
	"io"
	"os"

	"github.com/minio/sha256-simd"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

const k1 = 2000      // irrelevant here
const k2 = 1800      // number of collected results
const difficulty = 1 // just to go through the file

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

func Prove(cpu int, filename string) error {
	f, err := setup(filename)
	if f != nil {
		defer f.Close()
	}
	if err != nil {
		return err
	}
	var (
		proof = make(chan uint64, k2)
		eg    errgroup.Group
	)
	for i := 0; i < cpu; i++ {
		eg.Go(func() error {
			buf := make([]byte, 1<<20)
			input := make([]byte, 37)
			copy(input, "any challenge")
			binary.BigEndian.PutUint32(input[32:], 101)
			for {
				n, err := f.Read(buf)
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				// one byte label
				for _, b := range buf[:n] {
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
				if errors.Is(err, io.EOF) {
					return nil
				}
			}
		})
	}
	return eg.Wait()
}
