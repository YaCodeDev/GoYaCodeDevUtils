package yatgstorage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net/http"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IEntitySessionStorageRepo interface {
	UpdateAuthKey(ctx context.Context, entityID int64, encryptedAuthKey []byte) yaerrors.Error
	FetchAuthKey(ctx context.Context, entityID int64) ([]byte, yaerrors.Error)
}

type SessionStorage struct {
	entityID int64
	secret   string
	repo     IEntitySessionStorageRepo
}

func NewSessionStorage(entityID int64, secret string) *SessionStorage {
	return &SessionStorage{
		entityID: entityID,
		secret:   secret,
		repo:     NewMemorySessionStorage(entityID),
	}
}

func NewSessionStorageWithCustomRepo(entityID int64, secret string, repo IEntitySessionStorageRepo) *SessionStorage {
	return &SessionStorage{
		entityID: entityID,
		secret:   secret,
		repo:     repo,
	}
}

func EncryptAES(key string, text []byte) ([]byte, yaerrors.Error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"could not create new cipher",
		)
	}

	cipherText := make([]byte, aes.BlockSize+len(text))

	iv := cipherText[:aes.BlockSize]
	if _, err = io.ReadFull(rand.Reader, iv); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"could not encrypt",
		)
	}

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], text)

	return cipherText, nil
}

func DecryptAES(key string, text []byte) ([]byte, yaerrors.Error) {
	if len(text) < aes.BlockSize {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"invalid text block size",
		)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"could not create new cipher",
		)
	}

	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(text, text)

	return text, nil
}

func DeriveAESKey(data string) []byte {
	sum := sha256.Sum256([]byte(data))

	return sum[:]
}

func (s *SessionStorage) StoreSession(ctx context.Context, data []byte) error {
	out, err := EncryptAES(s.secret, data)
	if err != nil {
		return err.Wrap("failed to encrypt AES")
	}

	if err = s.repo.UpdateAuthKey(ctx, s.entityID, out); err != nil {
		return err.Wrap("failed to save updated session")
	}

	return nil
}

func (s *SessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	session, err := s.repo.FetchAuthKey(ctx, s.entityID)
	if err != nil {
		return nil, err.Wrap("failed to fetch session")
	}

	if len(session) == 0 {
		return nil, nil
	}

	out, err := DecryptAES(s.secret, session)
	if err != nil {
		return nil, err.Wrap("failed to decrypt AES")
	}

	return out, nil
}

type YaTgClientSession struct {
	EntityID         int64     `gorm:"primaryKey;autoIncrement:false"`
	EncryptedAuthKey []byte    `gorm:"type:blob"`
	UpdatedAt        time.Time `gorm:"autoUpdatedAt"`
}

const FieldEncryptedAuthKey = "encrypted_session"

type GormRepo struct {
	poolDB *gorm.DB
}

func NewGormSessionStore(poolDB *gorm.DB) *GormRepo {
	return &GormRepo{
		poolDB: poolDB,
	}
}

func (g *GormRepo) UpdateAuthKey(
	ctx context.Context,
	entityID int64,
	encryptedAuthKey []byte,
) yaerrors.Error {
	if err := g.poolDB.WithContext(ctx).
		Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{FieldEncryptedAuthKey})}).
		Model(&YaTgClientSession{}).
		Where(&YaTgClientSession{EntityID: entityID}).
		Create(&YaTgClientSession{
			EntityID:         entityID,
			EncryptedAuthKey: encryptedAuthKey,
		}).Error; err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to update encrypted auth key",
		)
	}

	return nil
}

func (g *GormRepo) FetchAuthKey(
	ctx context.Context,
	entityID int64,
) ([]byte, yaerrors.Error) {
	var botSession YaTgClientSession

	if err := g.poolDB.WithContext(ctx).
		Model(&YaTgClientSession{}).
		Where(&YaTgClientSession{EntityID: entityID}).
		Select(FieldEncryptedAuthKey).
		Take(&botSession).Error; err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to fetch encrypted auth key",
		)
	}

	return botSession.EncryptedAuthKey, nil
}

type MemoryRepo struct {
	Client YaTgClientSession
}

func NewMemorySessionStorage(entityID int64) *MemoryRepo {
	return &MemoryRepo{
		Client: YaTgClientSession{
			EntityID:  entityID,
			UpdatedAt: time.Now(),
		},
	}
}

func (m *MemoryRepo) UpdateAuthKey(
	_ context.Context,
	_ int64,
	encryptedAuthKey []byte,
) yaerrors.Error {
	m.Client.EncryptedAuthKey = encryptedAuthKey
	m.Client.UpdatedAt = time.Now()

	return nil
}

func (m *MemoryRepo) FetchAuthKey(
	_ context.Context,
	_ int64,
) ([]byte, yaerrors.Error) {
	return m.Client.EncryptedAuthKey, nil
}
