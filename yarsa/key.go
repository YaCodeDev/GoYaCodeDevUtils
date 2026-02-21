// Package yarsa — RSA key utilities (deterministic keygen + private-key parsing).
//
// This file provides two main capabilities:
//
//  1. Deterministic RSA key generation:
//     GenerateDeterministicRSA(KeyOpts) -> *rsa.PrivateKey
//     - Reproducible for the same (Seed, Bits, E).
//     - Uses an internal DRBG (HMAC-SHA256(counter)) and a deterministic prime
//     search with the top TWO bits forced for each prime; that strongly biases
//     p and q to the top quarter of their ranges so the final modulus has the
//     requested bit-length.
//     - The stdlib rsa.GenerateKey is NOT guaranteed deterministic even with a
//     deterministic io.Reader (due to internal jitter), so custom prime generation
//     is implemented here.
//
//  2. Private key parsing convenience:
//     ParsePrivateKey(string) -> *rsa.PrivateKey
//     - Accepts:
//     * PEM (PKCS#1 “RSA PRIVATE KEY” or PKCS#8 “PRIVATE KEY”)
//     * Base64 of PEM (std or URL-safe, with/without padding)
//     * Raw DER bytes (PKCS#1 or PKCS#8) encoded as base64
//     - Returns yaerrors.Error on failure with HTTP-500 semantics to fit the
//     existing error handling style of this codebase.
//
// Notes:
//   - For deterministic keygen, supply a high-entropy Seed. A weak or guessable
//     seed trivially compromises the private key.
//   - StripCRLF(s) helps when keys are transported with line-wraps (pasted base64).
package yarsa

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

const (
	bigValueOne = 1
	bigValueTwo = 2
)

var (
	bigOne = big.NewInt(bigValueOne)
	bigTwo = big.NewInt(bigValueTwo)
)

// KeyOpts holds parameters for deterministic RSA key generation.
//   - Bits: modulus size (e.g., 2048, 3072, 4096). Must be even and >= 512.
//   - Exponent: public exponent (use 65537 if 0).
//   - Seed: high-entropy secret seed; same inputs -> same keypair.
type KeyOpts struct {
	Bits     int
	Exponent int
	Seed     []byte
}

// GenerateDeterministicRSAPrivateKey returns a reproducible *rsa.PrivateKey from KeyOpts.
// Implementation details:
//   - Uses a deterministic byte stream (NewDeterministicReader) to draw prime candidates.
//   - Forces each prime’s top two bits and oddness to ensure target bit length.
//   - Ensures gcd(e, p−1) == gcd(e, q−1) == 1 and p != q.
//   - Validates the key and precomputes CRT values.
//
// Errors if Bits invalid, Seed empty, or validation fails.
//
// Example:
//
//	opts := yarsa.KeyOpts{
//	    Bits: 2048,
//	    E:    65537,
//	    Seed: []byte("deterministic-seed"),
//	}
//
//	key, err := yarsa.GenerateDeterministicRSAPrivateKey(opts)
//	if err != nil {
//	    log.Fatalf("failed to generate key: %v", err)
//	}
//
//	// Calling again with the same seed -> identical key
//	key2, _ := yarsa.GenerateDeterministicRSAPrivateKey(opts)
//	fmt.Println(key.N.Cmp(key2.N) == 0) // true
func GenerateDeterministicRSAPrivateKey(opts KeyOpts) (*rsa.PrivateKey, yaerrors.Error) {
	if opts.Bits < 512 || opts.Bits%2 != 0 {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"bits must be even and >= 512",
		)
	}

	if opts.Exponent == 0 {
		opts.Exponent = 65537
	}

	if len(opts.Seed) == 0 {
		return nil, yaerrors.FromString(http.StatusInternalServerError, "seed required")
	}

	reader := NewDeterministicReader(opts.Seed)

	const bits = 2

	pBits := opts.Bits / bits
	qBits := opts.Bits - pBits

	e := big.NewInt(int64(opts.Exponent))

	var (
		p, q *big.Int
		err  yaerrors.Error
	)

	for {
		p, err = nextPrime(reader, pBits)
		if err != nil {
			return nil, err.Wrap("failed to get next prime")
		}

		pm1 := new(big.Int).Sub(p, bigOne)
		if new(big.Int).GCD(nil, nil, e, pm1).Cmp(bigOne) == 0 {
			break
		}
	}

	for {
		q, err = nextPrime(reader, qBits)
		if err != nil {
			return nil, err.Wrap("failed to get next prime")
		}

		if p.Cmp(q) == 0 {
			continue
		}

		qm1 := new(big.Int).Sub(q, bigOne)
		if new(big.Int).GCD(nil, nil, e, qm1).Cmp(bigOne) != 0 {
			continue
		}

		n := new(big.Int).Mul(p, q)
		if n.BitLen() == opts.Bits {
			break
		}
	}

	if p.Cmp(q) < 0 {
		p, q = q, p
	}

	n := new(big.Int).Mul(p, q)
	phi := new(big.Int).Mul(new(big.Int).Sub(p, bigOne), new(big.Int).Sub(q, bigOne))

	d := new(big.Int).ModInverse(e, phi)
	if d == nil {
		return nil, yaerrors.FromString(http.StatusInternalServerError, "no modular inverse for d")
	}

	private := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{
			N: n,
			E: int(e.Int64()),
		},
		D:      d,
		Primes: []*big.Int{new(big.Int).Set(p), new(big.Int).Set(q)},
	}

	if err := private.Validate(); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to validate private key",
		)
	}

	private.Precompute()

	return private, nil
}

// nextPrime returns a prime of exact bit length `bits` from reader r.
// It sets the top two bits and the low bit (odd), then checks ProbablyPrime(64).
// If the candidate isn’t prime, it does a bounded deterministic +2 search
// (staying within the bit length) before drawing fresh bytes again.
func nextPrime(r io.Reader, bits int) (*big.Int, yaerrors.Error) {
	const minBits = 2
	if bits < minBits {
		return nil, yaerrors.FromString(http.StatusInternalServerError, "bits too small")
	}

	const (
		bit7 = 7
		bit8 = 8
	)

	byteLen := (bits + bit7) / bit8
	buf := make([]byte, byteLen)

	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"failed to read full buffer",
			)
		}

		const mask = 0xFF

		topMask := byte(mask)
		if m := bits % bit8; m != 0 {
			topMask = mask >> (bit8 - m)
		}

		buf[0] &= topMask

		const bit2 = 2

		if bits%bit8 == 0 {
			buf[0] |= 0xC0
		} else {
			msb := uint((bits - 1) % bit8)
			nmsb := uint((bits - bit2) % bit8)
			buf[0] |= (1 << msb)
			buf[0] |= (1 << nmsb)
		}

		buf[len(buf)-1] |= 1

		cand := new(big.Int).SetBytes(buf)
		if cand.BitLen() != bits {
			continue
		}

		const prime = 64
		if cand.ProbablyPrime(prime) {
			return cand, nil
		}

		const bit12 = 12

		limit := 1 << bit12
		for range limit {
			cand.Add(cand, bigTwo)

			if cand.BitLen() != bits {
				break
			}

			if cand.ProbablyPrime(prime) {
				return cand, nil
			}
		}
	}
}

// ParsePrivateKey tries to parse an RSA private key provided as:
//  1. PEM: "-----BEGIN RSA PRIVATE KEY-----" (PKCS#1) or "-----BEGIN PRIVATE KEY-----" (PKCS#8)
//  2. Base64 of PEM (standard or URL-safe, with/without padding)
//  3. Raw DER bytes encoded as base64 (PKCS#1 or PKCS#8)
//
// It returns a *rsa.PrivateKey or a yaerrors.Error describing what failed.
//
// Example:
//
//	// Parse PEM-formatted private key
//	const pemKey = `-----BEGIN RSA PRIVATE KEY-----
//	MIIEowIBAAKCAQEA3vRcvK...
//	-----END RSA PRIVATE KEY-----`
//
//	key, err := yarsa.ParsePrivateKey(pemKey)
//	if err != nil {
//	    log.Fatalf("parse failed: %v", err)
//	}
//
//	fmt.Println("Modulus bits:", key.N.BitLen())
func ParsePrivateKey(s string) (*rsa.PrivateKey, yaerrors.Error) {
	input := strings.TrimSpace(s)

	if looksLikePEMPrivateKey(input) {
		key, err := parsePrivateKey([]byte(input))
		if err != nil {
			return nil, err.Wrap("[RSA] failed to parse private PEM key")
		}

		return key, nil
	}

	noCRLF := StripCRLF(input)

	decoded, err := base64.StdEncoding.DecodeString(noCRLF)
	if err != nil {
		if alt, altErr := tryBase64URLAll(noCRLF); altErr == nil {
			decoded = alt
		} else {
			return nil, yaerrors.FromString(
				http.StatusInternalServerError,
				"[RSA] invalid key: expected PEM (PKCS#1/PKCS#8) or base64 of PEM/DER",
			)
		}
	}

	if looksLikePEMPrivateKey(string(decoded)) {
		key, err := parsePrivateKey(decoded)
		if err != nil {
			return nil, err.Wrap("[RSA] failed to parse private PEM key")
		}

		return key, nil
	}

	if key, yaerr := parsePKCS1DER(decoded); yaerr == nil {
		return key, nil
	}

	if key, yaerr := parsePKCS8DER(decoded); yaerr == nil {
		return key, nil
	}

	return nil, yaerrors.FromString(
		http.StatusInternalServerError,
		"[RSA] invalid key: expected PEM (PKCS#1/PKCS#8) or base64 of PEM/DER",
	)
}

// looksLikePEMPrivateKey performs cheap string checks to detect any PEM
// private-key header/footer without fully decoding the PEM. It’s used as
// a fast-path before attempting base64.
func looksLikePEMPrivateKey(s string) bool {
	upper := strings.ToUpper(s)

	return strings.Contains(upper, "-----BEGIN ") &&
		strings.Contains(upper, " PRIVATE KEY-----")
}

// StripCRLF removes CR and LF characters and then trims surrounding spaces.
// This allows base64 payloads to be pasted with line wraps.
func StripCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")

	return strings.TrimSpace(s)
}

// tryBase64URLAll attempts to decode s as URL-safe base64 in both variants:
//   - RawURLEncoding (no '=' padding expected)
//   - URLEncoding (padding expected; add best-effort padding if missing)
//
// It returns decoded bytes or an error if neither variant works.
func tryBase64URLAll(s string) ([]byte, yaerrors.Error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}

	res, err := base64.URLEncoding.DecodeString(padBase64(s))
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[RSA] failed to decode string as bytes",
		)
	}

	return res, nil
}

// parsePrivateKey parses a PEM-encoded RSA private key in PKCS#1 or PKCS#8 form.
// Returns *rsa.PrivateKey or yaerrors.Error on failure.
func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, yaerrors.Error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"[RSA] failed to decode PEM block",
		)
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"[RSA] failed to parse PKCS#1",
			)
		}

		return key, nil

	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				err,
				"[RSA] failed to parse PKCS#8",
			)
		}

		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, yaerrors.FromString(
				http.StatusInternalServerError,
				"[RSA] PKCS#8 is not an RSA key",
			)
		}

		return key, nil
	default:
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"[RSA] unsupported PEM type: "+block.Type,
		)
	}
}

// parsePKCS1DER parses a PKCS#1 DER-encoded RSA private key.
func parsePKCS1DER(der []byte) (*rsa.PrivateKey, yaerrors.Error) {
	key, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[RSA] DER PKCS#1 parse failed",
		)
	}

	return key, nil
}

// parsePKCS8DER parses a PKCS#8 DER-encoded private key, ensuring the type is RSA.
func parsePKCS8DER(der []byte) (*rsa.PrivateKey, yaerrors.Error) {
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[RSA] DER PKCS#8 parse failed",
		)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"[RSA] DER PKCS#8 is not an RSA key",
		)
	}

	return key, nil
}

// padBase64 appends '=' characters until len(s) is a multiple of 4.
// This is a best-effort fix for inputs that dropped base64 padding.
func padBase64(s string) string {
	const (
		padding = 4
		change  = "="
	)

	if m := len(s) % padding; m != 0 {
		s += strings.Repeat(change, padding-m)
	}

	return s
}
