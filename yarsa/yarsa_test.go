package yarsa_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yarsa"
	"github.com/stretchr/testify/assert"
)

func mustKey2048(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	return key
}

func TestMaxChunkFormula2048(t *testing.T) {
	const expected = 190

	key := mustKey2048(t)

	result := key.PublicKey.Size() - 2*sha256.Size - 2

	assert.Equal(t, expected, result)
}

func TestRoundTrip_SmallMessages(t *testing.T) {
	key := mustKey2048(t)

	vectors := [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("Hello, RZK!"),
		bytes.Repeat([]byte("x"), 15),
		bytes.Repeat([]byte("y"), 189),
		bytes.Repeat([]byte("z"), 190),
	}

	for i, msg := range vectors {
		ct, err := yarsa.Encrypt(msg, &key.PublicKey)
		if err != nil {
			t.Fatalf("case %d: encrypt failed: %v", i, err)
		}

		if len(ct)%key.PublicKey.Size() != 0 {
			t.Fatalf("case %d: ciphertext length %d not multiple of %d",
				i, len(ct), key.PublicKey.Size())
		}

		pt, err := yarsa.Decrypt(ct, key)
		if err != nil {
			t.Fatalf("case %d: decrypt failed: %v", i, err)
		}

		if !bytes.Equal(pt, msg) {
			t.Fatalf("case %d: plaintext mismatch\n got: %q\nwant: %q", i, pt, msg)
		}
	}
}

func TestRoundTrip_LargeMessages(t *testing.T) {
	key := mustKey2048(t)
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
		msg := make([]byte, n)
		if _, err := rand.Read(msg); err != nil {
			t.Fatalf("rand.Read failed: %v", err)
		}

		ct, err := yarsa.Encrypt(msg, &key.PublicKey)
		if err != nil {
			t.Fatalf("n=%d: encrypt failed: %v", n, err)
		}
		if len(ct)%key.PublicKey.Size() != 0 {
			t.Fatalf("n=%d: ciphertext length %d not multiple of %d",
				n, len(ct), key.PublicKey.Size())
		}

		pt, err := yarsa.Decrypt(ct, key)
		if err != nil {
			t.Fatalf("n=%d: decrypt failed: %v", n, err)
		}
		if !bytes.Equal(pt, msg) {
			t.Fatalf("n=%d: plaintext mismatch", n)
		}
	}
}

func TestDecrypt_WithWrongKey_ShouldFail(t *testing.T) {
	key1 := mustKey2048(t)

	key2 := mustKey2048(t)

	msg := []byte("wrong key test")
	ct, err := yarsa.Encrypt(msg, &key1.PublicKey)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if _, err := yarsa.Decrypt(ct, key2); err == nil {
		t.Fatalf("expected decrypt error with wrong key, got nil")
	}
}

func TestDecrypt_TamperedCiphertext_ShouldFail(t *testing.T) {
	key := mustKey2048(t)

	msg := []byte("tamper test")
	ct, err := yarsa.Encrypt(msg, &key.PublicKey)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if len(ct) == 0 {
		t.Fatalf("unexpected empty ciphertext")
	}
	ct[len(ct)/2] ^= 0xFF

	if _, err := yarsa.Decrypt(ct, key); err == nil {
		t.Fatalf("expected decrypt error on tampered ciphertext, got nil")
	}
}

func TestDecrypt_InvalidLength_ShouldFail(t *testing.T) {
	key := mustKey2048(t)

	msg := []byte("length test")
	ct, err := yarsa.Encrypt(msg, &key.PublicKey)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	ct = ct[:len(ct)-1]

	if _, err := yarsa.Decrypt(ct, key); err == nil {
		t.Fatalf("expected decrypt error for invalid block multiple, got nil")
	}
}

func FuzzEncryptDecrypt(f *testing.F) {
	seed := [][]byte{
		{},
		[]byte("a"),
		[]byte("hello"),
		bytes.Repeat([]byte{0}, 190),
		bytes.Repeat([]byte{1}, 191),
	}
	for _, s := range seed {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Skipf("keygen failed: %v", err)
		}

		ct, err := yarsa.Encrypt(data, &key.PublicKey)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}

		pt, err := yarsa.Decrypt(ct, key)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}

		if !bytes.Equal(pt, data) {
			t.Fatalf("round-trip mismatch")
		}
	})
}

func TestBudget_LargeMessage(t *testing.T) {
	t.Skip("enable manually")
	key := mustKey2048(t)
	const N = 190*50 + 77
	msg := make([]byte, N)
	_, _ = rand.Read(msg)

	ct, err := yarsa.Encrypt(msg, &key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	if len(ct)%key.PublicKey.Size() != 0 {
		t.Fatalf("ciphertext size not multiple of block size")
	}

	pt, err := yarsa.Decrypt(ct, key)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pt, msg) {
		t.Fatalf("plaintext mismatch")
	}
}
