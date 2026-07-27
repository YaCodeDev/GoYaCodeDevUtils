package yatotp

const (
	// AlgorithmSHA1 is the HMAC hash every mainstream authenticator app
	// implements, and the default RFC 6238 defines.
	AlgorithmSHA1 Algorithm = "SHA1"

	// AlgorithmSHA256 is accepted by fewer authenticator apps than AlgorithmSHA1.
	AlgorithmSHA256 Algorithm = "SHA256"

	// AlgorithmSHA512 is accepted by fewer authenticator apps than AlgorithmSHA1.
	AlgorithmSHA512 Algorithm = "SHA512"
)

const (
	// DefaultPeriod is the RFC 6238 default time step, in seconds.
	DefaultPeriod Period = 30

	// DefaultDigits is the RFC 6238 default code length.
	DefaultDigits Digits = 6

	// DefaultSkew tolerates one time step either side of the current one,
	// covering ordinary clock drift between a phone and a server.
	DefaultSkew Skew = 1

	// DefaultSecretBytes is the shared-secret length RFC 4226 recommends.
	DefaultSecretBytes = 20

	// DefaultRecoveryCodeCount is how many single-use recovery codes
	// NewRecoveryCodes mints unless a caller asks for another number.
	DefaultRecoveryCodeCount RecoveryCodeCount = 10

	// MinDigits and MaxDigits bound a Digits value. Six is the shortest code RFC
	// 4226 allows; eight is the longest authenticator apps display.
	MinDigits Digits = 6
	MaxDigits Digits = 8

	// MaxSkew bounds how many extra time steps Validate will accept. A wide skew
	// lengthens the window a stolen code stays usable in, so it is capped rather
	// than left to a caller's config typo.
	MaxSkew Skew = 10
)

const (
	logTag = "[TOTP]"

	redactedValue = "[REDACTED]"

	minSecretBytes      = 16
	recoveryCodeBytes   = 10
	recoveryCodeMaxSize = 64

	uriScheme = "otpauth"
	uriHost   = "totp"

	queryFieldSecret    = "secret"
	queryFieldIssuer    = "issuer"
	queryFieldAlgorithm = "algorithm"
	queryFieldDigits    = "digits"
	queryFieldPeriod    = "period"

	hotpTruncationMask = 0x7fffffff
	hotpOffsetMask     = 0x0f
	hotpCounterBytes   = 8

	decimalBase = 10
)
