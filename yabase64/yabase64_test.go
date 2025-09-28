package yabase64_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabase64"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase64_FlowWorks(t *testing.T) {
	in := sample{
		ID:    7,
		Name:  "RZK",
		Tags:  []string{"a", "b", "c"},
		Meta:  map[string]string{"k1": "v1", "k2": "v2"},
		Bytes: []byte{0, 1, 2, 250, 251, 252},
	}

	buf, err := yabase64.Encode(in)
	require.NoError(t, err, "encode failed")

	b64 := buf.String()

	out, yaerr := yabase64.Decode[sample](b64)
	require.Nil(t, yaerr, "decode failed: %v", yaerr)
	require.NotNil(t, out, "decoded value is nil")

	assert.Equal(t, in, *out, "mismatch after round-trip")
}

type sample struct {
	ID    int               `json:"id"`
	Name  string            `json:"name"`
	Tags  []string          `json:"tags"`
	Meta  map[string]string `json:"meta"`
	Bytes []byte            `json:"bytes"`
}
