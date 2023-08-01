package hare3alt

import (
	"testing"

	"github.com/benbjohnson/clock"
)

func testHare(tb testing.TB, n int) {
	tb.Helper()
	wall := clock.NewMock()
	for i := 0; i < n; i++ {

	}
}

func TestHare(t *testing.T) {
	t.Run("small", func(t *testing.T) {
		testHare(t, 2)
	})

}
