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
	contentTypeMixed        = "multipart/mixed; boundary=\"%s\""
	contentTypeOctetStream  = "application/octet-stream"
	contentTypeAttachment   = "%s; name=\"%s\""
	contentDisposition      = "attachment; filename=\"%s\""
	contentTransferEncoding = "8bit"
	contentTransferBase64   = "base64"
	crlf                    = "\r\n"
	boundaryRandomBytes     = 16
	base64LineLength        = 76
)
