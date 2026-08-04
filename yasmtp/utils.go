package yasmtp

import (
	"crypto/rand"
	"encoding/base64"
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

func sanitizeFilename(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(sanitizeHeaderValue(value), `"`, ""))
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

func buildMessage(from From, message *Message) ([]byte, yaerrors.Error) {
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

	if len(message.Attachments) == 0 {
		if err := writeStandaloneBody(&builder, message); err != nil {
			return nil, err.Wrap(logTag + " failed to build message body")
		}

		return []byte(builder.String()), nil
	}

	mixedBoundary, err := generateBoundary()
	if err != nil {
		return nil, err.Wrap(logTag + " failed to build mixed message")
	}

	writeHeader(&builder, "Content-Type", fmt.Sprintf(contentTypeMixed, mixedBoundary))
	builder.WriteString(crlf)
	writeBoundaryOpen(&builder, mixedBoundary)

	if bodyErr := writeNestedBody(&builder, message); bodyErr != nil {
		return nil, bodyErr.Wrap(logTag + " failed to build mixed message body")
	}

	for _, attachment := range message.Attachments {
		writeBoundaryOpen(&builder, mixedBoundary)
		writeAttachmentPart(&builder, attachment)
	}

	writeBoundaryClose(&builder, mixedBoundary)

	return []byte(builder.String()), nil
}

func writeStandaloneBody(builder *strings.Builder, message *Message) yaerrors.Error {
	switch {
	case message.HTML != "" && message.Text != "":
		boundary, err := generateBoundary()
		if err != nil {
			return err.Wrap(logTag + " failed to build multipart message")
		}

		writeHeader(builder, "Content-Type", fmt.Sprintf(contentTypeMultipart, boundary))
		builder.WriteString(crlf)
		writeAlternativeParts(builder, boundary, message)
	case message.HTML != "":
		writeBody(builder, contentTypeHTML, string(message.HTML))
	default:
		writeBody(builder, contentTypeText, string(message.Text))
	}

	return nil
}

func writeNestedBody(builder *strings.Builder, message *Message) yaerrors.Error {
	switch {
	case message.HTML != "" && message.Text != "":
		boundary, err := generateBoundary()
		if err != nil {
			return err.Wrap(logTag + " failed to build nested multipart body")
		}

		writeHeader(builder, "Content-Type", fmt.Sprintf(contentTypeMultipart, boundary))
		builder.WriteString(crlf)
		writeAlternativeParts(builder, boundary, message)
	case message.HTML != "":
		writeBody(builder, contentTypeHTML, string(message.HTML))
		builder.WriteString(crlf)
	default:
		writeBody(builder, contentTypeText, string(message.Text))
		builder.WriteString(crlf)
	}

	return nil
}

func writeAlternativeParts(builder *strings.Builder, boundary string, message *Message) {
	writeBoundaryPart(builder, boundary, contentTypeText, string(message.Text))
	writeBoundaryPart(builder, boundary, contentTypeHTML, string(message.HTML))
	writeBoundaryClose(builder, boundary)
}

func writeAttachmentPart(builder *strings.Builder, attachment Attachment) {
	filename := sanitizeFilename(string(attachment.Filename))

	mediaType := sanitizeHeaderValue(string(attachment.ContentType))
	if mediaType == "" {
		mediaType = contentTypeOctetStream
	}

	writeHeader(builder, "Content-Type", fmt.Sprintf(contentTypeAttachment, mediaType, filename))
	writeHeader(builder, "Content-Transfer-Encoding", contentTransferBase64)
	writeHeader(builder, "Content-Disposition", fmt.Sprintf(contentDisposition, filename))
	builder.WriteString(crlf)
	writeBase64Content(builder, attachment.Content)
	builder.WriteString(crlf)
}

func writeBase64Content(builder *strings.Builder, content []byte) {
	encoded := base64.StdEncoding.EncodeToString(content)

	for start := 0; start < len(encoded); start += base64LineLength {
		end := min(start+base64LineLength, len(encoded))

		builder.WriteString(encoded[start:end])
		builder.WriteString(crlf)
	}
}

func writeBoundaryOpen(builder *strings.Builder, boundary string) {
	builder.WriteString("--")
	builder.WriteString(boundary)
	builder.WriteString(crlf)
}

func writeBoundaryClose(builder *strings.Builder, boundary string) {
	builder.WriteString("--")
	builder.WriteString(boundary)
	builder.WriteString("--")
	builder.WriteString(crlf)
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
