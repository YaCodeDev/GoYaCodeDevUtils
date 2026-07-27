package yaturnstile_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaturnstile"
)

const (
	testSecretKey = yaturnstile.SecretKey("test-secret-key")
	testToken     = yaturnstile.Token("test-token")
	testRemoteIP  = yaturnstile.RemoteIP("203.0.113.10")
)

func testLogger() yalogger.Logger {
	return yalogger.NewBaseLogger(nil).NewLogger()
}

func TestVerifierVerify(t *testing.T) {
	t.Parallel()

	t.Run(
		"when verification is disabled / then it passes without a request",
		func(t *testing.T) {
			t.Parallel()

			const unreachableEndpoint = yaturnstile.Endpoint("http://127.0.0.1:1")

			verifier := yaturnstile.NewVerifier(
				&yaturnstile.Config{Endpoint: unreachableEndpoint, Disabled: true},
				testLogger(),
			)

			verified, err := verifier.Verify(
				context.Background(),
				testToken,
				testRemoteIP,
				testLogger(),
			)
			if err != nil {
				t.Fatalf("a disabled verifier should not fail: %v", err)
			}

			if !verified {
				t.Fatal("a disabled verifier should pass every token")
			}
		},
	)

	t.Run(
		"when secret key is missing and verification is not disabled / then it fails closed",
		func(t *testing.T) {
			t.Parallel()

			const (
				unreachableEndpoint = yaturnstile.Endpoint("http://127.0.0.1:1")
				wantCode            = http.StatusInternalServerError
			)

			verifier := yaturnstile.NewVerifier(
				&yaturnstile.Config{Endpoint: unreachableEndpoint},
				testLogger(),
			)

			verified, err := verifier.Verify(
				context.Background(),
				testToken,
				testRemoteIP,
				testLogger(),
			)
			if err == nil {
				t.Fatal("a missing secret key should surface as a misconfiguration error")
			}

			if err.Code() != wantCode {
				t.Errorf(
					"error code should be an internal server error: got %d, want %d",
					err.Code(),
					wantCode,
				)
			}

			if verified {
				t.Error("a misconfigured verifier must not pass a token")
			}
		},
	)

	t.Run(
		"when siteverify answers success / then verification passes with the posted form",
		func(t *testing.T) {
			t.Parallel()

			const successBody = `{"success":true}`

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if formErr := r.ParseForm(); formErr != nil {
						t.Errorf("siteverify form should parse: %v", formErr)

						return
					}

					if r.FormValue("secret") != string(testSecretKey) {
						t.Errorf(
							"posted secret should be the configured key: got %q, want %q",
							r.FormValue("secret"),
							testSecretKey,
						)
					}

					if r.FormValue("response") != string(testToken) {
						t.Errorf(
							"posted response should be the client token: got %q, want %q",
							r.FormValue("response"),
							testToken,
						)
					}

					if r.FormValue("remoteip") != string(testRemoteIP) {
						t.Errorf(
							"posted remoteip should be the client address: got %q, want %q",
							r.FormValue("remoteip"),
							testRemoteIP,
						)
					}

					if _, writeErr := w.Write([]byte(successBody)); writeErr != nil {
						t.Errorf("siteverify stub should write its body: %v", writeErr)
					}
				}),
			)
			defer server.Close()

			verifier := yaturnstile.NewVerifier(&yaturnstile.Config{
				SecretKey: testSecretKey,
				Endpoint:  yaturnstile.Endpoint(server.URL),
			}, testLogger())

			verified, err := verifier.Verify(
				context.Background(),
				testToken,
				testRemoteIP,
				testLogger(),
			)
			if err != nil {
				t.Fatalf("successful siteverify should not fail: %v", err)
			}

			if !verified {
				t.Fatal("successful siteverify should pass the token")
			}
		},
	)

	t.Run(
		"when siteverify answers failure / then verification is rejected without an error",
		func(t *testing.T) {
			t.Parallel()

			const failureBody = `{"success":false,"error-codes":["invalid-input-response"]}`

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if _, writeErr := w.Write([]byte(failureBody)); writeErr != nil {
						t.Errorf("siteverify stub should write its body: %v", writeErr)
					}
				}),
			)
			defer server.Close()

			verifier := yaturnstile.NewVerifier(&yaturnstile.Config{
				SecretKey: testSecretKey,
				Endpoint:  yaturnstile.Endpoint(server.URL),
			}, testLogger())

			verified, err := verifier.Verify(
				context.Background(),
				testToken,
				testRemoteIP,
				testLogger(),
			)
			if err != nil {
				t.Fatalf("a rejected token should not surface as an error: %v", err)
			}

			if verified {
				t.Fatal("a rejected token should not pass")
			}
		},
	)

	t.Run(
		"when token is empty / then verification is rejected without a request",
		func(t *testing.T) {
			t.Parallel()

			const (
				emptyToken          = yaturnstile.Token("")
				unreachableEndpoint = yaturnstile.Endpoint("http://127.0.0.1:1")
			)

			verifier := yaturnstile.NewVerifier(&yaturnstile.Config{
				SecretKey: testSecretKey,
				Endpoint:  unreachableEndpoint,
			}, testLogger())

			verified, err := verifier.Verify(
				context.Background(),
				emptyToken,
				testRemoteIP,
				testLogger(),
			)
			if err != nil {
				t.Fatalf("a missing token should not surface as an error: %v", err)
			}

			if verified {
				t.Fatal("a missing token should not pass")
			}
		},
	)

	t.Run(
		"when siteverify answers a non-200 status / then verification returns a bad gateway",
		func(t *testing.T) {
			t.Parallel()

			const wantCode = http.StatusBadGateway

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}),
			)
			defer server.Close()

			verifier := yaturnstile.NewVerifier(&yaturnstile.Config{
				SecretKey: testSecretKey,
				Endpoint:  yaturnstile.Endpoint(server.URL),
			}, testLogger())

			verified, err := verifier.Verify(
				context.Background(),
				testToken,
				testRemoteIP,
				testLogger(),
			)
			if err == nil {
				t.Fatal("a non-200 siteverify answer should surface as an error")
			}

			if err.Code() != wantCode {
				t.Errorf(
					"error code should be a bad gateway: got %d, want %d",
					err.Code(),
					wantCode,
				)
			}

			if verified {
				t.Error("an errored verification should not pass")
			}
		},
	)

	t.Run(
		"when siteverify body exceeds the read cap / then decoding fails instead of streaming",
		func(t *testing.T) {
			t.Parallel()

			const wantCode = http.StatusBadGateway

			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					padding := make([]byte, yaturnstile.DefaultMaxResponseBytes*2)
					for index := range padding {
						padding[index] = 'a'
					}

					if _, writeErr := w.Write(
						[]byte(`{"success":true,"error-codes":["` + string(padding)),
					); writeErr != nil {
						t.Errorf("siteverify stub should write its body: %v", writeErr)
					}
				}),
			)
			defer server.Close()

			verifier := yaturnstile.NewVerifier(&yaturnstile.Config{
				SecretKey: testSecretKey,
				Endpoint:  yaturnstile.Endpoint(server.URL),
			}, testLogger())

			verified, err := verifier.Verify(
				context.Background(),
				testToken,
				testRemoteIP,
				testLogger(),
			)
			if err == nil {
				t.Fatal("a truncated oversized body should surface as a decode error")
			}

			if err.Code() != wantCode {
				t.Errorf(
					"error code should be a bad gateway: got %d, want %d",
					err.Code(),
					wantCode,
				)
			}

			if verified {
				t.Error("an errored verification should not pass")
			}
		},
	)

	t.Run(
		"when siteverify is unreachable / then verification returns an error",
		func(t *testing.T) {
			t.Parallel()

			const unreachableEndpoint = yaturnstile.Endpoint("http://127.0.0.1:1")

			verifier := yaturnstile.NewVerifier(&yaturnstile.Config{
				SecretKey: testSecretKey,
				Endpoint:  unreachableEndpoint,
			}, testLogger())

			verified, err := verifier.Verify(
				context.Background(),
				testToken,
				testRemoteIP,
				testLogger(),
			)
			if err == nil {
				t.Fatal("an unreachable siteverify endpoint should surface as an error")
			}

			if verified {
				t.Error("an errored verification should not pass")
			}
		},
	)

	t.Run(
		"when config and log are nil / then the verifier fails closed without panicking",
		func(t *testing.T) {
			t.Parallel()

			verifier := yaturnstile.NewVerifier(nil, nil)

			verified, err := verifier.Verify(context.Background(), testToken, testRemoteIP, nil)
			if err == nil {
				t.Fatal("a nil config should surface as a misconfiguration error")
			}

			if verified {
				t.Error("a nil config must not pass a token")
			}
		},
	)
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	t.Run("when endpoint is empty / then validation fails", func(t *testing.T) {
		t.Parallel()

		const wantCode = http.StatusBadRequest

		config := yaturnstile.Config{SecretKey: testSecretKey}

		err := config.Validate()
		if err == nil {
			t.Fatal("an empty endpoint should fail validation")
		}

		if err.Code() != wantCode {
			t.Errorf("error code should be a bad request: got %d, want %d", err.Code(), wantCode)
		}
	})

	t.Run("when endpoint is relative / then validation fails", func(t *testing.T) {
		t.Parallel()

		const relativeEndpoint = yaturnstile.Endpoint("/turnstile/v0/siteverify")

		config := yaturnstile.Config{SecretKey: testSecretKey, Endpoint: relativeEndpoint}

		if err := config.Validate(); err == nil {
			t.Fatal("a relative endpoint should fail validation")
		}
	})

	t.Run(
		"when secret key is missing and verification is not disabled / then validation fails",
		func(t *testing.T) {
			t.Parallel()

			const wantCode = http.StatusBadRequest

			config := yaturnstile.Config{Endpoint: yaturnstile.DefaultEndpoint}

			err := config.Validate()
			if err == nil {
				t.Fatal("a missing secret key should fail validation")
			}

			if err.Code() != wantCode {
				t.Errorf(
					"error code should be a bad request: got %d, want %d",
					err.Code(),
					wantCode,
				)
			}
		},
	)

	t.Run(
		"when secret key is missing but verification is disabled / then validation passes",
		func(t *testing.T) {
			t.Parallel()

			config := yaturnstile.Config{Endpoint: yaturnstile.DefaultEndpoint, Disabled: true}

			if err := config.Validate(); err != nil {
				t.Fatalf("an explicitly disabled config should validate: %v", err)
			}
		},
	)

	t.Run(
		"when secret key and default endpoint are set / then validation passes",
		func(t *testing.T) {
			t.Parallel()

			config := yaturnstile.Config{
				SecretKey: testSecretKey,
				Endpoint:  yaturnstile.DefaultEndpoint,
			}

			if err := config.Validate(); err != nil {
				t.Fatalf("a fully configured verifier should validate: %v", err)
			}
		},
	)
}

func TestSecretKeyRedaction(t *testing.T) {
	t.Parallel()

	const wantRedacted = "[REDACTED]"

	if got := testSecretKey.LogString(); got != wantRedacted {
		t.Errorf("secret key should redact: got %q, want %q", got, wantRedacted)
	}

	if got := testToken.LogString(); got != wantRedacted {
		t.Errorf("token should redact: got %q, want %q", got, wantRedacted)
	}
}

func TestSecretKeyConfigured(t *testing.T) {
	t.Parallel()

	const emptyKey = yaturnstile.SecretKey("")

	if emptyKey.Configured() {
		t.Error("an empty secret key should report as unconfigured")
	}

	if !testSecretKey.Configured() {
		t.Error("a non-empty secret key should report as configured")
	}
}
