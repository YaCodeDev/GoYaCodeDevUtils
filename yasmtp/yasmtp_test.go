package yasmtp_test

import (
	"context"
	"crypto/tls"
	"html/template"
	"strings"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yasmtp"
)

const (
	testFrom             = "noreply@example.com"
	testTo               = "user@example.com"
	testSubject          = "Test subject"
	testText             = "Test body text"
	testHTML             = "<p>Test body html</p>"
	testMaliciousSubject = "hi\r\nBcc: attacker@evil.com"
	testTemplateCode     = "123456"
	testUnroutablePort   = 1
	wantHeaderLineCount  = 7
)

type templateData struct {
	Code string
}

func testLogger() yalogger.Logger {
	return yalogger.NewBaseLogger(nil).NewLogger()
}

func newTestMailer(t *testing.T, server *fakeSMTPServer) *yasmtp.Mailer {
	t.Helper()

	return yasmtp.NewMailer(&yasmtp.Config{
		Host: server.Host(),
		Port: server.Port(t),
		From: testFrom,
		TLSConfig: &tls.Config{
			RootCAs:    server.CertPool(),
			ServerName: testServerHost,
			MinVersion: tls.VersionTLS12,
		},
	}, testLogger())
}

func TestMailer_Send(t *testing.T) {
	t.Parallel()

	t.Run("when message has text body / then it is delivered", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTPServer(t)
		mailer := newTestMailer(t, server)

		err := mailer.Send(context.Background(), yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		mails := server.Mails()
		if len(mails) != 1 {
			t.Fatalf("expected 1 delivered mail, got %d", len(mails))
		}

		if mails[0].From != testFrom {
			t.Errorf("expected from %q, got %q", testFrom, mails[0].From)
		}

		if len(mails[0].To) != 1 || mails[0].To[0] != testTo {
			t.Errorf("expected to %q, got %v", testTo, mails[0].To)
		}

		if !strings.Contains(mails[0].Data, testText) {
			t.Errorf("expected body to contain %q, got %q", testText, mails[0].Data)
		}
	})

	t.Run("when message has html and text body / then multipart is delivered", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTPServer(t)
		mailer := newTestMailer(t, server)

		err := mailer.Send(context.Background(), yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
			HTML:    testHTML,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		mails := server.Mails()
		if len(mails) != 1 {
			t.Fatalf("expected 1 delivered mail, got %d", len(mails))
		}

		if !strings.Contains(mails[0].Data, testText) {
			t.Errorf("expected multipart body to contain text part, got %q", mails[0].Data)
		}

		if !strings.Contains(mails[0].Data, testHTML) {
			t.Errorf("expected multipart body to contain html part, got %q", mails[0].Data)
		}
	})

	t.Run("when connection is reused across sends / then both sends succeed", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTPServer(t)
		mailer := newTestMailer(t, server)
		ctx := context.Background()

		message := yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
		}

		if err := mailer.Send(ctx, message); err != nil {
			t.Fatalf("first send failed: %v", err)
		}

		if err := mailer.Send(ctx, message); err != nil {
			t.Fatalf("second send failed: %v", err)
		}

		if len(server.Mails()) != 2 {
			t.Fatalf("expected 2 delivered mails, got %d", len(server.Mails()))
		}
	})

	t.Run("when subject contains crlf / then header injection is neutralized", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTPServer(t)
		mailer := newTestMailer(t, server)

		err := mailer.Send(context.Background(), yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testMaliciousSubject,
			Text:    testText,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		mails := server.Mails()
		if len(mails) != 1 {
			t.Fatalf("expected 1 delivered mail, got %d", len(mails))
		}

		if strings.Contains(mails[0].Data, "\r\nBcc:") {
			t.Errorf("expected no separate injected Bcc header line, got %q", mails[0].Data)
		}

		headerLines := strings.Split(strings.SplitN(mails[0].Data, "\r\n\r\n", 2)[0], "\r\n")
		if len(headerLines) != wantHeaderLineCount {
			t.Errorf(
				"expected exactly %d header lines (no injected extra header), got %d: %q",
				wantHeaderLineCount,
				len(headerLines),
				headerLines,
			)
		}
	})

	t.Run("when message has no recipients / then it fails without dialing", func(t *testing.T) {
		t.Parallel()

		mailer := yasmtp.NewMailer(&yasmtp.Config{From: testFrom}, testLogger())

		err := mailer.Send(
			context.Background(),
			yasmtp.Message{Subject: testSubject, Text: testText},
		)
		if err == nil {
			t.Fatal("expected error for missing recipients, got nil")
		}
	})

	t.Run("when message has no body / then it fails without dialing", func(t *testing.T) {
		t.Parallel()

		mailer := yasmtp.NewMailer(&yasmtp.Config{From: testFrom}, testLogger())

		err := mailer.Send(
			context.Background(),
			yasmtp.Message{To: []yasmtp.Recipient{testTo}, Subject: testSubject},
		)
		if err == nil {
			t.Fatal("expected error for missing body, got nil")
		}
	})

	t.Run(
		"when context is already canceled / then retries stop immediately with an error",
		func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			mailer := yasmtp.NewMailer(&yasmtp.Config{
				Host: testServerHost,
				Port: testUnroutablePort,
				From: testFrom,
			}, testLogger())

			err := mailer.Send(
				ctx,
				yasmtp.Message{
					To:      []yasmtp.Recipient{testTo},
					Subject: testSubject,
					Text:    testText,
				},
			)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		},
	)
}

func TestMailer_SendTemplate(t *testing.T) {
	t.Parallel()

	t.Run("when template renders / then html body is delivered", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTPServer(t)
		mailer := newTestMailer(t, server)

		tmpl := template.Must(template.New("code").Parse("Your code: {{.Code}}"))

		err := mailer.SendTemplate(
			context.Background(),
			[]yasmtp.Recipient{testTo},
			testSubject,
			tmpl,
			templateData{Code: testTemplateCode},
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		mails := server.Mails()
		if len(mails) != 1 {
			t.Fatalf("expected 1 delivered mail, got %d", len(mails))
		}

		if !strings.Contains(mails[0].Data, "Your code: "+testTemplateCode) {
			t.Errorf("expected rendered body, got %q", mails[0].Data)
		}
	})
}

func TestMailer_Close(t *testing.T) {
	t.Parallel()

	t.Run("when never connected / then close is a no-op", func(t *testing.T) {
		t.Parallel()

		mailer := yasmtp.NewMailer(&yasmtp.Config{From: testFrom}, testLogger())

		if err := mailer.Close(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("when connected / then close releases the connection", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTPServer(t)
		mailer := newTestMailer(t, server)

		err := mailer.Send(context.Background(), yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
		})
		if err != nil {
			t.Fatalf("send failed: %v", err)
		}

		if err := mailer.Close(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}
