package yasmtp_test

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yasmtp"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestMailerModule(t *testing.T) {
	t.Parallel()

	t.Run("when Module is wired / then it resolves a usable *Mailer", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTPServer(t)

		var mailer *yasmtp.Mailer

		fxtest.New(
			t,
			yasmtp.Module,
			yalogger.LoggerModule,
			fx.Supply((*yalogger.Config)(nil)),
			fx.Supply(&yasmtp.Config{
				Host: server.Host(),
				Port: server.Port(t),
				From: testFrom,
				TLSConfig: &tls.Config{
					RootCAs:    server.CertPool(),
					ServerName: testServerHost,
					MinVersion: tls.VersionTLS12,
				},
			}),
			fx.Populate(&mailer),
		)

		if mailer == nil {
			t.Fatalf("expected Module to populate a non-nil *Mailer")
		}

		err := mailer.Send(context.Background(), yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
		})
		if err != nil {
			t.Fatalf("expected send through the wired mailer to succeed, got %v", err)
		}

		if len(server.Mails()) != 1 {
			t.Fatalf("expected 1 delivered mail, got %d", len(server.Mails()))
		}
	})
}
