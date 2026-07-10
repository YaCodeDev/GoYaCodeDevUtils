package yasmtp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

func sanitizeHeaderValue(value string) string {
	sanitized := strings.ReplaceAll(value, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\n", "")

	return sanitized
}

func generateBoundary() (string, yaerrors.Error) {
	raw := make([]byte, boundaryRandomBytes)

	if _, err := rand.Read(raw); err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrBuildBoundaryRand),
			logTag+" failed to build multipart boundary",
		)
	}

	return hex.EncodeToString(raw), nil
}

func buildMessage(from From, message Message) ([]byte, yaerrors.Error) {
	sanitizedFrom := sanitizeHeaderValue(string(from))
	sanitizedSubject := sanitizeHeaderValue(string(message.Subject))

	sanitizedRecipients := make([]string, 0, len(message.To))
	for _, recipient := range message.To {
		sanitizedRecipients = append(sanitizedRecipients, sanitizeHeaderValue(string(recipient)))
	}

	var builder strings.Builder

	writeHeader(&builder, "From", sanitizedFrom)
	writeHeader(&builder, "To", strings.Join(sanitizedRecipients, ", "))
	writeHeader(&builder, "Subject", sanitizedSubject)
	writeHeader(&builder, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&builder, "MIME-Version", mimeVersion)

	switch {
	case message.HTML != "" && message.Text != "":
		boundary, err := generateBoundary()
		if err != nil {
			return nil, err.Wrap(logTag + " failed to build multipart message")
		}

		writeHeader(&builder, "Content-Type", fmt.Sprintf(contentTypeMultipart, boundary))
		builder.WriteString(crlf)
		writeBoundaryPart(&builder, boundary, contentTypeText, string(message.Text))
		writeBoundaryPart(&builder, boundary, contentTypeHTML, string(message.HTML))
		builder.WriteString("--")
		builder.WriteString(boundary)
		builder.WriteString("--")
		builder.WriteString(crlf)
	case message.HTML != "":
		writeBody(&builder, contentTypeHTML, string(message.HTML))
	default:
		writeBody(&builder, contentTypeText, string(message.Text))
	}

	return []byte(builder.String()), nil
}

func writeHeader(builder *strings.Builder, name string, value string) {
	builder.WriteString(name)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteString(crlf)
}

func writeBoundaryPart(
	builder *strings.Builder,
	boundary string,
	contentType string,
	content string,
) {
	builder.WriteString("--")
	builder.WriteString(boundary)
	builder.WriteString(crlf)
	writeHeader(builder, "Content-Type", contentType)
	writeHeader(builder, "Content-Transfer-Encoding", contentTransferEncoding)
	builder.WriteString(crlf)
	builder.WriteString(content)
	builder.WriteString(crlf)
	builder.WriteString(crlf)
}

func writeBody(builder *strings.Builder, contentType string, content string) {
	writeHeader(builder, "Content-Type", contentType)
	writeHeader(builder, "Content-Transfer-Encoding", contentTransferEncoding)
	builder.WriteString(crlf)
	builder.WriteString(content)
}
