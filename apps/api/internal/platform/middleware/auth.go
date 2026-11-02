// Package middleware provides HTTP middleware for authentication and
// authorization using JWT bearer tokens. It is shared by every service that
// accepts end-user tokens, so that the token contract is defined exactly
// once: user-service mints the tokens here, and every service verifies them
// with the same code.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/golang-jwt/jwt/v5"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/httperr"
)

// roleAdmin is the role name granted administrative access.
const roleAdmin = "admin"

// roleManager is the role name granted scoped management access.
const roleManager = "manager"

// jwtIssuer is the iss claim set on every session token.
const jwtIssuer = "user-service"

// jwtAudience is the aud claim set on every session token and required on
// parse.
const jwtAudience = "pupitre-api"

// signingAlg is the only JOSE algorithm accepted on parse. Pinning it means
// a token header advertising another algorithm is rejected before the
// signature is even considered.
const signingAlg = "HS256"

// bearerPrefix is the required prefix of the Authorization header value.
const bearerPrefix = "Bearer "

// Claims are the JWT claims embedded in issued access tokens.
type Claims struct {
	jwt.RegisteredClaims

	Email             string `json:"email"`
	Role              string `json:"role"`
	PreferredUsername string `json:"preferred_username,omitempty"` //nolint:tagliatelle // preferred_username is the standard OIDC claim name
}

// contextKey is the type used for context values set by this package.
type contextKey string

// ClaimsKey is the context key under which verified Claims are stored.
const ClaimsKey contextKey = "claims"

// CreateToken issues a signed JWT for the given user identity.
func CreateToken(userID, email, role, username, secret string, expiryHours int) (string, error) {
	claims := Claims{
		Email:             email,
		Role:              role,
		PreferredUsername: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryHours) * time.Hour)),
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return token, nil
}

// VerifyToken parses and validates a JWT, returning its Claims. The parser
// is pinned to a single symmetric algorithm and requires iss, aud and exp,
// so a token that omits any of them is rejected rather than treated as
// unconstrained.
func VerifyToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}

		return []byte(secret), nil
	},
		jwt.WithValidMethods([]string{signingAlg}),
		jwt.WithAudience(jwtAudience),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrTokenInvalidClaims
}

// GetClaims retrieves the Claims stored in the request context by Auth.
func GetClaims(req *http.Request) *Claims {
	if c, ok := req.Context().Value(ClaimsKey).(*Claims); ok {
		return c
	}

	return nil
}

// Session is a signed-in user's state on the server, read on each request.
type Session struct {
	// Valid is false when the user no longer exists, is deactivated, or
	// revoked the sessions the token belongs to.
	Valid bool
	// Role is the user's current role: it wins over the role in the token,
	// so a demotion takes effect at once.
	Role string
}

// SessionCheck reads the session of the user subject for a token issued at
// issuedAt (docs/specs/190-backend-defects.md B7).
type SessionCheck func(ctx context.Context, subject string, issuedAt time.Time) (Session, error)

// Option tunes a guard.
type Option func(*guardOptions)

// guardOptions are the settings an Option changes.
type guardOptions struct {
	check SessionCheck
}

// WithSessionCheck makes the guard check, on every request, that the
// token's user may still use it. A nil check is ignored.
func WithSessionCheck(check SessionCheck) Option {
	return func(options *guardOptions) {
		options.check = check
	}
}

// guard is the single authentication path used by every exported middleware
// in this package: it verifies the bearer token, rejects a token that
// carries no subject, and optionally enforces that the token's role is one
// of allowedRoles. Keeping one implementation is deliberate — the previous
// per-service copies of this logic drifted, and only some of them checked
// the subject. An empty allowedRoles means authentication only.
func guard(secret, forbiddenMsg string, allowedRoles []string, opts []Option) func(http.Handler) http.Handler {
	var options guardOptions
	for _, opt := range opts {
		opt(&options)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			auth := req.Header.Get("Authorization")
			if !strings.HasPrefix(auth, bearerPrefix) {
				httperr.Write(resp, http.StatusUnauthorized, "Missing Authorization header")

				return
			}

			claims, err := VerifyToken(strings.TrimPrefix(auth, bearerPrefix), secret)
			if err != nil {
				zap.L().Error("token verification failed", zap.Error(err))
				httperr.Write(resp, http.StatusUnauthorized, "Invalid token")

				return
			}

			// Handlers read Subject as the authenticated user ID and pass it
			// straight into ownership and enrollment queries, so an empty
			// one must never reach them.
			if claims.Subject == "" {
				httperr.Write(resp, http.StatusUnauthorized, "Invalid token: missing user ID")

				return
			}

			if options.check != nil && !applySession(resp, req, claims, options.check) {
				return
			}

			if len(allowedRoles) > 0 && !slices.Contains(allowedRoles, claims.Role) {
				httperr.Write(resp, http.StatusForbidden, forbiddenMsg)

				return
			}

			ctx := context.WithValue(req.Context(), ClaimsKey, claims)
			next.ServeHTTP(resp, req.WithContext(ctx))
		})
	}
}

// applySession runs the session check of a verified token. It writes the
// error response and returns false when the session is not valid; otherwise
// it replaces the token's role with the user's current one.
func applySession(resp http.ResponseWriter, req *http.Request, claims *Claims, check SessionCheck) bool {
	var issuedAt time.Time
	if claims.IssuedAt != nil {
		issuedAt = claims.IssuedAt.Time
	}

	session, err := check(req.Context(), claims.Subject, issuedAt)
	if err != nil {
		zap.L().Error("session check failed", zap.Error(err))
		httperr.Write(resp, http.StatusInternalServerError, "Session check failed")

		return false
	}

	if !session.Valid {
		httperr.Write(resp, http.StatusUnauthorized, "Session expired")

		return false
	}

	// The role of the database, not the one frozen in the token.
	claims.Role = session.Role

	return true
}

// Auth returns middleware that requires a valid bearer token.
func Auth(secret string, opts ...Option) func(http.Handler) http.Handler {
	return guard(secret, "", nil, opts)
}

// Manager returns middleware that requires a valid bearer token whose
// claims carry the manager role.
func Manager(secret string, opts ...Option) func(http.Handler) http.Handler {
	return guard(secret, "Manager access required", []string{roleManager}, opts)
}

// Admin returns middleware that requires a valid bearer token whose
// claims carry the admin role.
func Admin(secret string, opts ...Option) func(http.Handler) http.Handler {
	return guard(secret, "Admin access required", []string{roleAdmin}, opts)
}

// ManagerOrAdmin returns middleware that requires a valid bearer token whose
// claims carry either the manager or admin role.
func ManagerOrAdmin(secret string, opts ...Option) func(http.Handler) http.Handler {
	return guard(secret, "Manager or admin access required", []string{roleManager, roleAdmin}, opts)
}
