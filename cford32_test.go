package cford32

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompactRoundtrip(t *testing.T) {
	buf := make([]byte, 13)
	for i := uint64(0); i < (1 << 15); i++ {
		res := AppendCompact(i, buf[:0])
		back, err := Uint64(res)
		assert.NoError(t, err)
		assert.Equal(t, back, i, "%q: mismatch between encoded value (%d) and retrieved value (%d)", string(buf), i, back)
	}
	for i := uint64(1<<34 - 1024); i < (1<<34 + 1024); i++ {
		res := AppendCompact(i, buf[:0])
		back, err := Uint64(res)
		// println(string(res))
		assert.NoError(t, err)
		assert.Equal(t, back, i, "%q: mismatch between encoded value (%d) and retrieved value (%d)", string(buf), i, back)
	}
	for i := uint64(1<<64 - 5000); i != 0; i++ {
		res := AppendCompact(i, buf[:0])
		back, err := Uint64(res)
		assert.NoError(t, err)
		assert.Equal(t, back, i, "%q: mismatch between encoded value (%d) and retrieved value (%d)", string(buf), i, back)
	}
}

func BenchmarkCompact(b *testing.B) {
	buf := make([]byte, 13)
	for i := 0; i < b.N; i++ {
		_ = AppendCompact(uint64(i), buf[:0])
	}
}
