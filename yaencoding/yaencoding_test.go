package yaencoding_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	ID    int
	Name  string
	Tags  []string
	Meta  map[string]string
	Bytes []byte
}

func TestGobEncoding_Flow(t *testing.T) {
	t.Run("Full Round Trip", func(t *testing.T) {
		in := sample{
			ID:    7,
			Name:  "RZK",
			Tags:  []string{"a", "b", "c"},
			Meta:  map[string]string{"k1": "v1", "k2": "v2"},
			Bytes: []byte{0, 1, 2, 250, 251, 252},
		}

		b64, err := yaencoding.EncodeGob(in)
		require.NoError(t, err, "encode failed")

		out, yaerr := yaencoding.DecodeGob[sample](b64)
		require.Nil(t, yaerr, "decode failed")
		require.NotNil(t, out, "decoded value is nil")

		assert.Equal(t, in, *out, "mismatch after round-trip")
	})

	t.Run("Invalid Base64 Returns Error", func(t *testing.T) {
		out, err := yaencoding.DecodeGob[sample]("!!!INVALID!!!")
		require.Nil(t, out)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to decode base64")
	})

	t.Run("Invalid Gob Data Returns Error", func(t *testing.T) {
		invalid := yaencoding.ToString([]byte("not-gob-data"))
		out, err := yaencoding.DecodeGob[sample](invalid)
		require.Nil(t, out)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to decode gob")
	})

	t.Run("Utility ToString-ToBytes Round Trip", func(t *testing.T) {
		data := []byte{1, 2, 3, 4, 5}
		str := yaencoding.ToString(data)
		res, err := yaencoding.ToBytes(str)
		require.Nil(t, err)
		assert.Equal(t, data, res)
	})

	t.Run("Utility ToBytes Invalid Input", func(t *testing.T) {
		res, err := yaencoding.ToBytes("!!bad!!")
		require.Nil(t, res)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to decode string to bytes")
	})
}

func TestMessagePackEncoding_Flow(t *testing.T) {
	t.Run("Full Encode/Decode Round Trip", func(t *testing.T) {
		in := sample{
			ID:    42,
			Name:  "YaCode",
			Tags:  []string{"x", "y"},
			Meta:  map[string]string{"foo": "bar"},
			Bytes: []byte{1, 2, 3},
		}

		str, err := yaencoding.EncodeMessagePack(in)
		require.NoError(t, err, "encode failed")

		out, yaerr := yaencoding.DecodeMessagePack[sample](str)
		require.Nil(t, yaerr, "decode failed")
		require.NotNil(t, out)
		assert.Equal(t, in, *out)
	})

	t.Run("Invalid Base64 Returns Error", func(t *testing.T) {
		out, err := yaencoding.DecodeMessagePack[sample]("!invalid-base64")
		require.Nil(t, out)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to decode string as bytes")
	})

	t.Run("Invalid MessagePack Data Returns Error", func(t *testing.T) {
		b64 := yaencoding.ToString([]byte("not-msgpack-data"))
		out, err := yaencoding.DecodeMessagePack[sample](b64)
		require.Nil(t, out)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to marshal")
	})
}
