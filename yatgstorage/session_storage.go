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
	"github.com/gotd/td/telegram"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ISessionStorage defines the methods for session management, including encryption, storage, and retrieval.
// It also provides compatibility with the Telegram session storage interface.
type ISessionStorage interface {
	// LoadSession loads the session data from the repository and decrypts it.
	//
	// Example usage:
	//
	//   sessionData, err := storage.LoadSession(ctx)
	LoadSession(ctx context.Context) ([]byte, yaerrors.Error)

	// StoreSession stores the session data in the repository after encrypting it.
	//
	// Example usage:
	//
	//   err := storage.StoreSession(ctx, sessionData)
	StoreSession(ctx context.Context, data []byte) yaerrors.Error

	// TelegramSessionStorageCompatible provides compatibility with `gotd` session storage interface.
	//
	// Example usage:
	//
	//   telegramStorage := storage.TelegramSessionStorageCompatible()
	TelegramSessionStorageCompatible() telegram.SessionStorage
}

// IEntitySessionStorageRepo defines the methods for storing and fetching encrypted authentication keys for a session.
//
// UpdateAuthKey:
// - This method allows updating or inserting an encrypted
// authentication key for a specific entity identified by `entityID`.
//
// FetchAuthKey:
// - This method retrieves the encrypted authentication key associated with the given `entityID`.
type IEntitySessionStorageRepo interface {
	// UpdateAuthKey updates the encrypted authentication key for a specific entity.
	//
	// Example usage:
	//
	//   repo.UpdateAuthKey(ctx, entityID, encryptedAuthKey)
	UpdateAuthKey(ctx context.Context, entityID int64, encryptedAuthKey []byte) yaerrors.Error

	// FetchAuthKey retrieves the encrypted authentication key for a specific entity.
	//
	// Example usage:
	//
	//   repo.FetchAuthKey(ctx, entityID)
	FetchAuthKey(ctx context.Context, entityID int64) ([]byte, yaerrors.Error)
}

// SessionStorage manages session data, including encryption and storage, using the provided repository.
type SessionStorage struct {
	entityID int64
	aes      AES
	repo     IEntitySessionStorageRepo
}

// NewSessionStorage creates a new SessionStorage instance with an in-memory repository for session data storage.
//
// entityID: The ID of the entity (user, bot) whose session is being managed.
// secret: The secret key used for encrypting/decrypting session data.
//
// Returns a pointer to a new SessionStorage instance.
func NewSessionStorage(entityID int64, secret string) *SessionStorage {
	return NewSessionStorageWithCustomRepo(entityID, secret, NewMemorySessionStorage(entityID))
}

// NewSessionStorageWithCustomRepo creates a SessionStorage instance with a custom repository for session data storage.
//
// entityID: The ID of the entity (user, bot) whose session is being managed.
// secret: The secret key used for encrypting/decrypting session data.
// repo: A custom repository implementing the IEntitySessionStorageRepo interface.
//
// Returns a pointer to a new SessionStorage instance.
func NewSessionStorageWithCustomRepo(
	entityID int64,
	secret string,
	repo IEntitySessionStorageRepo,
) *SessionStorage {
	return &SessionStorage{
		entityID: entityID,
		aes:      NewAES(secret),
		repo:     repo,
	}
}

// StoreSession encrypts the session data and stores it using the provided repository.
//
// ctx: The context for the operation.
// data: The session data to be encrypted and stored.
//
// Returns an error if encryption or storage fails.
//
// Example usage:
//
//	err := sessionStorage.StoreSession(ctx, sessionData)
func (s *SessionStorage) StoreSession(ctx context.Context, data []byte) yaerrors.Error {
	out, err := s.aes.Encrypt(data)
	if err != nil {
		return err.Wrap("failed to encrypt AES")
	}

	if err = s.repo.UpdateAuthKey(ctx, s.entityID, out); err != nil {
		return err.Wrap("failed to save updated session")
	}

	return nil
}

// LoadSession retrieves and decrypts the session data from the repository.
//
// ctx: The context for the operation.
//
// Returns the decrypted session data or nil if no session data exists, along with an error if decryption fails.
//
// Example usage:
//
//	sessionData, err := sessionStorage.LoadSession(ctx)
func (s *SessionStorage) LoadSession(ctx context.Context) ([]byte, yaerrors.Error) {
	session, err := s.repo.FetchAuthKey(ctx, s.entityID)
	if err != nil {
		return nil, err.Wrap("failed to fetch session")
	}

	if len(session) == 0 {
		return nil, nil
	}

	out, err := s.aes.Decrypt(session)
	if err != nil {
		return nil, err.Wrap("failed to decrypt AES")
	}

	return out, nil
}

// TelegramSessionStorageCompatible provides a compatibility layer to work with Telegram's SessionStorage interface.
//
// Returns a SessionStorage-compatible implementation that works with gotd.
func (s *SessionStorage) TelegramSessionStorageCompatible() telegram.SessionStorage {
	return &telegramSessionStorage{
		storage: s,
	}
}

// telegramSessionStorage is an implementation of the Telegram SessionStorage interface,
// which is used to store and load sessions in a way compatible with the gotd library.
type telegramSessionStorage struct {
	storage *SessionStorage
}

// StoreSession stores the session data using the SessionStorage's StoreSession method.
func (t *telegramSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	return t.storage.StoreSession(ctx, data)
}

// LoadSession loads the session data using the SessionStorage's LoadSession method.
func (t *telegramSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	return t.storage.LoadSession(ctx)
}

// YaTgClientSession is the database model for storing encrypted session data for a client. It holds the
// entity ID, encrypted authentication key, and the timestamp of when the session was last updated.
type YaTgClientSession struct {
	EntityID         int64     `gorm:"primaryKey;autoIncrement:false"`
	EncryptedAuthKey []byte    `gorm:"type:blob"`
	UpdatedAt        time.Time `gorm:"autoUpdatedAt"`
}

// FieldEncryptedAuthKey is the field name used for storing the encrypted authentication key in the database.
const FieldEncryptedAuthKey = "encrypted_auth_key"

// GormRepo is the repository that manages the session storage in a GORM-backed database.
type GormRepo struct {
	poolDB *gorm.DB
}

// NewGormSessionStorage creates a new GormRepo and runs the migrations for the YaTgClientSession model.
//
// poolDB: The GORM database connection.
//
// Returns a new instance of GormRepo and any errors encountered during migration.
func NewGormSessionStorage(poolDB *gorm.DB) (*GormRepo, yaerrors.Error) {
	if err := poolDB.AutoMigrate(&YaTgClientSession{}); err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"failed to make auto migrate",
		)
	}

	return &GormRepo{poolDB: poolDB}, nil
}

// UpdateAuthKey updates the encrypted authentication key for a specific entity in the database.
//
// ctx: The context for the operation.
// entityID: The ID of the entity whose session is being updated.
// encryptedAuthKey: The new encrypted authentication key.
//
// Returns an error if the update fails.
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

// FetchAuthKey retrieves the encrypted authentication key for a specific entity from the database.
//
// ctx: The context for the operation.
// entityID: The ID of the entity whose session is being fetched.
//
// Returns the encrypted authentication key or an error if the fetch operation fails.
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

// MemoryRepo is an in-memory implementation of the IEntitySessionStorageRepo interface,
// used for testing or simple scenarios where persistence is not required.
type MemoryRepo struct {
	Client YaTgClientSession
}

// NewMemorySessionStorage initializes a new MemoryRepo instance for the given entityID.
//
// entityID: The ID of the entity whose session is being managed.
//
// Returns a new MemoryRepo instance.
func NewMemorySessionStorage(entityID int64) *MemoryRepo {
	return &MemoryRepo{
		Client: YaTgClientSession{
			EntityID:  entityID,
			UpdatedAt: time.Now(),
		},
	}
}

// UpdateAuthKey updates the session's encrypted authentication key in memory.
//
// _ context.Context: The context for the operation (not used in this in-memory implementation).
// _ int64: The entityID (not used in this in-memory implementation).
// encryptedAuthKey: The encrypted authentication key to be stored.
//
// Returns nil after storing the key in memory.
func (m *MemoryRepo) UpdateAuthKey(
	_ context.Context,
	_ int64,
	encryptedAuthKey []byte,
) yaerrors.Error {
	m.Client.EncryptedAuthKey = encryptedAuthKey
	m.Client.UpdatedAt = time.Now()

	return nil
}

// FetchAuthKey fetches the encrypted authentication key from memory.
//
// _ context.Context: The context for the operation (not used in this in-memory implementation).
// _ int64: The entityID (not used in this in-memory implementation).
//
// Returns the encrypted authentication key stored in memory.
func (m *MemoryRepo) FetchAuthKey(
	_ context.Context,
	_ int64,
) ([]byte, yaerrors.Error) {
	return m.Client.EncryptedAuthKey, nil
}

// AES is a struct that holds the encryption key used for AES encryption and decryption.
// It provides methods to encrypt and decrypt data using AES (CTR mode).
type AES struct {
	key []byte
}

// NewAES creates a new AES instance with the given key. The key is used for encryption and decryption.
//
// key: The AES encryption key as a string.
//
// Returns an AES instance that can be used for encrypting and decrypting data.
func NewAES(key string) AES {
	return AES{
		key: DeriveAESKey(key),
	}
}

// Encrypt encrypts data using AES encryption with the provided key (CTR mode).
//
// text: The data to be encrypted.
//
// Returns the encrypted data (ciphertext) and any errors encountered during the process.
//
// Example usage:
//
//	encryptedData, err := aes.Encrypt(sessionData)
func (a *AES) Encrypt(text []byte) ([]byte, yaerrors.Error) {
	block, err := aes.NewCipher(a.key)
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

// Decrypt decrypts data that was encrypted using AES encryption with the provided key (CTR mode).
//
// text: The encrypted data (ciphertext) to be decrypted.
//
// Returns the decrypted data and any errors encountered during the decryption process.
//
// Example usage:
//
//	decryptedData, err := aes.Decrypt(encryptedData)
func (a *AES) Decrypt(text []byte) ([]byte, yaerrors.Error) {
	if len(text) < aes.BlockSize {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			"invalid text block size",
		)
	}

	block, err := aes.NewCipher(a.key)
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

// DeriveAESKey generates a 256-bit AES key from the provided input string using SHA-256 hashing.
//
// This function takes a string, hashes it using SHA-256, and returns the resulting 256-bit key
// that can be used for AES encryption (AES-256). The result is a 32-byte array, which is suitable
// for AES-256 encryption (256-bit key length).
//
// Parameters:
//   - data (string): The input string used to derive the AES key.
//
// Returns:
//   - []byte: A 256-bit AES key derived from the input string.
//
// Example usage:
//
//	key := DeriveAESKey("my_secret_key")
func DeriveAESKey(data string) []byte {
	sum := sha256.Sum256([]byte(data))

	return sum[:]
}
