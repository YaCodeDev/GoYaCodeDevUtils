package yatgstorage_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

const (
	entityID         = 1000
	secret           = "123456789:ABCDFEG"
	encryptedAuthKey = "stolyarovtop"
)

func newMockDB(t *testing.T) *gorm.DB {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in memory")
	}

	poolDB, err := gorm.Open(
		gorm.Dialector(
			sqlite.Dialector{
				Conn:       sqlDB,
				DriverName: "sqlite",
			},
		), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to in-memory database: %v", err)
	}

	return poolDB
}

func TestSessionStorage_WorkflowWorks(t *testing.T) {
	ctx := context.Background()

	storage := yatgstorage.NewSessionStorage(entityID, secret)

	_ = storage.StoreSession(ctx, []byte(encryptedAuthKey))

	aes := yatgstorage.NewAES(secret)

	expected, _ := aes.Encrypt([]byte(encryptedAuthKey))

	assert.NotEqual(t, []byte(encryptedAuthKey), expected)

	expected, _ = aes.Decrypt(expected)

	result, _ := storage.LoadSession(ctx)

	assert.Equal(t, expected, result)
}

func TestAutoMigrate_Works(t *testing.T) {
	poolDB := newMockDB(t)

	_, _ = yatgstorage.NewGormSessionStorage(poolDB)

	expected := true

	assert.Equal(t, expected, poolDB.Migrator().HasTable(&yatgstorage.YaTgClientSession{}))
}

func TestGormSessionStorage_WorkflowWorks(t *testing.T) {
	ctx := context.Background()

	poolDB := newMockDB(t)

	storage, _ := yatgstorage.NewGormSessionStorage(poolDB)

	_ = storage.UpdateAuthKey(ctx, entityID, []byte(encryptedAuthKey))

	t.Run("Create works", func(t *testing.T) {
		err := poolDB.Where(&yatgstorage.YaTgClientSession{
			EntityID:         entityID,
			EncryptedAuthKey: []byte(encryptedAuthKey),
		}).Find(&yatgstorage.YaTgClientSession{}).Error

		assert.Equal(t, nil, err)
	})

	t.Run("Fetch works", func(t *testing.T) {
		expected := yatgstorage.YaTgClientSession{}

		_ = poolDB.Where(&yatgstorage.YaTgClientSession{
			EntityID:         entityID,
			EncryptedAuthKey: []byte(encryptedAuthKey),
		}).Find(&expected)

		result, _ := storage.FetchAuthKey(ctx, entityID)

		assert.Equal(t, expected.EncryptedAuthKey, result)
	})
}

func TestMemorySessionStorage_WorkflowWorks(t *testing.T) {
	ctx := context.Background()

	storage := yatgstorage.NewMemorySessionStorage(entityID)

	_ = storage.UpdateAuthKey(ctx, entityID, []byte(encryptedAuthKey))

	t.Run("Create works", func(t *testing.T) {
		assert.Equal(t, []byte(encryptedAuthKey), storage.Client.EncryptedAuthKey)
	})

	t.Run("Fetch works", func(t *testing.T) {
		result, _ := storage.FetchAuthKey(ctx, entityID)

		assert.Equal(t, []byte(encryptedAuthKey), result)
	})
}
