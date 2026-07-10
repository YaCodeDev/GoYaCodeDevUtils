package yasmtp_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yasmtp"
)

const testInvalidRecipient = "not-an-address"

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	t.Run("when config is complete / then it is valid", func(t *testing.T) {
		t.Parallel()

		config := yasmtp.Config{Host: testServerHost, Port: 587, From: testFrom}

		if err := config.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("when host is empty / then it is invalid", func(t *testing.T) {
		t.Parallel()

		config := yasmtp.Config{Port: 587, From: testFrom}

		if err := config.Validate(); err == nil {
			t.Fatal("expected error for missing host, got nil")
		}
	})

	t.Run("when port is zero / then it is invalid", func(t *testing.T) {
		t.Parallel()

		config := yasmtp.Config{Host: testServerHost, From: testFrom}

		if err := config.Validate(); err == nil {
			t.Fatal("expected error for missing port, got nil")
		}
	})

	t.Run("when from is empty / then it is invalid", func(t *testing.T) {
		t.Parallel()

		config := yasmtp.Config{Host: testServerHost, Port: 587}

		if err := config.Validate(); err == nil {
			t.Fatal("expected error for missing from, got nil")
		}
	})

	t.Run("when from is not a valid address / then it is invalid", func(t *testing.T) {
		t.Parallel()

		config := yasmtp.Config{Host: testServerHost, Port: 587, From: testInvalidRecipient}

		if err := config.Validate(); err == nil {
			t.Fatal("expected error for invalid from, got nil")
		}
	})
}

func TestMessage_Validate(t *testing.T) {
	t.Parallel()

	t.Run("when message is complete / then it is valid", func(t *testing.T) {
		t.Parallel()

		message := yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
		}

		if err := message.Validate(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("when message has no recipients / then it is invalid", func(t *testing.T) {
		t.Parallel()

		message := yasmtp.Message{Subject: testSubject, Text: testText}

		if err := message.Validate(); err == nil {
			t.Fatal("expected error for missing recipients, got nil")
		}
	})

	t.Run("when a recipient is not a valid address / then it is invalid", func(t *testing.T) {
		t.Parallel()

		message := yasmtp.Message{
			To:      []yasmtp.Recipient{testInvalidRecipient},
			Subject: testSubject,
			Text:    testText,
		}

		if err := message.Validate(); err == nil {
			t.Fatal("expected error for invalid recipient, got nil")
		}
	})

	t.Run("when message has no body / then it is invalid", func(t *testing.T) {
		t.Parallel()

		message := yasmtp.Message{To: []yasmtp.Recipient{testTo}, Subject: testSubject}

		if err := message.Validate(); err == nil {
			t.Fatal("expected error for missing body, got nil")
		}
	})
}

func TestPassword_LogString(t *testing.T) {
	t.Parallel()

	t.Run("when password is logged / then it is redacted", func(t *testing.T) {
		t.Parallel()

		password := yasmtp.Password("super-secret")

		if got := password.LogString(); got != "[REDACTED]" {
			t.Fatalf("expected redacted password, got %q", got)
		}
	})
}
