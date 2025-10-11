// Package yarsa provides practical helpers to encrypt and decrypt arbitrary-length
// data with RSA-OAEP (SHA-256), handling chunking under the hood.
//
// The API is intentionally minimal:
//
//   - Encrypt(plaintext []byte, public  *rsa.PublicKey)  -> []byte (concatenated ciphertext blocks)
//   - Decrypt(cipher    []byte, private *rsa.PrivateKey) -> []byte (reconstructed plaintext)
//
// Notes:
//
//   - RSA-OAEP(SHA-256) with a 2048-bit key allows at most 190 bytes of plaintext
//     per block (k − 2*hLen − 2 = 256 − 2*32 − 2). Larger inputs are split into
//     190-byte chunks automatically.
//   - Ciphertext block size is always exactly the modulus size (256 bytes for
//     RSA-2048). Therefore, the total ciphertext length is a multiple of 256.
//   - Transport encodings (e.g., base64) are intentionally not handled here.
//     Keep base64 “at the edges” of your app.
//   - Errors are returned as yaerrors.Error with HTTP 500 semantics to match
//     the rest of your codebase.
//
// Example (basic round-trip with RSA-2048):
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//
//	msg := []byte("Hello, RZK! This may be longer than 190 bytes; it will be chunked automatically.")
//
//	// Encrypt - concatenated 256-byte blocks
//	ct, err := yarsa.Encrypt(msg, &key.PublicKey)
//	if err != nil {
//	    log.Fatalf("encrypt failed: %v", err)
//	}
//
//	// Decrypt - validate multiple of 256, then OAEP-decrypt each block
//	pt, err := yarsa.Decrypt(ct, key)
//	if err != nil {
//	    log.Fatalf("decrypt failed: %v", err)
//	}
//
//	fmt.Println(string(pt)) // "Hello, RZK! …"
package yarsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Encrypt applies RSA-OAEP(SHA-256) to plaintext, chunking as needed.
// Each plaintext chunk (≤190 bytes for RSA-2048) is encrypted into a fixed-size
// 256-byte ciphertext block. All blocks are concatenated and returned.
//
// Example:
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	plaintext := []byte("Hello, this message will be chunked at 190 bytes if longer.")
//
//	ciphertext, err := yarsa.Encrypt(plaintext, &key.PublicKey)
//	if err != nil {
//	    log.Fatalf("encrypt failed: %v", err)
//	}
//
//	fmt.Printf("ciphertext length: %d\n", len(ciphertext))
//
// Returns:
//   - []byte: concatenated ciphertext blocks
//   - yaerrors.Error: wrapped error with HTTP 500 semantics
func Encrypt(plaintext []byte, public *rsa.PublicKey) ([]byte, yaerrors.Error) {
	hash := sha256.New()

	label := []byte{}

	const padding = 2

	chunksCount := public.Size() - padding*sha256.Size - padding
	if chunksCount <= 0 {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"[RSA] invalid OAEP max chunk size",
		)
	}

	var out []byte

	for i := 0; i < len(plaintext); i += chunksCount {
		end := i + chunksCount

		end = min(end, len(plaintext))

		block, err := rsa.EncryptOAEP(hash, rand.Reader, public, plaintext[i:end], label)
		if err != nil {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"[RSA] failed to encrypt chunk with OAEP",
			)
		}

		out = append(out, block...)
	}

	return out, nil
}

// Decrypt reverses Encrypt by splitting ciphertext into fixed-size blocks
// (256 bytes for RSA-2048), decrypting each with RSA-OAEP(SHA-256), and
// concatenating results.
//
// Example:
//
//	key, _ := rsa.GenerateKey(rand.Reader, 2048)
//	plaintext := []byte("Hello, this message will be encrypted and decrypted.")
//
//	ciphertext, _ := yarsa.Encrypt(plaintext, &key.PublicKey)
//
//	decrypted, err := yarsa.Decrypt(ciphertext, key)
//	if err != nil {
//	    log.Fatalf("decrypt failed: %v", err)
//	}
//
//	fmt.Println(string(decrypted)) // "Hello, this message will be encrypted and decrypted."
//
// Returns:
//   - []byte: reconstructed plaintext
//   - yaerrors.Error: wrapped error with HTTP 500 semantics
func Decrypt(ciphertext []byte, private *rsa.PrivateKey) ([]byte, yaerrors.Error) {
	hash := sha256.New()

	label := []byte{}

	blockSize := private.Size()

	if len(ciphertext)%blockSize != 0 {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			fmt.Sprintf("[RSA] ciphertext length is not a multiple of RSA block size (expected exact {%d}-byte blocks)", blockSize),
		)
	}

	var out []byte

	for i := 0; i < len(ciphertext); i += blockSize {
		end := i + blockSize

		plain, err := rsa.DecryptOAEP(hash, rand.Reader, private, ciphertext[i:end], label)
		if err != nil {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"[RSA] failed to decrypt chunk with OAEP",
			)
		}

		out = append(out, plain...)
	}

	return out, nil
}
