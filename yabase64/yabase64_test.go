package yabase64_test

import (
	"bytes"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabase64"
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
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	b64 := buf.String()

	out, yaerr := yabase64.Decode[sample](b64)
	if yaerr != nil {
		t.Fatalf("decode failed: %v", yaerr)
	}

	if !equal(in, *out) {
		t.Fatalf("mismatch after round-trip\nin:  %+v\nout: %+v", in, *out)
	}
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
