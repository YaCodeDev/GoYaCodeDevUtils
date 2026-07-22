package yatginitdata_test

import (
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatginitdata"
	"github.com/stretchr/testify/assert"
)

const testBotToken = "123456:AAtest-bot-token"

func TestValidate_AcceptsSignedPayload(t *testing.T) {
	t.Parallel()

	authDate := time.Now()

	initData := yatginitdata.Sign(map[string]string{
		"query_id": "AAabc123",
		"user":     `{"id":42,"first_name":"Ada","last_name":"Lovelace"}`,
	}, testBotToken, authDate)

	err := yatginitdata.Validate(initData, testBotToken, 24*time.Hour)

	assert.Nil(t, err, "signed payload should validate")
}

func TestValidate_RejectsTamperedPayload(t *testing.T) {
	t.Parallel()

	authDate := time.Now()

	initData := yatginitdata.Sign(map[string]string{
		"user": `{"id":42,"first_name":"Ada"}`,
	}, testBotToken, authDate)

	tampered := initData + "0"

	err := yatginitdata.Validate(tampered, testBotToken, 24*time.Hour)

	assert.NotNil(t, err, "tampered payload should be rejected")
}

func TestValidate_RejectsWrongBotToken(t *testing.T) {
	t.Parallel()

	authDate := time.Now()

	initData := yatginitdata.Sign(map[string]string{
		"user": `{"id":42,"first_name":"Ada"}`,
	}, testBotToken, authDate)

	err := yatginitdata.Validate(initData, "other-bot-token", 24*time.Hour)

	assert.NotNil(t, err, "payload signed by a different bot token should be rejected")
}

func TestValidate_RejectsExpired(t *testing.T) {
	t.Parallel()

	authDate := time.Now().Add(-48 * time.Hour)

	initData := yatginitdata.Sign(map[string]string{
		"user": `{"id":42,"first_name":"Ada"}`,
	}, testBotToken, authDate)

	err := yatginitdata.Validate(initData, testBotToken, 24*time.Hour)

	assert.NotNil(t, err, "stale auth_date should be rejected")
}

func TestValidate_SkipsExpiryCheckWhenMaxAgeIsZero(t *testing.T) {
	t.Parallel()

	authDate := time.Now().Add(-48 * time.Hour)

	initData := yatginitdata.Sign(map[string]string{
		"user": `{"id":42,"first_name":"Ada"}`,
	}, testBotToken, authDate)

	err := yatginitdata.Validate(initData, testBotToken, 0)

	assert.Nil(t, err, "maxAge<=0 should skip the expiry check")
}

func TestValidate_RejectsMissingHash(t *testing.T) {
	t.Parallel()

	err := yatginitdata.Validate("query_id=AAabc123", testBotToken, 24*time.Hour)

	assert.NotNil(t, err, "missing hash should be rejected")
}

func TestParse_PopulatesFields(t *testing.T) {
	t.Parallel()

	authDate := time.Now()

	initData := yatginitdata.Sign(map[string]string{
		"query_id":      "AAabc123",
		"start_param":   "ref_42",
		"chat_type":     "private",
		"chat_instance": "987654321",
		"user":          `{"id":42,"first_name":"Ada","last_name":"Lovelace","username":"ada"}`,
	}, testBotToken, authDate)

	data, err := yatginitdata.Parse(initData)

	assert.Nil(t, err, "well-formed payload should parse")
	assert.Equal(t, "AAabc123", data.QueryID)
	assert.Equal(t, "ref_42", data.StartParam)
	assert.Equal(t, "private", data.ChatType)
	assert.Equal(t, int64(987654321), data.ChatInstance)
	assert.Equal(t, int64(42), data.User.ID)
	assert.Equal(t, "Ada", data.User.FirstName)
	assert.Equal(t, "Lovelace", data.User.LastName)
	assert.Equal(t, "ada", data.User.Username)
	assert.WithinDuration(t, authDate, data.AuthDate(), time.Second)
}

func TestParse_RejectsMalformedUserJSON(t *testing.T) {
	t.Parallel()

	authDate := time.Now()

	initData := yatginitdata.Sign(map[string]string{
		"user": "not-json",
	}, testBotToken, authDate)

	_, err := yatginitdata.Parse(initData)

	assert.NotNil(t, err, "malformed user JSON should fail to parse")
}

func TestParse_RejectsMalformedQueryString(t *testing.T) {
	t.Parallel()

	_, err := yatginitdata.Parse("%zz")

	assert.NotNil(t, err, "malformed query string should fail to parse")
}
