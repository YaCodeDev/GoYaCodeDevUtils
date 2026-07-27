package yatotp_test

import (
	"encoding/base32"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatotp"
)

const (
	rfcSeedSHA1   = "12345678901234567890"
	rfcSeedSHA256 = "12345678901234567890123456789012"
	rfcSeedSHA512 = "1234567890123456789012345678901234567890123456789012345678901234"

	rfcDigits = yatotp.Digits(8)
	rfcPeriod = yatotp.Period(30)

	testAccount = yatotp.AccountName("user@example.com")
	testIssuer  = yatotp.Issuer("Example")
)

func encodeSecret(t *testing.T, seed string) yatotp.Secret {
	t.Helper()

	return yatotp.Secret(
		base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(seed)),
	)
}

func TestGenerateMatchesRFC6238Vectors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		algorithm yatotp.Algorithm
		seed      string
		unix      int64
		want      yatotp.Code
	}{
		{"sha1 at 59", yatotp.AlgorithmSHA1, rfcSeedSHA1, 59, "94287082"},
		{"sha1 at 1111111109", yatotp.AlgorithmSHA1, rfcSeedSHA1, 1111111109, "07081804"},
		{"sha1 at 1111111111", yatotp.AlgorithmSHA1, rfcSeedSHA1, 1111111111, "14050471"},
		{"sha1 at 1234567890", yatotp.AlgorithmSHA1, rfcSeedSHA1, 1234567890, "89005924"},
		{"sha1 at 2000000000", yatotp.AlgorithmSHA1, rfcSeedSHA1, 2000000000, "69279037"},
		{"sha1 at 20000000000", yatotp.AlgorithmSHA1, rfcSeedSHA1, 20000000000, "65353130"},
		{"sha256 at 59", yatotp.AlgorithmSHA256, rfcSeedSHA256, 59, "46119246"},
		{"sha256 at 1111111109", yatotp.AlgorithmSHA256, rfcSeedSHA256, 1111111109, "68084774"},
		{"sha256 at 1111111111", yatotp.AlgorithmSHA256, rfcSeedSHA256, 1111111111, "67062674"},
		{"sha256 at 1234567890", yatotp.AlgorithmSHA256, rfcSeedSHA256, 1234567890, "91819424"},
		{"sha256 at 2000000000", yatotp.AlgorithmSHA256, rfcSeedSHA256, 2000000000, "90698825"},
		{"sha256 at 20000000000", yatotp.AlgorithmSHA256, rfcSeedSHA256, 20000000000, "77737706"},
		{"sha512 at 59", yatotp.AlgorithmSHA512, rfcSeedSHA512, 59, "90693936"},
		{"sha512 at 1111111109", yatotp.AlgorithmSHA512, rfcSeedSHA512, 1111111109, "25091201"},
		{"sha512 at 1111111111", yatotp.AlgorithmSHA512, rfcSeedSHA512, 1111111111, "99943326"},
		{"sha512 at 1234567890", yatotp.AlgorithmSHA512, rfcSeedSHA512, 1234567890, "93441116"},
		{"sha512 at 2000000000", yatotp.AlgorithmSHA512, rfcSeedSHA512, 2000000000, "38618901"},
		{"sha512 at 20000000000", yatotp.AlgorithmSHA512, rfcSeedSHA512, 20000000000, "47863826"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			authenticator := yatotp.NewAuthenticator(&yatotp.Config{
				Algorithm: testCase.algorithm,
				Digits:    rfcDigits,
				Period:    rfcPeriod,
			})

			got, err := authenticator.Generate(
				encodeSecret(t, testCase.seed),
				time.Unix(testCase.unix, 0).UTC(),
			)
			if err != nil {
				t.Fatalf("generating an rfc vector should not fail: %v", err)
			}

			if got != testCase.want {
				t.Errorf("code should match the rfc vector: got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestAuthenticatorCounter(t *testing.T) {
	t.Parallel()

	t.Run("when instant is inside a step / then the step index is returned", func(t *testing.T) {
		t.Parallel()

		const (
			unix        = int64(59)
			wantCounter = yatotp.Counter(1)
		)

		authenticator := yatotp.NewAuthenticator(nil)

		if got := authenticator.Counter(time.Unix(unix, 0).UTC()); got != wantCounter {
			t.Errorf("counter should be the step index: got %d, want %d", got, wantCounter)
		}
	})

	t.Run("when instant precedes the epoch / then the counter clamps to zero", func(t *testing.T) {
		t.Parallel()

		const (
			unix        = int64(-1000)
			wantCounter = yatotp.Counter(0)
		)

		authenticator := yatotp.NewAuthenticator(nil)

		if got := authenticator.Counter(time.Unix(unix, 0).UTC()); got != wantCounter {
			t.Errorf("a pre-epoch instant should clamp: got %d, want %d", got, wantCounter)
		}
	})
}

func TestAuthenticatorValidate(t *testing.T) {
	t.Parallel()

	t.Run("when code is current / then it verifies at the current step", func(t *testing.T) {
		t.Parallel()

		const unix = int64(1234567890)

		at := time.Unix(unix, 0).UTC()
		authenticator := yatotp.NewAuthenticator(&yatotp.Config{Issuer: testIssuer})
		secret := encodeSecret(t, rfcSeedSHA1)

		code, err := authenticator.Generate(secret, at)
		if err != nil {
			t.Fatalf("generating a code should not fail: %v", err)
		}

		matched, verified, err := authenticator.Validate(secret, code, at)
		if err != nil {
			t.Fatalf("validating a fresh code should not fail: %v", err)
		}

		if !verified {
			t.Fatal("a freshly generated code should verify")
		}

		if matched != authenticator.Counter(at) {
			t.Errorf(
				"matched counter should be the current step: got %d, want %d",
				matched,
				authenticator.Counter(at),
			)
		}
	})

	t.Run("when code is one step old / then default skew accepts it", func(t *testing.T) {
		t.Parallel()

		const unix = int64(1234567890)

		at := time.Unix(unix, 0).UTC()
		authenticator := yatotp.NewAuthenticator(&yatotp.Config{Skew: yatotp.DefaultSkew})
		secret := encodeSecret(t, rfcSeedSHA1)

		previous := authenticator.Counter(at) - 1

		code, err := authenticator.GenerateAt(secret, previous)
		if err != nil {
			t.Fatalf("generating a previous-step code should not fail: %v", err)
		}

		matched, verified, err := authenticator.Validate(secret, code, at)
		if err != nil {
			t.Fatalf("validating a previous-step code should not fail: %v", err)
		}

		if !verified {
			t.Fatal("one step of drift should be tolerated")
		}

		if matched != previous {
			t.Errorf(
				"matched counter should be the previous step: got %d, want %d",
				matched,
				previous,
			)
		}
	})

	t.Run("when code is outside the skew window / then it is rejected", func(t *testing.T) {
		t.Parallel()

		const (
			unix       = int64(1234567890)
			stepsStale = yatotp.Counter(5)
		)

		at := time.Unix(unix, 0).UTC()
		authenticator := yatotp.NewAuthenticator(&yatotp.Config{Skew: yatotp.DefaultSkew})
		secret := encodeSecret(t, rfcSeedSHA1)

		code, err := authenticator.GenerateAt(secret, authenticator.Counter(at)-stepsStale)
		if err != nil {
			t.Fatalf("generating a stale code should not fail: %v", err)
		}

		_, verified, err := authenticator.Validate(secret, code, at)
		if err != nil {
			t.Fatalf("validating a stale code should not error: %v", err)
		}

		if verified {
			t.Fatal("a code well outside the skew window should be rejected")
		}
	})

	t.Run("when skew is zero / then only the current step verifies", func(t *testing.T) {
		t.Parallel()

		const unix = int64(1234567890)

		at := time.Unix(unix, 0).UTC()
		authenticator := yatotp.NewAuthenticator(&yatotp.Config{Skew: 0})
		secret := encodeSecret(t, rfcSeedSHA1)

		code, err := authenticator.GenerateAt(secret, authenticator.Counter(at)-1)
		if err != nil {
			t.Fatalf("generating a previous-step code should not fail: %v", err)
		}

		_, verified, err := authenticator.Validate(secret, code, at)
		if err != nil {
			t.Fatalf("validating should not error: %v", err)
		}

		if verified {
			t.Fatal("zero skew should reject a previous-step code")
		}
	})

	t.Run(
		"when code has the wrong shape / then validation reports a bad request",
		func(t *testing.T) {
			t.Parallel()

			const wantCode = http.StatusBadRequest

			shapes := map[string]yatotp.Code{
				"empty":       "",
				"too short":   "12345",
				"too long":    "1234567",
				"not numeric": "12a456",
			}

			authenticator := yatotp.NewAuthenticator(nil)
			secret := encodeSecret(t, rfcSeedSHA1)

			for name, code := range shapes {
				_, verified, err := authenticator.Validate(secret, code, time.Unix(1, 0).UTC())
				if err == nil {
					t.Errorf("%s code should fail validation", name)

					continue
				}

				if err.Code() != wantCode {
					t.Errorf(
						"%s code should report a bad request: got %d, want %d",
						name,
						err.Code(),
						wantCode,
					)
				}

				if verified {
					t.Errorf("%s code must not verify", name)
				}
			}
		},
	)

	t.Run("when secret is unusable / then validation fails", func(t *testing.T) {
		t.Parallel()

		const shortSecret = yatotp.Secret("GEZDGNBV")

		authenticator := yatotp.NewAuthenticator(nil)
		at := time.Unix(1234567890, 0).UTC()

		_, verified, err := authenticator.Validate(shortSecret, "123456", at)
		if err == nil {
			t.Fatal("a too-short secret should fail validation")
		}

		if verified {
			t.Error("a failed validation must not verify")
		}
	})
}

func TestAuthenticatorValidateAfter(t *testing.T) {
	t.Parallel()

	t.Run("when the step is newer than the last accepted / then it verifies", func(t *testing.T) {
		t.Parallel()

		const unix = int64(1234567890)

		at := time.Unix(unix, 0).UTC()
		authenticator := yatotp.NewAuthenticator(nil)
		secret := encodeSecret(t, rfcSeedSHA1)

		code, err := authenticator.Generate(secret, at)
		if err != nil {
			t.Fatalf("generating a code should not fail: %v", err)
		}

		matched, verified, err := authenticator.ValidateAfter(secret, code, at, 0)
		if err != nil {
			t.Fatalf("validating a fresh code should not fail: %v", err)
		}

		if !verified {
			t.Fatal("a code newer than the last accepted step should verify")
		}

		if matched != authenticator.Counter(at) {
			t.Errorf("matched counter should be the current step: got %d", matched)
		}
	})

	t.Run("when the same step is presented twice / then the replay is refused", func(t *testing.T) {
		t.Parallel()

		const unix = int64(1234567890)

		at := time.Unix(unix, 0).UTC()
		authenticator := yatotp.NewAuthenticator(nil)
		secret := encodeSecret(t, rfcSeedSHA1)

		code, err := authenticator.Generate(secret, at)
		if err != nil {
			t.Fatalf("generating a code should not fail: %v", err)
		}

		accepted, verified, err := authenticator.ValidateAfter(secret, code, at, 0)
		if err != nil || !verified {
			t.Fatalf("the first presentation should verify: verified=%v err=%v", verified, err)
		}

		_, replayed, err := authenticator.ValidateAfter(secret, code, at, accepted)
		if err != nil {
			t.Fatalf("a replay should not error: %v", err)
		}

		if replayed {
			t.Fatal("the same time step must not be accepted twice")
		}
	})

	t.Run(
		"when the code is invalid / then the last accepted step is irrelevant",
		func(t *testing.T) {
			t.Parallel()

			const (
				unix        = int64(1234567890)
				wrongCode   = yatotp.Code("000000")
				lastCounter = yatotp.Counter(0)
			)

			at := time.Unix(unix, 0).UTC()
			authenticator := yatotp.NewAuthenticator(nil)

			_, verified, err := authenticator.ValidateAfter(
				encodeSecret(t, rfcSeedSHA1),
				wrongCode,
				at,
				lastCounter,
			)
			if err != nil {
				t.Fatalf("a wrong code should not error: %v", err)
			}

			if verified {
				t.Fatal("a wrong code must not verify")
			}
		},
	)
}

func TestProvisioningURI(t *testing.T) {
	t.Parallel()

	t.Run("when issuer is set / then the uri carries label and query", func(t *testing.T) {
		t.Parallel()

		const (
			wantScheme = "otpauth"
			wantHost   = "totp"
		)

		authenticator := yatotp.NewAuthenticator(&yatotp.Config{Issuer: testIssuer})
		secret := encodeSecret(t, rfcSeedSHA1)

		raw, err := authenticator.ProvisioningURI(secret, testAccount)
		if err != nil {
			t.Fatalf("building a provisioning uri should not fail: %v", err)
		}

		parsed, parseErr := url.Parse(string(raw))
		if parseErr != nil {
			t.Fatalf("the provisioning uri should parse: %v", parseErr)
		}

		if parsed.Scheme != wantScheme {
			t.Errorf("scheme should be otpauth: got %q", parsed.Scheme)
		}

		if parsed.Host != wantHost {
			t.Errorf("host should be totp: got %q", parsed.Host)
		}

		wantLabel := "/" + string(testIssuer) + ":" + string(testAccount)
		if parsed.Path != wantLabel {
			t.Errorf("label should be issuer:account: got %q, want %q", parsed.Path, wantLabel)
		}

		query := parsed.Query()

		if query.Get("secret") != string(secret) {
			t.Errorf("query should carry the secret: got %q, want %q", query.Get("secret"), secret)
		}

		if query.Get("issuer") != string(testIssuer) {
			t.Errorf("query should carry the issuer: got %q", query.Get("issuer"))
		}

		if query.Get("algorithm") != string(yatotp.AlgorithmSHA1) {
			t.Errorf("query should carry the algorithm: got %q", query.Get("algorithm"))
		}

		if query.Get("digits") != "6" {
			t.Errorf("query should carry the digit count: got %q", query.Get("digits"))
		}

		if query.Get("period") != "30" {
			t.Errorf("query should carry the period: got %q", query.Get("period"))
		}
	})

	t.Run("when issuer is blank / then the label is the bare account", func(t *testing.T) {
		t.Parallel()

		authenticator := yatotp.NewAuthenticator(nil)

		raw, err := authenticator.ProvisioningURI(encodeSecret(t, rfcSeedSHA1), testAccount)
		if err != nil {
			t.Fatalf("building a provisioning uri should not fail: %v", err)
		}

		parsed, parseErr := url.Parse(string(raw))
		if parseErr != nil {
			t.Fatalf("the provisioning uri should parse: %v", parseErr)
		}

		if parsed.Path != "/"+string(testAccount) {
			t.Errorf("label should be the bare account: got %q", parsed.Path)
		}

		if parsed.Query().Has("issuer") {
			t.Error("a blank issuer should not appear in the query")
		}
	})

	t.Run("when account name is unusable / then building fails", func(t *testing.T) {
		t.Parallel()

		accounts := map[string]yatotp.AccountName{
			"empty":         "",
			"carries colon": "issuer:user@example.com",
		}

		authenticator := yatotp.NewAuthenticator(nil)

		for name, account := range accounts {
			if _, err := authenticator.ProvisioningURI(
				encodeSecret(t, rfcSeedSHA1),
				account,
			); err == nil {
				t.Errorf("%s account name should fail", name)
			}
		}
	})

	t.Run("when secret is unusable / then building fails", func(t *testing.T) {
		t.Parallel()

		const emptySecret = yatotp.Secret("")

		authenticator := yatotp.NewAuthenticator(nil)

		if _, err := authenticator.ProvisioningURI(emptySecret, testAccount); err == nil {
			t.Fatal("an empty secret should fail")
		}
	})
}

func TestSecretValidate(t *testing.T) {
	t.Parallel()

	t.Run("when secret is well formed / then validation passes", func(t *testing.T) {
		t.Parallel()

		if err := encodeSecret(t, rfcSeedSHA1).Validate(); err != nil {
			t.Fatalf("a well formed secret should validate: %v", err)
		}
	})

	t.Run("when secret is lowercase padded and spaced / then it still decodes", func(t *testing.T) {
		t.Parallel()

		encoded := string(encodeSecret(t, rfcSeedSHA1))
		messy := yatotp.Secret(" " + strings.ToLower(encoded) + "== ")

		if err := messy.Validate(); err != nil {
			t.Fatalf("a normalizable secret should validate: %v", err)
		}
	})

	t.Run("when secret is unusable / then validation reports a bad request", func(t *testing.T) {
		t.Parallel()

		const wantCode = http.StatusBadRequest

		secrets := map[string]yatotp.Secret{
			"empty":          "",
			"invalid base32": "0189!!!!",
			"too short":      "GEZDGNBV",
		}

		for name, secret := range secrets {
			err := secret.Validate()
			if err == nil {
				t.Errorf("%s secret should fail validation", name)

				continue
			}

			if err.Code() != wantCode {
				t.Errorf(
					"%s secret should report a bad request: got %d, want %d",
					name,
					err.Code(),
					wantCode,
				)
			}
		}
	})
}

func TestNewSecret(t *testing.T) {
	t.Parallel()

	t.Run("when minted / then it validates and is unique", func(t *testing.T) {
		t.Parallel()

		first, err := yatotp.NewSecret()
		if err != nil {
			t.Fatalf("minting a secret should not fail: %v", err)
		}

		if validateErr := first.Validate(); validateErr != nil {
			t.Fatalf("a minted secret should validate: %v", validateErr)
		}

		second, err := yatotp.NewSecret()
		if err != nil {
			t.Fatalf("minting a second secret should not fail: %v", err)
		}

		if first == second {
			t.Fatal("two minted secrets should differ")
		}
	})

	t.Run("when size is below the minimum / then minting fails", func(t *testing.T) {
		t.Parallel()

		const tooSmall = 8

		if _, err := yatotp.NewSecretWithSize(tooSmall); err == nil {
			t.Fatal("a size below the rfc minimum should be refused")
		}
	})

	t.Run("when size is explicit / then the decoded secret matches it", func(t *testing.T) {
		t.Parallel()

		const size = 32

		secret, err := yatotp.NewSecretWithSize(size)
		if err != nil {
			t.Fatalf("minting an explicit-size secret should not fail: %v", err)
		}

		decoded, decodeErr := base32.StdEncoding.WithPadding(base32.NoPadding).
			DecodeString(string(secret))
		if decodeErr != nil {
			t.Fatalf("a minted secret should decode: %v", decodeErr)
		}

		if len(decoded) != size {
			t.Errorf("decoded secret length should match: got %d, want %d", len(decoded), size)
		}
	})
}

func TestRecoveryCodes(t *testing.T) {
	t.Parallel()

	t.Run("when minted / then the count matches and codes are unique", func(t *testing.T) {
		t.Parallel()

		codes, err := yatotp.NewRecoveryCodes(yatotp.DefaultRecoveryCodeCount)
		if err != nil {
			t.Fatalf("minting recovery codes should not fail: %v", err)
		}

		if len(codes) != int(yatotp.DefaultRecoveryCodeCount) {
			t.Fatalf(
				"minted count should match: got %d, want %d",
				len(codes),
				yatotp.DefaultRecoveryCodeCount,
			)
		}

		seen := make(map[yatotp.RecoveryCode]struct{}, len(codes))
		for _, code := range codes {
			if code == "" {
				t.Error("a minted recovery code should not be empty")
			}

			if _, duplicate := seen[code]; duplicate {
				t.Errorf("recovery codes should be unique: %q repeated", code)
			}

			seen[code] = struct{}{}
		}
	})

	t.Run("when count is out of range / then minting fails", func(t *testing.T) {
		t.Parallel()

		const tooMany = yatotp.RecoveryCodeCount(200)

		if _, err := yatotp.NewRecoveryCodes(0); err == nil {
			t.Error("a zero count should be refused")
		}

		if _, err := yatotp.NewRecoveryCodes(tooMany); err == nil {
			t.Error("an oversized count should be refused")
		}
	})

	t.Run("when compared / then only an identical code matches", func(t *testing.T) {
		t.Parallel()

		const (
			code  = yatotp.RecoveryCode("ABCDEFGHIJKLMNOP")
			other = yatotp.RecoveryCode("ABCDEFGHIJKLMNOQ")
			short = yatotp.RecoveryCode("ABCD")
		)

		if !code.Matches(code) {
			t.Error("an identical code should match")
		}

		if code.Matches(other) {
			t.Error("a differing code should not match")
		}

		if code.Matches(short) {
			t.Error("a differing-length code should not match")
		}
	})
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	t.Run("when config is default / then validation passes", func(t *testing.T) {
		t.Parallel()

		config := yatotp.Config{
			Issuer:    testIssuer,
			Algorithm: yatotp.AlgorithmSHA1,
			Digits:    yatotp.DefaultDigits,
			Period:    yatotp.DefaultPeriod,
			Skew:      yatotp.DefaultSkew,
		}

		if err := config.Validate(); err != nil {
			t.Fatalf("a default config should validate: %v", err)
		}
	})

	t.Run("when a field is out of range / then validation fails", func(t *testing.T) {
		t.Parallel()

		base := yatotp.Config{
			Issuer:    testIssuer,
			Algorithm: yatotp.AlgorithmSHA1,
			Digits:    yatotp.DefaultDigits,
			Period:    yatotp.DefaultPeriod,
			Skew:      yatotp.DefaultSkew,
		}

		broken := map[string]func(config *yatotp.Config){
			"issuer with colon": func(config *yatotp.Config) { config.Issuer = "bad:issuer" },
			"unknown algorithm": func(config *yatotp.Config) { config.Algorithm = "MD5" },
			"digits too few":    func(config *yatotp.Config) { config.Digits = 4 },
			"digits too many":   func(config *yatotp.Config) { config.Digits = 9 },
			"zero period":       func(config *yatotp.Config) { config.Period = 0 },
			"skew too wide":     func(config *yatotp.Config) { config.Skew = yatotp.MaxSkew + 1 },
		}

		for name, breakField := range broken {
			config := base
			breakField(&config)

			if err := config.Validate(); err == nil {
				t.Errorf("%s should fail validation", name)
			}
		}
	})
}

func TestNewAuthenticatorDefaults(t *testing.T) {
	t.Parallel()

	const (
		unix     = int64(1234567890)
		wantSize = 6
	)

	authenticator := yatotp.NewAuthenticator(nil)

	code, err := authenticator.Generate(encodeSecret(t, rfcSeedSHA1), time.Unix(unix, 0).UTC())
	if err != nil {
		t.Fatalf("a nil config should still generate: %v", err)
	}

	if len(code) != wantSize {
		t.Errorf("default digits should be six: got %d, want %d", len(code), wantSize)
	}
}

func TestRedaction(t *testing.T) {
	t.Parallel()

	const (
		wantRedacted = "[REDACTED]"

		secret = yatotp.Secret("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ")
		code   = yatotp.Code("123456")
		backup = yatotp.RecoveryCode("ABCDEFGHIJ")
		uri    = yatotp.ProvisioningURI("otpauth://totp/Example:user?secret=X")
	)

	if got := secret.LogString(); got != wantRedacted {
		t.Errorf("secret should redact: got %q", got)
	}

	if got := code.LogString(); got != wantRedacted {
		t.Errorf("code should redact: got %q", got)
	}

	if got := backup.LogString(); got != wantRedacted {
		t.Errorf("recovery code should redact: got %q", got)
	}

	if got := uri.LogString(); got != wantRedacted {
		t.Errorf("provisioning uri should redact: got %q", got)
	}
}
