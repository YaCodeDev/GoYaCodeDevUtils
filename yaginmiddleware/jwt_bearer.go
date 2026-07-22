package yaginmiddleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// DefaultJWTContextKey is the Gin context key JWTBearerAuth stores parsed claims
// under unless NewJWTBearerAuthWithContextKey specifies another one.
const DefaultJWTContextKey = "jwt_claims"

// JWTClaims is the HS256 claim shape used by this org's bearer tokens: a numeric
// subject id plus small numeric role/status fields, on top of the usual registered
// claims (exp/iat/nbf/...).
type JWTClaims struct {
	Sub    uint64 `json:"sub"`
	Role   uint8  `json:"role"`
	Status uint8  `json:"status"`
	jwt.RegisteredClaims
}

// GenerateJWT signs a new HS256 token carrying the given subject/role/status, valid
// for life starting now.
func GenerateJWT(
	subject uint64,
	role uint8,
	status uint8,
	life time.Duration,
	secret []byte,
) (string, yaerrors.Error) {
	now := time.Now()

	claims := JWTClaims{
		Sub:    subject,
		Role:   role,
		Status: status,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(life)),
		},
	}

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		return "", yaerrors.FromError(http.StatusInternalServerError, err, "failed to sign token")
	}

	return tokenString, nil
}

// ValidateJWT parses and validates an HS256 token against secret, with a one-minute
// clock-skew leeway, and returns its claims.
//
// This is a plain function, not a Gin middleware, so it can be reused outside an HTTP
// request handler — for example to authenticate a Socket.IO handshake.
func ValidateJWT(tokenString string, secret []byte) (JWTClaims, yaerrors.Error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithLeeway(time.Minute),
	)

	var claims JWTClaims

	token, err := parser.ParseWithClaims(tokenString, &claims, func(_ *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return JWTClaims{}, yaerrors.FromError(
			http.StatusUnauthorized,
			err,
			"failed to parse token",
		)
	}

	if !token.Valid {
		return JWTClaims{}, yaerrors.FromError(
			http.StatusUnauthorized,
			jwt.ErrTokenSignatureInvalid,
			"invalid token",
		)
	}

	return claims, nil
}

// JWTBearerAuth is a Gin middleware that validates an "Authorization: Bearer <jwt>"
// header via ValidateJWT and rejects requests whose claims.Role is below MinRole. On
// success it stores the parsed JWTClaims in the Gin context under its context key.
//
// It covers the common "validate the token and require a minimum role" case.
// Services with extra claim-based rules (status handling, alternate token forms,
// side effects on auth) should call ValidateJWT directly from their own middleware
// instead of wrapping this type.
//
// Must be registered after an ErrorBoundary in the middleware chain; see
// ErrorBoundary's doc comment.
type JWTBearerAuth struct {
	secret     []byte
	minRole    uint8
	contextKey string
}

// NewJWTBearerAuth constructs a JWTBearerAuth requiring claims.Role >= minRole,
// storing parsed claims under DefaultJWTContextKey.
func NewJWTBearerAuth(secret []byte, minRole uint8) *JWTBearerAuth {
	return &JWTBearerAuth{secret: secret, minRole: minRole, contextKey: DefaultJWTContextKey}
}

// NewJWTBearerAuthWithContextKey behaves like NewJWTBearerAuth but stores parsed
// claims under contextKey instead of DefaultJWTContextKey.
func NewJWTBearerAuthWithContextKey(
	secret []byte,
	minRole uint8,
	contextKey string,
) *JWTBearerAuth {
	return &JWTBearerAuth{secret: secret, minRole: minRole, contextKey: contextKey}
}

// Handle implements the Middleware interface.
func (a *JWTBearerAuth) Handle(ctx *gin.Context) {
	tokenString := strings.TrimPrefix(ctx.GetHeader("Authorization"), bearerPrefix)
	if tokenString == "" {
		abortWithError(
			ctx,
			yaerrors.FromString(http.StatusUnauthorized, "authorization header missing"),
		)

		return
	}

	claims, err := ValidateJWT(tokenString, a.secret)
	if err != nil {
		abortWithError(ctx, err.Wrap("invalid token"))

		return
	}

	if claims.Role < a.minRole {
		abortWithError(ctx, yaerrors.FromString(http.StatusUnauthorized, "insufficient role"))

		return
	}

	ctx.Set(a.contextKey, claims)

	ctx.Next()
}
