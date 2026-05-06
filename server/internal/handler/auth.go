package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/siddhant2408/nimbus/internal/auth"
	"github.com/siddhant2408/nimbus/internal/logger"
	db "github.com/siddhant2408/nimbus/pkg/db/generated"
)

// SignupError represents signup restriction errors
type SignupError struct {
	Message string
}

func (e SignupError) Error() string {
	return e.Message
}

var ErrSignupProhibited = SignupError{Message: "user registration is disabled on this self-hosted instance"}
var ErrEmailNotAllowed = SignupError{Message: "email address or domain not allowed on this instance"}

const devVerificationCodeEnv = "MULTICA_DEV_VERIFICATION_CODE"

type UserResponse struct {
	ID                      string          `json:"id"`
	Name                    string          `json:"name"`
	Email                   string          `json:"email"`
	AvatarURL               *string         `json:"avatar_url"`
	OnboardedAt             *string         `json:"onboarded_at"`
	OnboardingQuestionnaire json.RawMessage `json:"onboarding_questionnaire"`
	StarterContentState     *string         `json:"starter_content_state"`
	CreatedAt               string          `json:"created_at"`
	UpdatedAt               string          `json:"updated_at"`
}

func userToResponse(u db.User) UserResponse {
	// JSONB column is []byte with DEFAULT '{}', so it's never nil at the DB
	// level. Defensive coalesce just in case a future ALTER makes the column
	// nullable and some row comes back with no default applied.
	q := u.OnboardingQuestionnaire
	if len(q) == 0 {
		q = []byte("{}")
	}
	return UserResponse{
		ID:                      uuidToString(u.ID),
		Name:                    u.Name,
		Email:                   u.Email,
		AvatarURL:               textToPtr(u.AvatarUrl),
		OnboardedAt:             timestampToPtr(u.OnboardedAt),
		OnboardingQuestionnaire: json.RawMessage(q),
		StarterContentState:     textToPtr(u.StarterContentState),
		CreatedAt:               timestampToString(u.CreatedAt),
		UpdatedAt:               timestampToString(u.UpdatedAt),
	}
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type SendCodeRequest struct {
	Email string `json:"email"`
}

type VerifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func generateCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(buf[:]) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func isDevVerificationCode(code string) bool {
	if isProductionEnv() {
		return false
	}

	devCode := strings.TrimSpace(os.Getenv(devVerificationCodeEnv))
	if !isSixDigitCode(devCode) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(code), []byte(devCode)) == 1
}

func isProductionEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func isSixDigitCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (h *Handler) issueJWT(user db.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   uuidToString(user.ID),
		"email": user.Email,
		"name":  user.Name,
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	return token.SignedString(auth.JWTSecret())
}

// findOrCreateUser returns the existing user for an email, or creates one if
// none exists. isNew reports whether this call created the user — the signup
// event fires on that edge, covering both the verification-code and Google
// OAuth entry points.
func (h *Handler) findOrCreateUser(ctx context.Context, email string) (user db.User, isNew bool, err error) {
	user, err = h.Queries.GetUserByEmail(ctx, email)
	isNew = isNotFound(err)
	if err != nil && !isNew {
		return db.User{}, false, err
	}

	if !isNew {
		return user, false, nil
	}

	name := email
	if at := strings.Index(email, "@"); at > 0 {
		name = email[:at]
	}
	created, err := h.Queries.CreateUser(ctx, db.CreateUserParams{
		Name:  name,
		Email: email,
	})
	if err != nil {
		return db.User{}, false, err
	}
	return created, true, nil
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

type UpdateMeRequest struct {
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

type userInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req userInfo
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "No email provided")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))

	user, _, err := h.findOrCreateUser(r.Context(), email)
	if err != nil {
		var signupErr SignupError
		if errors.As(err, &signupErr) {
			writeError(w, http.StatusForbidden, signupErr.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("google login failed", append(logger.RequestAttrs(r), "error", err, "email", email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	if err := auth.SetAuthCookies(w, tokenString); err != nil {
		slog.Warn("failed to set auth cookies", "error", err)
	}

	slog.Info("user logged in", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}

// IssueCliToken returns a fresh JWT for the authenticated user.
// This allows cookie-authenticated browser sessions to obtain a bearer token
// that can be handed off to the CLI via the cli_callback redirect.
func (h *Handler) IssueCliToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("cli-token: failed to issue JWT", append(logger.RequestAttrs(r), "error", err, "user_id", userID)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": tokenString})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearAuthCookies(w)
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req UpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	currentUser, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	name := currentUser.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
	}

	params := db.UpdateUserParams{
		ID:   currentUser.ID,
		Name: name,
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: strings.TrimSpace(*req.AvatarURL), Valid: true}
	}

	updatedUser, err := h.Queries.UpdateUser(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(updatedUser))
}
