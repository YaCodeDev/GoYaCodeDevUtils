// Package yatginitdata parses and validates Telegram Mini App `initData` login
// payloads (window.Telegram.WebApp.initData), per Telegram's documented algorithm:
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
//
// # Algorithm
//
// initData arrives as a URL query string. Validation recomputes an HMAC-SHA256 hash
// over a "data-check-string" built from every field except hash, sorted
// alphabetically by key and joined with "\n" as `key=value` pairs, and compares it
// (hex-encoded) against the received hash field. The HMAC key is itself
// HMAC-SHA256("WebAppData", botToken).
//
// Cross-checked against Telegram's official documentation and against the
// github.com/telegram-mini-apps/init-data-golang v1.5.0 reference implementation
// this package replaces.
//
// # Example
//
//	err := yatginitdata.Validate(initData, botToken, 24*time.Hour)
//	if err != nil {
//	    // reject login
//	}
//
//	data, err := yatginitdata.Parse(initData)
//	if err != nil {
//	    // malformed payload
//	}
//	fmt.Println(data.User.ID, data.User.FirstName)
package yatginitdata

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// webAppDataKey is the fixed HMAC key Telegram uses to derive the per-bot secret key
// from the bot token. It is a protocol constant, not a secret.
const webAppDataKey = "WebAppData"

// User describes the `user`/`receiver` fields of an initData payload.
// https://docs.telegram-mini-apps.com/launch-parameters/init-data#user
//
//nolint:tagliatelle // fields mirror Telegram's own snake_case wire format, not this org's convention
type User struct {
	ID                    int64  `json:"id"`
	IsBot                 bool   `json:"is_bot"`
	FirstName             string `json:"first_name"`
	LastName              string `json:"last_name"`
	Username              string `json:"username"`
	LanguageCode          string `json:"language_code"`
	IsPremium             bool   `json:"is_premium"`
	AllowsWriteToPm       bool   `json:"allows_write_to_pm"`
	AddedToAttachmentMenu bool   `json:"added_to_attachment_menu"`
	PhotoURL              string `json:"photo_url"`
}

// Chat describes the `chat` field of an initData payload.
// https://docs.telegram-mini-apps.com/launch-parameters/init-data#chat
//
//nolint:tagliatelle // fields mirror Telegram's own snake_case wire format, not this org's convention
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	PhotoURL string `json:"photo_url"`
	Username string `json:"username"`
}

// Data holds a parsed initData payload.
// https://docs.telegram-mini-apps.com/launch-parameters/init-data#parameters-list
type Data struct {
	AuthDateRaw     int64
	CanSendAfterRaw int64
	Chat            Chat
	ChatType        string
	ChatInstance    int64
	Hash            string
	QueryID         string
	Receiver        User
	StartParam      string
	User            User
}

// AuthDate returns AuthDateRaw as a time.Time.
func (d *Data) AuthDate() time.Time {
	return time.Unix(d.AuthDateRaw, 0)
}

// CanSendAfter returns the earliest time at which answerWebAppQuery may be called,
// derived from AuthDate and CanSendAfterRaw.
func (d *Data) CanSendAfter() time.Time {
	return d.AuthDate().Add(time.Duration(d.CanSendAfterRaw) * time.Second)
}

// Parse converts a raw initData query string into a Data struct. It does not verify
// the signature — call Validate first (or alongside) for any payload from an
// untrusted client.
func Parse(initData string) (Data, yaerrors.Error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return Data{}, yaerrors.FromError(
			http.StatusBadRequest,
			err,
			"failed to parse init data as query string",
		)
	}

	data := Data{
		Hash:       values.Get("hash"),
		QueryID:    values.Get("query_id"),
		StartParam: values.Get("start_param"),
		ChatType:   values.Get("chat_type"),
	}

	var yaErr yaerrors.Error

	if data.AuthDateRaw, yaErr = parseInt64Field(values, "auth_date"); yaErr != nil {
		return Data{}, yaErr
	}

	if data.CanSendAfterRaw, yaErr = parseInt64Field(values, "can_send_after"); yaErr != nil {
		return Data{}, yaErr
	}

	if data.ChatInstance, yaErr = parseInt64Field(values, "chat_instance"); yaErr != nil {
		return Data{}, yaErr
	}

	if yaErr := parseJSONField(values, "user", &data.User); yaErr != nil {
		return Data{}, yaErr
	}

	if yaErr := parseJSONField(values, "receiver", &data.Receiver); yaErr != nil {
		return Data{}, yaErr
	}

	if yaErr := parseJSONField(values, "chat", &data.Chat); yaErr != nil {
		return Data{}, yaErr
	}

	return data, nil
}

func parseInt64Field(values url.Values, key string) (int64, yaerrors.Error) {
	raw := values.Get(key)
	if raw == "" {
		return 0, nil
	}

	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, yaerrors.FromError(
			http.StatusBadRequest,
			err,
			"failed to parse "+key,
		)
	}

	return parsed, nil
}

func parseJSONField(values url.Values, key string, target any) yaerrors.Error {
	raw := values.Get(key)
	if raw == "" {
		return nil
	}

	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return yaerrors.FromError(
			http.StatusBadRequest,
			err,
			"failed to parse "+key,
		)
	}

	return nil
}

// Validate recomputes the initData signature and compares it against the received
// hash field, using botToken as the signing secret.
//
// When maxAge is greater than zero, Validate also rejects payloads whose auth_date is
// missing or older than maxAge. Passing maxAge <= 0 skips the expiry check.
func Validate(initData, botToken string, maxAge time.Duration) yaerrors.Error {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return yaerrors.FromError(
			http.StatusBadRequest,
			err,
			"failed to parse init data as query string",
		)
	}

	hash := values.Get("hash")
	if hash == "" {
		return yaerrors.FromString(http.StatusBadRequest, "init data hash is missing")
	}

	if yaErr := validateNotExpired(values, maxAge); yaErr != nil {
		return yaErr
	}

	dataCheckString := buildDataCheckString(values)

	if computeHash(dataCheckString, botToken) != hash {
		return yaerrors.FromString(http.StatusBadRequest, "init data hash is invalid")
	}

	return nil
}

func validateNotExpired(values url.Values, maxAge time.Duration) yaerrors.Error {
	if maxAge <= 0 {
		return nil
	}

	raw := values.Get("auth_date")
	if raw == "" {
		return yaerrors.FromString(http.StatusBadRequest, "init data auth_date is missing")
	}

	authDateUnix, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return yaerrors.FromError(http.StatusBadRequest, err, "failed to parse auth_date")
	}

	if time.Unix(authDateUnix, 0).Add(maxAge).Before(time.Now()) {
		return yaerrors.FromString(http.StatusBadRequest, "init data is expired")
	}

	return nil
}

func buildDataCheckString(values url.Values) string {
	pairs := make([]string, 0, len(values))

	for key, value := range values {
		if key == "hash" || len(value) == 0 {
			continue
		}

		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value[0]))
	}

	sort.Strings(pairs)

	return strings.Join(pairs, "\n")
}

// Sign builds a signed initData query string from fields and authDate, for tests and
// local tooling that need a valid payload without a real Telegram client (mirrors
// this org's previous helpers/hash_telegram CLI use case). Any "hash" or "auth_date"
// entries already in fields are ignored; authDate always wins for auth_date.
func Sign(fields map[string]string, botToken string, authDate time.Time) string {
	values := url.Values{}

	pairs := make([]string, 0, len(fields)+1)

	for key, value := range fields {
		if key == "hash" || key == "auth_date" {
			continue
		}

		values.Set(key, value)
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, value))
	}

	authDateValue := strconv.FormatInt(authDate.Unix(), 10)
	values.Set("auth_date", authDateValue)
	pairs = append(pairs, "auth_date="+authDateValue)

	sort.Strings(pairs)

	values.Set("hash", computeHash(strings.Join(pairs, "\n"), botToken))

	return values.Encode()
}

func computeHash(dataCheckString, botToken string) string {
	secretKey := hmac.New(sha256.New, []byte(webAppDataKey))
	secretKey.Write([]byte(botToken))

	mac := hmac.New(sha256.New, secretKey.Sum(nil))
	mac.Write([]byte(dataCheckString))

	return hex.EncodeToString(mac.Sum(nil))
}
