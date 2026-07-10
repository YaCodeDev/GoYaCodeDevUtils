package yasmtp

import "time"

const (
	DefaultDialTimeout = 10 * time.Second

	DefaultMaxAttempts          = 3
	DefaultRetryInitialInterval = 500 * time.Millisecond
	DefaultRetryMultiplier      = 2.0
	DefaultRetryMaxInterval     = 5 * time.Second

	logTag = "[SMTP]"

	redactedValue = "[REDACTED]"

	mimeVersion             = "1.0"
	contentTypeText         = "text/plain; charset=\"UTF-8\""
	contentTypeHTML         = "text/html; charset=\"UTF-8\""
	contentTypeMultipart    = "multipart/alternative; boundary=\"%s\""
	contentTransferEncoding = "8bit"
	crlf                    = "\r\n"
	boundaryRandomBytes     = 16
)
