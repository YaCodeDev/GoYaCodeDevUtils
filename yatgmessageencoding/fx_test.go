package yatgmessageencoding_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgmessageencoding"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestMessageEncodingModule(t *testing.T) {
	t.Parallel()

	t.Run(
		"when MessageEncodingModule is wired / then it resolves a usable MessageEncoding",
		func(t *testing.T) {
			t.Parallel()

			var encoding yatgmessageencoding.MessageEncoding

			fxtest.New(
				t,
				yatgmessageencoding.MessageEncodingModule,
				fx.Populate(&encoding),
			)

			if encoding == nil {
				t.Fatalf("expected MessageEncodingModule to populate a non-nil MessageEncoding")
			}
		},
	)
}
