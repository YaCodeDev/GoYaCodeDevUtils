package yaturnstile_test

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaturnstile"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestVerifierModule(t *testing.T) {
	t.Parallel()

	t.Run("when Module is wired / then it resolves a usable Verifier", func(t *testing.T) {
		t.Parallel()

		var verifier yaturnstile.Verifier

		fxtest.New(
			t,
			yaturnstile.Module,
			yalogger.LoggerModule,
			fx.Supply((*yalogger.Config)(nil)),
			fx.Supply(&yaturnstile.Config{
				Endpoint: yaturnstile.DefaultEndpoint,
				Disabled: true,
			}),
			fx.Populate(&verifier),
		)

		if verifier == nil {
			t.Fatal("Module should populate a non-nil Verifier")
		}

		verified, err := verifier.Verify(context.Background(), testToken, testRemoteIP, nil)
		if err != nil {
			t.Fatalf("the wired verifier should not fail when disabled: %v", err)
		}

		if !verified {
			t.Fatal("a disabled wired verifier should pass")
		}
	})
}
