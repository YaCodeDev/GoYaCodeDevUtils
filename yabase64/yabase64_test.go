package yabase64_test

import (
	"bytes"
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

func equal(a, b sample) bool {
	if a.ID != b.ID || a.Name != b.Name {
		return false
	}

	if len(a.Tags) != len(b.Tags) || len(a.Meta) != len(b.Meta) || !bytes.Equal(a.Bytes, b.Bytes) {
		return false
	}

	for i := range a.Tags {
		if a.Tags[i] != b.Tags[i] {
			return false
		}
	}

	for k, v := range a.Meta {
		if b.Meta[k] != v {
			return false
		}
	}

	return true
}
