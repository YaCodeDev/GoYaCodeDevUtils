package yatotp_test

import (
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatotp"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestAuthenticatorModule(t *testing.T) {
	t.Parallel()

	t.Run("when Module is wired / then it resolves a usable *Authenticator", func(t *testing.T) {
		t.Parallel()

		const unix = int64(1234567890)

		var authenticator *yatotp.Authenticator

		fxtest.New(
			t,
			yatotp.Module,
			fx.Supply(&yatotp.Config{Issuer: testIssuer}),
			fx.Populate(&authenticator),
		)

		if authenticator == nil {
			t.Fatal("Module should populate a non-nil *Authenticator")
		}

		secret := encodeSecret(t, rfcSeedSHA1)
		at := time.Unix(unix, 0).UTC()

		code, err := authenticator.Generate(secret, at)
		if err != nil {
			t.Fatalf("the wired authenticator should generate: %v", err)
		}

		if _, verified, validateErr := authenticator.Validate(
			secret,
			code,
			at,
		); validateErr != nil ||
			!verified {
			t.Fatalf(
				"the wired authenticator should verify its own code: verified=%v err=%v",
				verified,
				validateErr,
			)
		}
	})
}
