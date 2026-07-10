package yasmtp

import "errors"

var (
	ErrConfigHostRequired = errors.New("[SMTP] config host is required")
	ErrConfigPortRequired = errors.New("[SMTP] config port is required")
	ErrConfigFromRequired = errors.New("[SMTP] config from is required")
	ErrConfigFromInvalid  = errors.New("[SMTP] config from is not a valid address")
	ErrRecipientInvalid   = errors.New("[SMTP] recipient is not a valid address")
	ErrNoRecipients       = errors.New("[SMTP] message has no recipients")
	ErrNoBody             = errors.New("[SMTP] message has neither text nor html body")
	ErrDial               = errors.New("[SMTP] failed to dial relay")
	ErrStartTLS           = errors.New("[SMTP] failed to start tls")
	ErrAuth               = errors.New("[SMTP] failed to authenticate")
	ErrNoop               = errors.New("[SMTP] failed noop health check")
	ErrMailFrom           = errors.New("[SMTP] failed `MAIL FROM`")
	ErrRcptTo             = errors.New("[SMTP] failed `RCPT TO`")
	ErrData               = errors.New("[SMTP] failed `DATA`")
	ErrWriteBody          = errors.New("[SMTP] failed to write message body")
	ErrCloseWriter        = errors.New("[SMTP] failed to close data writer")
	ErrCloseClient        = errors.New("[SMTP] failed to close client")
	ErrTemplateExecute    = errors.New("[SMTP] failed to execute template")
	ErrGiveUpAfterRetry   = errors.New("[SMTP] gave up after exhausting retry attempts")
	ErrBuildBoundaryRand  = errors.New("[SMTP] failed to generate multipart boundary")
)
