package yarsa_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yarsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func genKey2048(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)

	require.NoError(t, err, "failed to generate RSA key")

	return key
}

func TestEncryptAndDecrypt_Flow(t *testing.T) {
	t.Parallel()

	t.Run("[Math] MaxChunkFormula2048", func(t *testing.T) {
		t.Parallel()

		const expected = 190

		key := genKey2048(t)

		result := key.Size() - 2*sha256.Size - 2
		assert.Equal(t, expected, result)
	})

	t.Run("[RoundTrip] SmallMessages", func(t *testing.T) {
		t.Parallel()

		key := genKey2048(t)

		vectors := [][]byte{
			[]byte("a"),
			[]byte("Hello, RZK!"),
			bytes.Repeat([]byte("x"), 15),
			bytes.Repeat([]byte("y"), 189),
			bytes.Repeat([]byte("z"), 190),
		}

		for i, msg := range vectors {
			i, msg := i, msg
			t.Run(fmt.Sprintf("case#%d_len=%d", i, len(msg)), func(t *testing.T) {
				t.Parallel()

				ct, err := yarsa.Encrypt(msg, &key.PublicKey)
				require.NoError(t, err, "encrypt failed")

				assert.Equal(t, 0, len(ct)%key.Size(), "ciphertext length must be multiple of block size")

				pt, err := yarsa.Decrypt(ct, key)
				require.NoError(t, err, "decrypt failed")

				assert.Equal(t, msg, pt, "plaintext mismatch")
			})
		}
	})

	t.Run("[RoundTrip] LargeMessages", func(t *testing.T) {
		t.Parallel()

		key := genKey2048(t)
		const maxChunk = 190

		sizes := []int{
			maxChunk + 1,
			maxChunk*2 - 1,
			maxChunk * 2,
			maxChunk*2 + 17,
			maxChunk*3 + 123,
			maxChunk*10 + 3,
			maxChunk*20 + 77,
		}

		for _, n := range sizes {
			t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
				t.Parallel()

				msg := make([]byte, n)
				_, err := rand.Read(msg)
				require.NoError(t, err, "rand.Read failed")

				ct, err := yarsa.Encrypt(msg, &key.PublicKey)
				require.NoError(t, err, "encrypt failed")

				assert.Equal(t, 0, len(ct)%key.Size(), "ciphertext length must be multiple of block size")

				pt, err := yarsa.Decrypt(ct, key)
				require.NoError(t, err, "decrypt failed")

				assert.Equal(t, msg, pt, "plaintext mismatch")
			})
		}
	})

	t.Run("[Decrypt] WrongKey_ShouldFail", func(t *testing.T) {
		t.Parallel()

		key1 := genKey2048(t)
		key2 := genKey2048(t)

		msg := []byte("wrong key test")
		ct, err := yarsa.Encrypt(msg, &key1.PublicKey)
		require.NoError(t, err, "encrypt failed")

		_, err = yarsa.Decrypt(ct, key2)
		assert.Error(t, err, "expected decrypt error with wrong key")
	})

	t.Run("[Decrypt] TamperedCiphertext_ShouldFail", func(t *testing.T) {
		t.Parallel()

		key := genKey2048(t)

		msg := []byte("tamper test")
		ct, err := yarsa.Encrypt(msg, &key.PublicKey)
		require.NoError(t, err, "encrypt failed")
		require.NotEmpty(t, ct, "unexpected empty ciphertext")

		ct[len(ct)/2] ^= 0xFF

		_, err = yarsa.Decrypt(ct, key)
		assert.Error(t, err, "expected decrypt error on tampered ciphertext")
	})

	t.Run("[Decrypt] InvalidLength_ShouldFail", func(t *testing.T) {
		t.Parallel()

		key := genKey2048(t)

		msg := []byte("length test")
		ct, err := yarsa.Encrypt(msg, &key.PublicKey)
		require.NoError(t, err, "encrypt failed")
		require.Greater(t, len(ct), 0, "ciphertext should not be empty")

		ct = ct[:len(ct)-1]

		_, err = yarsa.Decrypt(ct, key)
		assert.Error(t, err, "expected decrypt error for invalid block multiple")
	})
}
