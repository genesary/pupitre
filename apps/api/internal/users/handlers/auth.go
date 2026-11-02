package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/jojo91550/pupitre2/apps/api/internal/platform/metrics"
	"github.com/jojo91550/pupitre2/apps/api/internal/platform/middleware"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/models"
	"github.com/jojo91550/pupitre2/apps/api/internal/users/repository"
)

// bcryptCost is the bcrypt hashing cost used for all stored passwords.
const bcryptCost = 12

// Settings/validation literals shared across this file, pulled out to
// satisfy goconst and mnd.
const (
	authSettingTrue         = "true"
	authEmailDomainParts    = 2
	authMinUsernameLen      = 3
	authMsgUsernameTooShort = "Username must be at least 3 characters"
	authProviderLocal       = "local"
	errNoLocalPassword      = "This account uses SSO login and has no local password."
	errDatabase             = "Database error"
)

// passwordHasUpper reports whether password contains an uppercase letter.
func passwordHasUpper(password string) bool {
	for _, char := range password {
		if unicode.IsUpper(char) {
			return true
		}
	}

	return false
}

// passwordHasDigit reports whether password contains a decimal digit.
func passwordHasDigit(password string) bool {
	for _, char := range password {
		if unicode.IsDigit(char) {
			return true
		}
	}

	return false
}

// validatePasswordPolicy checks password against the configured min
// length, uppercase, and digit requirements. It returns a rejection
// message and false, or an empty message and true when password is
// acceptable.
func (s *State) validatePasswordPolicy(request *http.Request, password string) (string, bool) {
	ctx := request.Context()

	minLen, _ := strconv.Atoi(repository.ReadSetting(ctx, s.Repos.Settings, settingKeyPasswordMinLength, "8"))
	if minLen < 1 {
		minLen = 8
	}

	if len(password) < minLen {
		return "Password must be at least " + strconv.Itoa(minLen) + " characters", false
	}

	requireUpper := repository.ReadSetting(ctx, s.Repos.Settings, settingKeyPasswordRequireUppercase, "false") == authSettingTrue
	if requireUpper && !passwordHasUpper(password) {
		return "Password must contain at least one uppercase letter", false
	}

	requireDigit := repository.ReadSetting(ctx, s.Repos.Settings, settingKeyPasswordRequireNumber, "false") == authSettingTrue
	if requireDigit && !passwordHasDigit(password) {
		return "Password must contain at least one number", false
	}

	return "", true
}

// registerRequest is the JSON payload for POST /api/auth/register.
type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginRequest is the JSON payload for POST /api/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// userPublicRow is the public-facing representation of a user account,
// returned by the auth/profile endpoints.
type userPublicRow struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`

	AvatarURL *string `json:"avatarUrl"`
	Bio       *string `json:"bio"`
	IsActive  bool    `json:"isActive"`

	AuthProvider string `json:"authProvider"`
	CreatedAt    string `json:"createdAt"`
}

// authResponse wraps a JWT and the authenticated user's public profile,
// returned by register/login.
type authResponse struct {
	Token string        `json:"token"`
	User  userPublicRow `json:"user"`
}

// toUserPublicRow converts a persisted user into its public representation.
func toUserPublicRow(user *models.User) userPublicRow {
	return userPublicRow{
		ID:           user.ID.String(),
		Username:     user.Username,
		Email:        user.Email,
		Role:         user.Role,
		AvatarURL:    user.AvatarURL,
		Bio:          user.Bio,
		IsActive:     user.IsActive,
		AuthProvider: user.AuthProvider,
		CreatedAt:    user.CreatedAt.Format(time.RFC3339),
	}
}

// registrationAllowedForEmail reports whether email's domain is allowed
// by the registration_email_whitelist setting. An empty/unset whitelist
// permits every domain.
func (s *State) registrationAllowedForEmail(ctx context.Context, email string) bool {
	whitelist := repository.ReadSetting(ctx, s.Repos.Settings, "registration_email_whitelist", "")
	if strings.TrimSpace(whitelist) == "" {
		return true
	}

	domain := ""

	parts := strings.SplitN(email, "@", authEmailDomainParts)
	if len(parts) == authEmailDomainParts {
		domain = parts[1]
	}

	for allowedDomain := range strings.SplitSeq(whitelist, ",") {
		if strings.EqualFold(strings.TrimSpace(allowedDomain), domain) {
			return true
		}
	}

	return false
}

// registerValidationCheck validates a registration request against
// platform settings and password policy. It returns the HTTP
// status/message to send back when registration must be rejected, or
// ok == true to proceed.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func (s *State) registerValidationCheck(request *http.Request, req registerRequest) (int, string, bool) {
	ctx := request.Context()

	if repository.ReadSetting(ctx, s.Repos.Settings, settingKeyRegistrationEnabled, authSettingTrue) != authSettingTrue {
		return http.StatusForbidden, "New user registration is currently disabled.", false
	}

	if !s.registrationAllowedForEmail(ctx, req.Email) {
		return http.StatusForbidden, "Registration is restricted to specific email domains.", false
	}

	if len(req.Username) < authMinUsernameLen {
		return http.StatusBadRequest, authMsgUsernameTooShort, false
	}

	if msg, ok := s.validatePasswordPolicy(request, req.Password); !ok {
		return http.StatusBadRequest, msg, false
	}

	return http.StatusOK, "", true
}

// createUser hashes req.Password and inserts a new user row, returning
// its public representation.
func (s *State) createUser(ctx context.Context, req registerRequest) (userPublicRow, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return userPublicRow{}, fmt.Errorf("hash password: %w", err)
	}

	passwordHash := string(hash)
	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: &passwordHash,
		Role:         groupsRoleStudent,
		IsActive:     true,
		AuthProvider: authProviderLocal,
	}

	err = s.Repos.Users.Create(ctx, user)
	if err != nil {
		return userPublicRow{}, fmt.Errorf("create user: %w", err)
	}

	return toUserPublicRow(user), nil
}

// Register godoc
// @Summary  Register a new user
// @Tags     Auth
// @Accept   json
// @Produce  json
// @Param    body  body  registerRequest  true  "Registration details"
// @Success  200   {object}  authResponse
// @Failure  400   {object}  map[string]string
// @Failure  409   {object}  map[string]string
// @Router   /api/auth/register [post].
func (s *State) Register(writer http.ResponseWriter, request *http.Request) {
	var req registerRequest

	err := decode(request, &req)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	status, msg, ok := s.registerValidationCheck(request, req)
	if !ok {
		s.Error(writer, status, msg)

		return
	}

	ctx := request.Context()

	taken, err := s.Repos.Users.ExistsByEmailOrUsername(ctx, req.Email, req.Username)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	if taken {
		s.Error(writer, http.StatusConflict, "Email or username already taken")

		return
	}

	user, err := s.createUser(ctx, req)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	token, err := middleware.CreateToken(user.ID, user.Email, user.Role, user.Username, s.Config.JWTSecret, s.Config.JWTExpiryH)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Token error")

		return
	}

	addToDefaultGroup(ctx, s.Repos.Groups, user.ID)
	syncGroupEnrollments(ctx, s.Repos.Groups, user.ID)
	metrics.ActiveUsers.Inc()
	s.JSON(writer, http.StatusCreated, authResponse{Token: token, User: user})
}

// loginUserRow holds a local-login lookup result: the user's public
// profile plus their password hash (nil for SSO-only accounts).
type loginUserRow struct {
	user         userPublicRow
	passwordHash *string
}

// loginLookup fetches the local-login row for email, including its
// password hash (nil for SSO-only accounts). Only active accounts match.
func (s *State) loginLookup(ctx context.Context, email string) (loginUserRow, error) {
	user, err := s.Repos.Users.FindByEmailActive(ctx, email)
	if err != nil {
		return loginUserRow{}, fmt.Errorf("find user: %w", err)
	}

	return loginUserRow{user: toUserPublicRow(user), passwordHash: user.PasswordHash}, nil
}

// spendPasswordCheckTime takes about as long as checking a password, for
// the sign-ins that have no hash to check: hashing costs the same as
// comparing.
func spendPasswordCheckTime(password string) {
	_, _ = bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
}

// Login godoc
// @Summary  Authenticate and receive a JWT
// @Tags     Auth
// @Accept   json
// @Produce  json
// @Param    body  body  loginRequest  true  "Login credentials"
// @Success  200   {object}  authResponse
// @Failure  401   {object}  map[string]string
// @Router   /api/auth/login [post].
func (s *State) Login(writer http.ResponseWriter, request *http.Request) {
	var req loginRequest

	err := decode(request, &req)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	ctx := request.Context()

	if repository.ReadSetting(ctx, s.Repos.Settings, settingKeySSOLocalLoginEnabled, authSettingTrue) != authSettingTrue {
		s.Error(writer, http.StatusForbidden, "Local login is disabled. Please sign in using an SSO provider.")

		return
	}

	// An unknown email, an SSO account and a wrong password get the same
	// answer in about the same time, so that the sign-in does not tell
	// which accounts exist (docs/specs/190-backend-defects.md B14).
	row, err := s.loginLookup(ctx, req.Email)
	if err != nil || row.passwordHash == nil {
		spendPasswordCheckTime(req.Password)
		s.Error(writer, http.StatusUnauthorized, "Invalid email or password")

		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(*row.passwordHash), []byte(req.Password))
	if err != nil {
		s.Error(writer, http.StatusUnauthorized, "Invalid email or password")

		return
	}

	token, err := middleware.CreateToken(row.user.ID, row.user.Email, row.user.Role, row.user.Username, s.Config.JWTSecret, s.Config.JWTExpiryH)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, "Token error")

		return
	}

	addToDefaultGroup(ctx, s.Repos.Groups, row.user.ID)
	syncGroupEnrollments(ctx, s.Repos.Groups, row.user.ID)
	s.touchLastLogin(ctx, row.user.ID)
	s.JSON(writer, http.StatusOK, authResponse{Token: token, User: row.user})
}

// Me godoc
// @Summary   Get current user profile
// @Tags      Auth
// @Security  BearerAuth
// @Produce   json
// @Success   200  {object}  userPublicRow
// @Failure   404  {object}  map[string]string
// @Router    /api/auth/me [get].
func (s *State) Me(writer http.ResponseWriter, request *http.Request) {
	claims := s.claims(request)

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "User not found")

		return
	}

	user, err := s.Repos.Users.FindByID(request.Context(), userID)
	if err != nil {
		s.Error(writer, http.StatusNotFound, "User not found")

		return
	}

	s.JSON(writer, http.StatusOK, toUserPublicRow(user))
}

// usernameChangeCheck validates a requested username change against
// platform settings and uniqueness. It returns the HTTP status/message
// to send back when the change must be rejected, or ok == true to
// proceed.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func (s *State) usernameChangeCheck(ctx context.Context, uname, currentUserID string) (int, string, bool) {
	if repository.ReadSetting(ctx, s.Repos.Settings, "profile_allow_username_change", authSettingTrue) != authSettingTrue {
		return http.StatusForbidden, "Username changes are not allowed.", false
	}

	if len(uname) < authMinUsernameLen {
		return http.StatusBadRequest, authMsgUsernameTooShort, false
	}

	id, err := uuid.Parse(currentUserID)
	if err == nil {
		taken, _ := s.Repos.Users.ExistsUsernameExcluding(ctx, uname, id)
		if taken {
			return http.StatusConflict, "Username already taken", false
		}
	}

	return http.StatusOK, "", true
}

// UpdateProfile godoc
// @Summary   Update current user profile
// @Tags      Auth
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  object  true  "username, bio, avatarUrl (all optional)"
// @Success   200   {object}  userPublicRow
// @Failure   400   {object}  map[string]string
// @Router    /api/auth/profile [put].
func (s *State) UpdateProfile(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Username *string `json:"username"`
		Bio      *string `json:"bio"`

		AvatarURL *string `json:"avatarUrl"`
	}

	err := decode(request, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	ctx := request.Context()
	claims := s.claims(request)

	if body.Username != nil {
		uname := strings.TrimSpace(*body.Username)
		if uname != "" {
			status, msg, ok := s.usernameChangeCheck(ctx, uname, claims.Subject)
			if !ok {
				s.Error(writer, status, msg)

				return
			}
		}
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	user, err := s.Repos.Users.UpdateProfile(ctx, userID, body.Username, body.Bio, body.AvatarURL)
	if err != nil {
		s.Error(writer, http.StatusInternalServerError, errDatabase)

		return
	}

	s.JSON(writer, http.StatusOK, toUserPublicRow(user))
}

// verifyCurrentPassword checks that candidate matches userID's stored
// password hash. It returns the HTTP status/message to send back when
// verification fails, or ok == true to proceed.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func (s *State) verifyCurrentPassword(ctx context.Context, userID, candidate string) (int, string, bool) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return http.StatusBadRequest, errNoLocalPassword, false
	}

	passwordHash, err := s.Repos.Users.GetPasswordHash(ctx, id)
	if err != nil || passwordHash == nil {
		return http.StatusBadRequest, errNoLocalPassword, false
	}

	err = bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte(candidate))
	if err != nil {
		return http.StatusUnauthorized, "Incorrect current password", false
	}

	return http.StatusOK, "", true
}

// ChangePassword godoc
// @Summary   Change the current user's password
// @Tags      Auth
// @Security  BearerAuth
// @Accept    json
// @Produce   json
// @Param     body  body  object  true  "oldPassword and newPassword"
// @Success   200   {object}  map[string]string
// @Failure   400   {object}  map[string]string
// @Failure   401   {object}  map[string]string
// @Router    /api/auth/password [put].
func (s *State) ChangePassword(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}

	err := decode(request, &body)
	if err != nil {
		s.Error(writer, http.StatusBadRequest, "Invalid JSON")

		return
	}

	if body.OldPassword == "" {
		s.Error(writer, http.StatusBadRequest, "oldPassword required")

		return
	}

	if body.NewPassword == "" {
		s.Error(writer, http.StatusBadRequest, "newPassword required")

		return
	}

	if msg, ok := s.validatePasswordPolicy(request, body.NewPassword); !ok {
		s.Error(writer, http.StatusBadRequest, msg)

		return
	}

	ctx := request.Context()
	claims := s.claims(request)

	status, msg, verified := s.verifyCurrentPassword(ctx, claims.Subject, body.OldPassword)
	if !verified {
		s.Error(writer, status, msg)

		return
	}

	status, msg, applied := s.applyNewPassword(ctx, claims.Subject, body.NewPassword)
	if !applied {
		s.Error(writer, status, msg)

		return
	}

	// A new password ends the sessions opened with the old one, this one
	// included (docs/specs/190-backend-defects.md B7).
	userID, _ := uuid.Parse(claims.Subject) // parsed by applyNewPassword already

	err = s.revokeSessions(ctx, userID)
	if err != nil {
		zap.L().Error("revoke sessions after password change", zap.Error(err))
	}

	s.JSON(writer, http.StatusOK, map[string]string{groupsRespKeyMessage: "Password changed successfully"})
}

// applyNewPassword hashes newPassword and persists it for the user
// identified by subject. It returns the HTTP status/message to send back
// when the update fails, or ok == true to proceed.
//
//nolint:gocritic // named results here would trip nonamedreturns instead; see doc comment above for the meaning of each value
func (s *State) applyNewPassword(ctx context.Context, subject, newPassword string) (int, string, bool) {
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return http.StatusInternalServerError, "Hash error", false
	}

	userID, err := uuid.Parse(subject)
	if err != nil {
		return http.StatusInternalServerError, errDatabase, false
	}

	err = s.Repos.Users.UpdatePasswordHash(ctx, userID, string(newHash))
	if err != nil {
		return http.StatusInternalServerError, errDatabase, false
	}

	return 0, "", true
}

// touchLastLogin updates lastloginat for the user identified by userIDStr.
// Failures are logged at warn level only — a login should not fail because
// of a best-effort timestamp update.
func (s *State) touchLastLogin(ctx context.Context, userIDStr string) {
	id, err := uuid.Parse(userIDStr)
	if err != nil {
		return
	}

	touchErr := s.Repos.Users.TouchLastLogin(ctx, id)
	if touchErr != nil {
		zap.L().Warn("touch last login", zap.Error(touchErr))
	}
}
