package yatotp

import "errors"

var (
	ErrSecretRequired      = errors.New("[TOTP] secret is required")
	ErrSecretInvalid       = errors.New("[TOTP] secret is not valid base32")
	ErrSecretTooShort      = errors.New("[TOTP] secret is shorter than the minimum length")
	ErrCodeRequired        = errors.New("[TOTP] code is required")
	ErrCodeLength          = errors.New("[TOTP] code length does not match the configured digits")
	ErrCodeNotNumeric      = errors.New("[TOTP] code is not numeric")
	ErrConfigDigits        = errors.New("[TOTP] config digits is out of range")
	ErrConfigPeriod        = errors.New("[TOTP] config period is required")
	ErrConfigSkew          = errors.New("[TOTP] config skew is out of range")
	ErrConfigAlgorithm     = errors.New("[TOTP] config algorithm is not supported")
	ErrConfigIssuerInvalid = errors.New("[TOTP] config issuer must not contain a colon")
	ErrAccountNameRequired = errors.New("[TOTP] account name is required")
	ErrAccountNameInvalid  = errors.New("[TOTP] account name must not contain a colon")
	ErrGenerateSecret      = errors.New("[TOTP] failed to read random bytes for a secret")
	ErrGenerateRecovery    = errors.New("[TOTP] failed to read random bytes for a recovery code")
	ErrRecoveryCodeCount   = errors.New("[TOTP] recovery code count is out of range")
)
