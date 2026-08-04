package yasmtp_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yasmtp"
)

func isBase64Line(line string) bool {
	if line == "" {
		return false
	}

	return strings.IndexFunc(line, func(symbol rune) bool {
		isUpper := symbol >= 'A' && symbol <= 'Z'
		isLower := symbol >= 'a' && symbol <= 'z'
		isDigit := symbol >= '0' && symbol <= '9'

		return !isUpper && !isLower && !isDigit && symbol != '+' && symbol != '/' &&
			symbol != '='
	}) < 0
}

const (
	testAttachmentName    = "report.csv"
	testAttachmentType    = "text/csv"
	testAttachmentBody    = "phone,product\n+123,telegram\n"
	testLargeContentSize  = 10000
	testMixedMediaType    = "multipart/mixed"
	testAltMediaType      = "multipart/alternative"
	testMaliciousFilename = "a\r\nBcc: attacker@evil.com\".csv"
	testMaxBase64Line     = 76
)

func parseDeliveredMessage(t *testing.T, data string) (*mail.Message, map[string]string) {
	t.Helper()

	parsed, err := mail.ReadMessage(strings.NewReader(data))
	if err != nil {
		t.Fatalf("expected parseable message, got %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("expected parseable content type, got %v", err)
	}

	params["media_type"] = mediaType

	return parsed, params
}

type capturedPart struct {
	contentType string
	encoding    string
	filename    string
	body        []byte
}

func (part capturedPart) decoded(t *testing.T) []byte {
	t.Helper()

	if part.encoding != "base64" {
		return part.body
	}

	joined := strings.NewReplacer("\r", "", "\n", "").Replace(string(part.body))

	decoded, err := base64.StdEncoding.DecodeString(joined)
	if err != nil {
		t.Fatalf("expected decodable base64 part, got %v", err)
	}

	return decoded
}

func collectParts(t *testing.T, body io.Reader, boundary string) []capturedPart {
	t.Helper()

	reader := multipart.NewReader(body, boundary)

	var parts []capturedPart

	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}

		raw, readErr := io.ReadAll(part)
		if readErr != nil {
			t.Fatalf("expected readable part, got %v", readErr)
		}

		parts = append(parts, capturedPart{
			contentType: part.Header.Get("Content-Type"),
			encoding:    part.Header.Get("Content-Transfer-Encoding"),
			filename:    part.FileName(),
			body:        raw,
		})
	}

	return parts
}

func sendWithAttachments(
	t *testing.T,
	message yasmtp.Message,
) string {
	t.Helper()

	server := newFakeSMTPServer(t)
	mailer := newTestMailer(t, server)

	if err := mailer.Send(context.Background(), message); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	mails := server.Mails()
	if len(mails) != 1 {
		t.Fatalf("expected 1 delivered mail, got %d", len(mails))
	}

	return mails[0].Data
}

func TestMailer_SendAttachment(t *testing.T) {
	t.Parallel()

	t.Run("when message has an attachment / then content survives base64", func(t *testing.T) {
		t.Parallel()

		data := sendWithAttachments(t, yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
			Attachments: []yasmtp.Attachment{{
				Filename:    testAttachmentName,
				ContentType: testAttachmentType,
				Content:     yasmtp.Content(testAttachmentBody),
			}},
		})

		parsed, params := parseDeliveredMessage(t, data)

		if params["media_type"] != testMixedMediaType {
			t.Fatalf("expected %q, got %q", testMixedMediaType, params["media_type"])
		}

		parts := collectParts(t, parsed.Body, params["boundary"])
		if len(parts) != 2 {
			t.Fatalf("expected body part and attachment part, got %d", len(parts))
		}

		if got := parts[1].filename; got != testAttachmentName {
			t.Errorf("expected filename %q, got %q", testAttachmentName, got)
		}

		if got := parts[1].encoding; got != "base64" {
			t.Errorf("expected base64 encoding, got %q", got)
		}

		if got := string(parts[1].decoded(t)); got != testAttachmentBody {
			t.Errorf("expected %q, got %q", testAttachmentBody, got)
		}
	})

	t.Run("when message has text html and attachment / then body nests", func(t *testing.T) {
		t.Parallel()

		data := sendWithAttachments(t, yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
			HTML:    testHTML,
			Attachments: []yasmtp.Attachment{{
				Filename:    testAttachmentName,
				ContentType: testAttachmentType,
				Content:     yasmtp.Content(testAttachmentBody),
			}},
		})

		parsed, params := parseDeliveredMessage(t, data)

		if params["media_type"] != testMixedMediaType {
			t.Fatalf("expected %q, got %q", testMixedMediaType, params["media_type"])
		}

		parts := collectParts(t, parsed.Body, params["boundary"])
		if len(parts) != 2 {
			t.Fatalf("expected body part and attachment part, got %d", len(parts))
		}

		innerType, innerParams, err := mime.ParseMediaType(parts[0].contentType)
		if err != nil {
			t.Fatalf("expected parseable nested content type, got %v", err)
		}

		if innerType != testAltMediaType {
			t.Fatalf("expected nested %q, got %q", testAltMediaType, innerType)
		}

		innerParts := collectParts(t, bytes.NewReader(parts[0].body), innerParams["boundary"])
		if len(innerParts) != 2 {
			t.Fatalf("expected text and html parts, got %d", len(innerParts))
		}

		if text := string(innerParts[0].decoded(t)); !strings.Contains(text, testText) {
			t.Errorf("expected text part to contain %q, got %q", testText, text)
		}

		if html := string(innerParts[1].decoded(t)); !strings.Contains(html, testHTML) {
			t.Errorf("expected html part to contain %q, got %q", testHTML, html)
		}
	})

	t.Run("when attachment is large / then base64 lines are wrapped", func(t *testing.T) {
		t.Parallel()

		content := bytes.Repeat([]byte("y"), testLargeContentSize)

		data := sendWithAttachments(t, yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
			Attachments: []yasmtp.Attachment{{
				Filename:    testAttachmentName,
				ContentType: testAttachmentType,
				Content:     content,
			}},
		})

		for _, line := range strings.Split(data, "\r\n") {
			if isBase64Line(line) && len(line) > testMaxBase64Line {
				t.Fatalf("expected wrapped lines, got line of %d chars", len(line))
			}
		}

		parsed, params := parseDeliveredMessage(t, data)

		parts := collectParts(t, parsed.Body, params["boundary"])
		if len(parts) != 2 {
			t.Fatalf("expected body part and attachment part, got %d", len(parts))
		}

		if decoded := parts[1].decoded(t); !bytes.Equal(decoded, content) {
			t.Errorf("expected %d bytes round trip, got %d", len(content), len(decoded))
		}
	})

	t.Run("when filename carries header injection / then it is sanitized", func(t *testing.T) {
		t.Parallel()

		data := sendWithAttachments(t, yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
			Attachments: []yasmtp.Attachment{{
				Filename:    testMaliciousFilename,
				ContentType: testAttachmentType,
				Content:     yasmtp.Content(testAttachmentBody),
			}},
		})

		if strings.Contains(data, "Bcc: attacker@evil.com\r\n") {
			t.Errorf("expected injected header to be neutralized, got %q", data)
		}

		parsed, params := parseDeliveredMessage(t, data)

		if parsed.Header.Get("Bcc") != "" {
			t.Errorf("expected no injected Bcc header, got %q", parsed.Header.Get("Bcc"))
		}

		parts := collectParts(t, parsed.Body, params["boundary"])
		if len(parts) != 2 {
			t.Fatalf("expected body part and attachment part, got %d", len(parts))
		}

		if strings.ContainsAny(parts[1].filename, "\r\n\"") {
			t.Errorf("expected sanitized filename, got %q", parts[1].filename)
		}
	})

	t.Run("when message has no attachments / then alternative is unchanged", func(t *testing.T) {
		t.Parallel()

		data := sendWithAttachments(t, yasmtp.Message{
			To:      []yasmtp.Recipient{testTo},
			Subject: testSubject,
			Text:    testText,
			HTML:    testHTML,
		})

		_, params := parseDeliveredMessage(t, data)

		if params["media_type"] != testAltMediaType {
			t.Fatalf("expected %q, got %q", testAltMediaType, params["media_type"])
		}
	})
}

func TestAttachment_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attachment yasmtp.Attachment
		wantErr    bool
	}{
		{
			name: "when filename and content are set / then it is valid",
			attachment: yasmtp.Attachment{
				Filename: testAttachmentName,
				Content:  yasmtp.Content(testAttachmentBody),
			},
			wantErr: false,
		},
		{
			name: "when filename is empty / then it is invalid",
			attachment: yasmtp.Attachment{
				Content: yasmtp.Content(testAttachmentBody),
			},
			wantErr: true,
		},
		{
			name: "when filename is only unsafe characters / then it is invalid",
			attachment: yasmtp.Attachment{
				Filename: "\r\n\"",
				Content:  yasmtp.Content(testAttachmentBody),
			},
			wantErr: true,
		},
		{
			name: "when content is empty / then it is invalid",
			attachment: yasmtp.Attachment{
				Filename: testAttachmentName,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.attachment.Validate()
			if test.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestMessage_ValidateAttachments(t *testing.T) {
	t.Parallel()

	message := yasmtp.Message{
		To:      []yasmtp.Recipient{testTo},
		Subject: testSubject,
		Text:    testText,
		Attachments: []yasmtp.Attachment{{
			Filename: testAttachmentName,
		}},
	}

	if err := message.Validate(); err == nil {
		t.Fatalf("expected invalid attachment to fail message validation, got nil")
	}
}
