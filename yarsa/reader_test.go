package yarsa_test

import (
	"bytes"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yarsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicReader_SameSeedSameStream(t *testing.T) {
	t.Parallel()

	seed := []byte("correct-horse-battery-staple")

	r1 := yarsa.NewDeterministicReader(seed)
	r2 := yarsa.NewDeterministicReader(seed)

	out1 := make([]byte, 4096)
	out2 := make([]byte, 4096)

	n1, err1 := r1.Read(out1)
	n2, err2 := r2.Read(out2)

	require.NoError(t, err1)
	require.NoError(t, err2)

	require.Equal(t, len(out1), n1)
	require.Equal(t, len(out2), n2)

	assert.True(t, bytes.Equal(out1, out2), "streams differ for same seed")
}

func TestDeterministicReader_DifferentSeedsDiffer(t *testing.T) {
	t.Parallel()

	r1 := yarsa.NewDeterministicReader([]byte("seed-A"))
	r2 := yarsa.NewDeterministicReader([]byte("seed-B"))

	out1 := make([]byte, 256)
	out2 := make([]byte, 256)

	_, _ = r1.Read(out1)
	_, _ = r2.Read(out2)

	assert.False(t, bytes.Equal(out1, out2), "different seeds produced identical output")
}

func TestDeterministicReader_MultiReadEqualsSingleRead(t *testing.T) {
	t.Parallel()

	seed := []byte("split-read")
	rAll := yarsa.NewDeterministicReader(seed)
	rParts := yarsa.NewDeterministicReader(seed)

	full := make([]byte, 10*1024+13)
	_, err := rAll.Read(full)
	require.NoError(t, err)

	part := make([]byte, 0, len(full))

	chunks := []int{1, 3, 7, 31, 32, 33, 1000, 4096, len(full) - (1 + 3 + 7 + 31 + 32 + 33 + 1000 + 4096)}
	for _, n := range chunks {
		buf := make([]byte, n)
		_, err := rParts.Read(buf)
		require.NoError(t, err)
		part = append(part, buf...)
	}

	assert.Equal(t, full, part, "split reads do not match single read")
}

func TestDeterministicReader_ZeroLengthRead(t *testing.T) {
	t.Parallel()

	r := yarsa.NewDeterministicReader([]byte("zlr"))
	buf := make([]byte, 0)

	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
func TestDeterministicReader_LongRead_ManyRefills(t *testing.T) {
	t.Parallel()

	r := yarsa.NewDeterministicReader([]byte("long-long-seed"))

	N := 1 << 20
	buf := make([]byte, N)

	n, err := r.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, N, n)

	r2 := yarsa.NewDeterministicReader([]byte("long-long-seed"))
	buf2 := make([]byte, N)
	_, _ = r2.Read(buf2)

	assert.Equal(t, buf2, buf)
}

func TestDeterministicReader_SeedCopyIsolation(t *testing.T) {
	t.Parallel()

	seed := []byte("mutable")
	r1 := yarsa.NewDeterministicReader(seed)

	seed[0] ^= 0xFF

	out1 := make([]byte, 256)
	_, _ = r1.Read(out1)

	r2 := yarsa.NewDeterministicReader(seed)
	out2 := make([]byte, 256)
	_, _ = r2.Read(out2)

	original := []byte("mutable")
	r1Expected := yarsa.NewDeterministicReader(original)

	exp := make([]byte, 256)
	_, _ = r1Expected.Read(exp)

	assert.True(t, bytes.Equal(out1, exp), "reader created before seed mutation changed unexpectedly")
	assert.False(t, bytes.Equal(out1, out2), "reader created after mutation should differ from original")
}
