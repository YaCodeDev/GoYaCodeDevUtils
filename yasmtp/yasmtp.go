// Package yasmtp sends email through a plain SMTP relay (STARTTLS + PLAIN
// auth), with connection reuse, bounded retry via yabackoff, and optional
// html/template rendering.
//
// Config is loadable via config.LoadConfigStructFromEnv and, once populated,
// checked with Config.Validate before it is handed to NewMailer.
//
// # Quick start
//
//	cfg := yasmtp.Config{
//	    Host:     "mail.example.com",
//	    Port:     587,
//	    Username: "noreply@example.com",
//	    Password: "secret",
//	    From:     "noreply@example.com",
//	}
//	if err := cfg.Validate(); err != nil {
//	    // handle error
//	}
//
//	mailer := yasmtp.NewMailer(&cfg, log)
//	defer mailer.Close()
//
//	err := mailer.Send(ctx, yasmtp.Message{
//	    To:      []yasmtp.Recipient{"user@example.com"},
//	    Subject: "Your code",
//	    Text:    "Your verification code is 123456",
//	})
package yasmtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabackoff"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

// Mailer sends Message values through a single, lazily-dialed, reused SMTP
// connection guarded by a mutex. A failed send closes and clears the
// connection so the next call redials.
type Mailer struct {
	config Config
	log    yalogger.Logger
	client *smtp.Client
	mu     sync.Mutex
}

// NewMailer builds a Mailer from a Config. The connection is not opened
// until the first Send call. config is copied once at construction, so the
// caller may reuse or discard the pointer afterward.
//
// Example:
//
//	mailer := yasmtp.NewMailer(cfg, log)
func NewMailer(config *Config, log yalogger.Logger) *Mailer {
	if log == nil {
		log = yalogger.NewBaseLogger(nil).NewLogger()
	}

	mailer := &Mailer{
		log: log,
	}

	if config != nil {
		mailer.config = *config
	}

	return mailer
}

// Send delivers message, retrying transient failures with an exponential
// backoff (yabackoff) up to DefaultMaxAttempts times. A stale or broken
// connection is transparently redialed.
//
// Example:
//
//	err := mailer.Send(ctx, yasmtp.Message{To: []yasmtp.Recipient{"a@b.com"}, Subject: "hi", Text: "hi"})
func (m *Mailer) Send(ctx context.Context, message Message) (err yaerrors.Error) {
	if validateErr := message.Validate(); validateErr != nil {
		return validateErr
	}

	body, buildErr := buildMessage(m.config.From, message)
	if buildErr != nil {
		return buildErr.Wrap(logTag + " failed to build message")
	}

	retryBackoff := yabackoff.NewExponential(
		DefaultRetryInitialInterval,
		DefaultRetryMultiplier,
		DefaultRetryMaxInterval,
		0,
	)

	for attempt := 1; attempt <= DefaultMaxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return yaerrors.FromError(
					http.StatusRequestTimeout,
					ctx.Err(),
					logTag+" context canceled while retrying send",
				)
			case <-time.After(retryBackoff.Next()):
			}
		}

		err = m.sendOnce(ctx, message.To, body)
		if err == nil {
			return nil
		}

		m.log.
			WithField("attempt", attempt).
			Warnf(logTag+" send attempt failed: %v", err)
	}

	return err.Wrap(
		fmt.Sprintf("%s %v after %d attempts", logTag, ErrGiveUpAfterRetry, DefaultMaxAttempts),
	)
}

// SendTemplate renders tmpl with data into an HTML body and sends it.
//
// Example:
//
//	tmpl := template.Must(template.New("code").Parse("Your code: {{.Code}}"))
//	err := mailer.SendTemplate(ctx, []yasmtp.Recipient{"a@b.com"}, "Your code", tmpl, struct{ Code string }{"123456"})
func (m *Mailer) SendTemplate(
	ctx context.Context,
	to []Recipient,
	subject Subject,
	tmpl *template.Template,
	data any,
) (err yaerrors.Error) {
	var rendered strings.Builder

	if execErr := tmpl.Execute(&rendered, data); execErr != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(execErr, ErrTemplateExecute),
			logTag+" failed to render template",
		)
	}

	return m.Send(ctx, Message{
		To:      to,
		Subject: subject,
		HTML:    Body(rendered.String()),
	})
}

// Close releases the pooled connection, if any. Safe to call even if no
// connection was ever opened.
//
// Example:
//
//	defer mailer.Close()
func (m *Mailer) Close() (err yaerrors.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return nil
	}

	closeErr := m.client.Close()
	m.client = nil

	if closeErr != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(closeErr, ErrCloseClient),
			logTag+" failed to close",
		)
	}

	return nil
}

func (m *Mailer) sendOnce(
	ctx context.Context,
	recipients []Recipient,
	body []byte,
) (err yaerrors.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil && m.client.Noop() != nil {
		m.closeClientLocked()
	}

	if m.client == nil {
		client, dialErr := m.dial(ctx)
		if dialErr != nil {
			return dialErr
		}

		m.client = client
	}

	if err = m.transmit(recipients, body); err != nil {
		m.closeClientLocked()

		return err
	}

	return nil
}

func (m *Mailer) dial(ctx context.Context) (client *smtp.Client, err yaerrors.Error) {
	host := string(m.config.Host)
	addr := net.JoinHostPort(host, strconv.Itoa(int(m.config.Port)))

	dialer := net.Dialer{Timeout: DefaultDialTimeout}

	conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
	if dialErr != nil {
		return nil, yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(dialErr, ErrDial),
			logTag+" failed to dial "+addr,
		)
	}

	newClient, clientErr := smtp.NewClient(conn, host)
	if clientErr != nil {
		_ = conn.Close()

		return nil, yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(clientErr, ErrDial),
			logTag+" failed to init client",
		)
	}

	tlsConfig := m.config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	}

	if tlsErr := newClient.StartTLS(tlsConfig); tlsErr != nil {
		_ = newClient.Close()

		return nil, yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(tlsErr, ErrStartTLS),
			logTag+" failed starttls",
		)
	}

	if m.config.Username != "" {
		auth := smtp.PlainAuth(
			"",
			string(m.config.Username),
			string(m.config.Password),
			host,
		)

		if authErr := newClient.Auth(auth); authErr != nil {
			_ = newClient.Close()

			return nil, yaerrors.FromError(
				http.StatusUnauthorized,
				errors.Join(authErr, ErrAuth),
				logTag+" failed auth",
			)
		}
	}

	m.log.Infof(logTag+" connected to %s", addr)

	return newClient, nil
}

func (m *Mailer) transmit(recipients []Recipient, body []byte) (err yaerrors.Error) {
	if mailErr := m.client.Mail(string(m.config.From)); mailErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(mailErr, ErrMailFrom),
			logTag+" failed mail from",
		)
	}

	for _, recipient := range recipients {
		if rcptErr := m.client.Rcpt(string(recipient)); rcptErr != nil {
			return yaerrors.FromError(
				http.StatusBadGateway,
				errors.Join(rcptErr, ErrRcptTo),
				logTag+" failed rcpt to "+string(recipient),
			)
		}
	}

	writer, dataErr := m.client.Data()
	if dataErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(dataErr, ErrData),
			logTag+" failed data",
		)
	}

	if _, writeErr := writer.Write(body); writeErr != nil {
		_ = writer.Close()

		return yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(writeErr, ErrWriteBody),
			logTag+" failed write body",
		)
	}

	if closeErr := writer.Close(); closeErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(closeErr, ErrCloseWriter),
			logTag+" failed close writer",
		)
	}

	if resetErr := m.client.Reset(); resetErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			errors.Join(resetErr, ErrCloseClient),
			logTag+" failed reset session",
		)
	}

	return nil
}

func (m *Mailer) closeClientLocked() {
	if m.client == nil {
		return
	}

	if closeErr := m.client.Close(); closeErr != nil {
		m.log.Warnf(logTag+" failed to close stale client: %v", closeErr)
	}

	m.client = nil
}
