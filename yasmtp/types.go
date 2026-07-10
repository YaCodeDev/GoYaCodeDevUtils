package yasmtp

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/mail"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Host is the SMTP relay hostname a Mailer dials.
type Host string

// Validate reports whether host is non-empty.
func (host Host) Validate() yaerrors.Error {
	if host == "" {
		return yaerrors.FromError(http.StatusBadRequest, ErrConfigHostRequired, logTag+" config")
	}

	return nil
}

// Port is the SMTP relay TCP port a Mailer dials.
type Port uint16

// Validate reports whether port is non-zero.
func (port Port) Validate() yaerrors.Error {
	if port == 0 {
		return yaerrors.FromError(http.StatusBadRequest, ErrConfigPortRequired, logTag+" config")
	}

	return nil
}

// Username authenticates to the SMTP relay via PLAIN auth. Empty skips auth.
type Username string

// Password authenticates to the SMTP relay via PLAIN auth, alongside Username.
type Password string

// LogString redacts Password from log output.
func (Password) LogString() string {
	return redactedValue
}

// From is the envelope and header sender address for every Message sent
// through a Mailer.
type From string

// Validate reports whether from is a non-empty, RFC 5322 parseable address.
func (from From) Validate() yaerrors.Error {
	if from == "" {
		return yaerrors.FromError(http.StatusBadRequest, ErrConfigFromRequired, logTag+" config")
	}

	if _, err := mail.ParseAddress(string(from)); err != nil {
		return yaerrors.FromError(
			http.StatusBadRequest,
			errors.Join(err, ErrConfigFromInvalid),
			logTag+" config",
		)
	}

	return nil
}

// Config configures a Mailer's SMTP relay connection. It is loadable via
// config.LoadConfigStructFromEnv (SMTP_HOST, SMTP_PORT, SMTP_USERNAME,
// SMTP_PASSWORD, SMTP_FROM under whatever key path the caller nests it at).
//
// Host and From have no sensible universal default and are required — every
// other field is deployment-specific but optional.
type Config struct {
	Host     Host     `default:""`
	Port     Port     `default:"587"`
	Username Username `default:""`
	Password Password `default:""`
	From     From     `default:""`

	// TLSConfig overrides the tls.Config used for STARTTLS. A nil value
	// derives a default from Host (ServerName, TLS 1.2 minimum, system root
	// CAs) — set this only to trust a private CA or a self-signed relay.
	TLSConfig *tls.Config
}

// Validate cascades validation across the required fields (Host, Port,
// From). Username and Password are optional and always valid.
func (config *Config) Validate() yaerrors.Error {
	if err := config.Host.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	if err := config.Port.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	if err := config.From.Validate(); err != nil {
		return err.Wrap(logTag + " invalid config")
	}

	return nil
}

// Recipient is a single message-destination email address.
type Recipient string

// Validate reports whether recipient is a non-empty, RFC 5322 parseable
// address.
func (recipient Recipient) Validate() yaerrors.Error {
	if recipient == "" {
		return yaerrors.FromError(http.StatusBadRequest, ErrRecipientInvalid, logTag+" message")
	}

	if _, err := mail.ParseAddress(string(recipient)); err != nil {
		return yaerrors.FromError(
			http.StatusBadRequest,
			errors.Join(err, ErrRecipientInvalid),
			logTag+" message",
		)
	}

	return nil
}

// Subject is an email's Subject header value.
type Subject string

// Body is an email's plain-text or HTML content.
type Body string

// Message is a single email to be sent through a Mailer.
//
// At least one of Text or HTML must be non-empty. When both are set, the
// message is sent as multipart/alternative with the plain-text part first.
type Message struct {
	To      []Recipient
	Subject Subject
	Text    Body
	HTML    Body
}

// Validate reports whether message has at least one valid recipient and a
// non-empty body.
func (message Message) Validate() yaerrors.Error {
	if len(message.To) == 0 {
		return yaerrors.FromError(http.StatusBadRequest, ErrNoRecipients, logTag+" cannot send")
	}

	for _, recipient := range message.To {
		if err := recipient.Validate(); err != nil {
			return err.Wrap(logTag + " cannot send")
		}
	}

	if message.Text == "" && message.HTML == "" {
		return yaerrors.FromError(http.StatusBadRequest, ErrNoBody, logTag+" cannot send")
	}

	return nil
}
