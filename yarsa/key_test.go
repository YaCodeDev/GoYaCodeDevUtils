package yarsa_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yarsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pubFingerprint(t *testing.T, pub *rsa.PublicKey) [32]byte {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)

	return sha256.Sum256(der)
}

func Test_GenerateDeterministicRSA_Determinism(t *testing.T) {
	t.Parallel()

	const bits = 2048

	seed := []byte("correct-horse-battery-staple")

	key1, err := yarsa.GenerateDeterministicRSA(yarsa.KeyOpts{Bits: bits, E: 65537, Seed: seed})
	require.NoError(t, err)

	key2, err := yarsa.GenerateDeterministicRSA(yarsa.KeyOpts{Bits: bits, E: 65537, Seed: seed})
	require.NoError(t, err)

	assert.Equal(
		t,
		pubFingerprint(t, &key1.PublicKey),
		pubFingerprint(t, &key2.PublicKey),
		"public keys differ for the same seed",
	)
	assert.Equal(t, key1.D, key2.D, "private exponent differs for the same seed")
	assert.Equal(t, 2, len(key1.Primes))
	assert.Equal(t, bits, key1.N.BitLen(), "modulus bit length mismatch")
}

func Test_GenerateDeterministicRSA_DifferentSeedsDiffer(t *testing.T) {
	t.Parallel()

	const bits = 2048

	seedA := []byte("seed-A")
	seedB := []byte("seed-B")

	keyA, err := yarsa.GenerateDeterministicRSA(yarsa.KeyOpts{Bits: bits, E: 65537, Seed: seedA})
	require.NoError(t, err)

	keyB, err := yarsa.GenerateDeterministicRSA(yarsa.KeyOpts{Bits: bits, E: 65537, Seed: seedB})
	require.NoError(t, err)

	assert.NotEqual(
		t,
		pubFingerprint(t, &keyA.PublicKey),
		pubFingerprint(t, &keyB.PublicKey),
		"different seeds yielded identical public keys",
	)

	assert.NotEqual(t, keyA.D, keyB.D, "different seeds yielded identical private exponents")
}

func Test_GenerateDeterministicRSA_DefaultExponent_And_PrimeOrder(t *testing.T) {
	t.Parallel()

	key, err := yarsa.GenerateDeterministicRSA(
		yarsa.KeyOpts{Bits: 2048, E: 0, Seed: []byte("exp-default")},
	)

	require.NoError(t, err)

	assert.Equal(t, 65537, key.E, "default exponent should be 65537")

	require.Equal(t, 2, len(key.Primes))

	assert.True(t, key.Primes[0].Cmp(key.Primes[1]) > 0, "expected p > q ordering")
}

func Test_GenerateDeterministicRSA_MultiBitLengths(t *testing.T) {
	t.Parallel()

	for _, bits := range []int{2048, 4096} {
		t.Run(fmt.Sprintf("bits=%d", bits), func(t *testing.T) {
			t.Parallel()

			key, err := yarsa.GenerateDeterministicRSA(
				yarsa.KeyOpts{Bits: bits, E: 65537, Seed: []byte("multi")},
			)

			require.NoError(t, err)

			assert.Equal(t, bits, key.N.BitLen(), "modulus bit length mismatch")

			assert.NoError(t, key.Validate(), "stdlib rsa key validation failed")
		})
	}
}

func Test_GenerateDeterministicRSA_InvalidOpts(t *testing.T) {
	t.Parallel()

	_, err := yarsa.GenerateDeterministicRSA(yarsa.KeyOpts{Bits: 511, E: 65537, Seed: []byte("x")})
	assert.Error(t, err, "odd bit length should fail")

	_, err = yarsa.GenerateDeterministicRSA(yarsa.KeyOpts{Bits: 2048, E: 65537, Seed: nil})
	assert.Error(t, err, "missing seed should fail")
}

func Test_ParsePrivateKey_AllFormats(t *testing.T) {
	t.Parallel()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pkcs1DER := x509.MarshalPKCS1PrivateKey(priv)
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkcs1DER})

	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)

	pkcs8PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8DER})

	t.Run("[PEM] PKCS1", func(t *testing.T) {
		t.Parallel()

		got, err := yarsa.ParsePrivateKey(string(pkcs1PEM))

		assert.Nil(t, err, "parse pkcs1 pem error: %v", err)

		require.NotNil(t, got)

		assert.Equal(t, priv.N, got.N)
	})

	t.Run("[PEM] PKCS8", func(t *testing.T) {
		t.Parallel()

		got, err := yarsa.ParsePrivateKey(string(pkcs8PEM))

		assert.Nil(t, err, "parse pkcs8 pem error: %v", err)

		require.NotNil(t, got)

		assert.Equal(t, priv.N, got.N)
	})

	t.Run("[Base64 std] PKCS1 DER", func(t *testing.T) {
		t.Parallel()

		b64 := base64.StdEncoding.EncodeToString(pkcs1DER)

		got, err := yarsa.ParsePrivateKey(b64)

		assert.Nil(t, err, "parse base64(pkcs1 der) error: %v", err)

		require.NotNil(t, got)

		assert.Equal(t, priv.N, got.N)
	})

	t.Run("[Base64 std] PKCS8 DER", func(t *testing.T) {
		t.Parallel()

		b64 := base64.StdEncoding.EncodeToString(pkcs8DER)

		got, err := yarsa.ParsePrivateKey(b64)

		assert.Nil(t, err, "parse base64(pkcs8 der) error: %v", err)

		require.NotNil(t, got)

		assert.Equal(t, priv.N, got.N)
	})

	t.Run("[Base64 URL raw] PKCS1 DER (no padding)", func(t *testing.T) {
		t.Parallel()

		b64url := base64.RawURLEncoding.EncodeToString(pkcs1DER)

		got, err := yarsa.ParsePrivateKey(b64url)

		assert.Nil(t, err, "parse rawURL b64(pkcs1 der) error: %v", err)

		require.NotNil(t, got)

		assert.Equal(t, priv.N, got.N)
	})

	t.Run("[Base64 URL padded] PKCS8 DER", func(t *testing.T) {
		t.Parallel()

		b64url := base64.URLEncoding.EncodeToString(pkcs8DER)

		got, err := yarsa.ParsePrivateKey(b64url)

		assert.Nil(t, err, "parse URL b64(pkcs8 der) error: %v", err)

		require.NotNil(t, got)

		assert.Equal(t, priv.N, got.N)
	})

	t.Run("[CRLF-wrapped base64] PKCS1 DER", func(t *testing.T) {
		t.Parallel()

		b64 := base64.StdEncoding.EncodeToString(pkcs1DER)

		wrapped := bytes.Join([][]byte{
			[]byte(b64[:48]),
			[]byte(b64[48:96]),
			[]byte(b64[96:]),
		}, []byte("\r\n"))

		got, err := yarsa.ParsePrivateKey(string(wrapped))

		assert.Nil(t, err, "parse wrapped base64(pkcs1 der) error: %v", err)

		require.NotNil(t, got)

		assert.Equal(t, priv.N, got.N)
	})

	t.Run("[Invalid] garbage string", func(t *testing.T) {
		t.Parallel()

		got, err := yarsa.ParsePrivateKey("!!!not-a-key!!!")

		assert.NotNil(t, err, "expected error for invalid input")

		assert.Nil(t, got)
	})
}
