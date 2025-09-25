package yarsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
)

func Encrypt(plaintext []byte, public *rsa.PublicKey) ([]byte, error) {
	hash := sha256.New()

	label := []byte(nil)

	maxChunk := public.Size() - 2*sha256.Size - 2
	if maxChunk <= 0 {
		return nil, errors.New("invalid OAEP max chunk size")
	}

	var out []byte

	for i := 0; i < len(plaintext); i += maxChunk {
		end := i + maxChunk

		end = min(end, len(plaintext))

		block, err := rsa.EncryptOAEP(hash, rand.Reader, public, plaintext[i:end], label)
		if err != nil {
			return nil, err
		}

		out = append(out, block...)
	}

	return out, nil
}

func Decrypt(ciphertext []byte, private *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.New()

	label := []byte(nil)

	blockSize := private.Size()
	if blockSize <= 0 {
		return nil, errors.New("invalid RSA modulus size")
	}

	if len(ciphertext)%blockSize != 0 {
		return nil, errors.New("ciphertext length is not a multiple of RSA block size (expected exact 256-byte blocks)")
	}

	var out []byte

	for i := 0; i < len(ciphertext); i += blockSize {
		end := i + blockSize

		plain, err := rsa.DecryptOAEP(hash, rand.Reader, private, ciphertext[i:end], label)
		if err != nil {
			return nil, err
		}

		out = append(out, plain...)
	}

	return out, nil
}
