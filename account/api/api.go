package api

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"xcontrol/account/internal/store"
)

const defaultSessionTTL = 24 * time.Hour
const defaultMFAChallengeTTL = 10 * time.Minute
const defaultTOTPIssuer = "XControl Account"
const defaultEmailVerificationTTL = 24 * time.Hour
const defaultPasswordResetTTL = 30 * time.Minute
const maxMFAVerificationAttempts = 5
const defaultMFALockoutDuration = 5 * time.Minute

type session struct {
	userID    string
	expiresAt time.Time
}

type handler struct {
	store                    store.Store
	sessions                 map[string]session
	mu                       sync.RWMutex
	sessionTTL               time.Duration
	mfaChallenges            map[string]mfaChallenge
	mfaMu                    sync.RWMutex
	mfaChallengeTTL          time.Duration
	totpIssuer               string
	emailSender              EmailSender
	emailVerificationEnabled bool
	verificationTTL          time.Duration
	verifications            map[string]emailVerification
	verificationMu           sync.RWMutex
	resetTTL                 time.Duration
	passwordResets           map[string]passwordReset
	resetMu                  sync.RWMutex
}

type mfaChallenge struct {
	userID         string
	expiresAt      time.Time
	totpSecret     string
	totpIssuer     string
	totpAccount    string
	totpIssuedAt   time.Time
	failedAttempts int
	lockedUntil    time.Time
}

type emailVerification struct {
	userID    string
	email     string
	expiresAt time.Time
}

type passwordReset struct {
	userID    string
	email     string
	expiresAt time.Time
}

// Option configures handler behaviour when registering routes.
type Option func(*handler)

// WithStore overrides the default in-memory store with the provided implementation.
func WithStore(st store.Store) Option {
	return func(h *handler) {
		if st != nil {
			h.store = st
		}
	}
}

// WithSessionTTL sets the TTL used for issued sessions.
func WithSessionTTL(ttl time.Duration) Option {
	return func(h *handler) {
		if ttl > 0 {
			h.sessionTTL = ttl
		}
	}
}

// WithEmailSender configures the handler to use the provided EmailSender for outbound notifications.
func WithEmailSender(sender EmailSender) Option {
	return func(h *handler) {
		if sender != nil {
			h.emailSender = sender
		}
	}
}

// WithEmailVerification configures whether user registration requires email verification.
func WithEmailVerification(enabled bool) Option {
	return func(h *handler) {
		h.emailVerificationEnabled = enabled
	}
}

// WithEmailVerificationTTL overrides the default TTL for email verification tokens.
func WithEmailVerificationTTL(ttl time.Duration) Option {
	return func(h *handler) {
		if ttl > 0 {
			h.verificationTTL = ttl
		}
	}
}

// WithPasswordResetTTL overrides the default TTL for password reset tokens.
func WithPasswordResetTTL(ttl time.Duration) Option {
	return func(h *handler) {
		if ttl > 0 {
			h.resetTTL = ttl
		}
	}
}

// RegisterRoutes attaches account service endpoints to the router.
func RegisterRoutes(r *gin.Engine, opts ...Option) {
	h := &handler{
		store:                    store.NewMemoryStore(),
		sessions:                 make(map[string]session),
		sessionTTL:               defaultSessionTTL,
		mfaChallenges:            make(map[string]mfaChallenge),
		mfaChallengeTTL:          defaultMFAChallengeTTL,
		totpIssuer:               defaultTOTPIssuer,
		emailSender:              noopEmailSender,
		emailVerificationEnabled: true,
		verificationTTL:          defaultEmailVerificationTTL,
		verifications:            make(map[string]emailVerification),
		resetTTL:                 defaultPasswordResetTTL,
		passwordResets:           make(map[string]passwordReset),
	}

	for _, opt := range opts {
		opt(h)
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	auth := r.Group("/api/auth")
	auth.POST("/register", h.register)
	auth.POST("/register/verify", h.verifyEmail)
	auth.POST("/login", h.login)
	auth.GET("/session", h.session)
	auth.DELETE("/session", h.deleteSession)
	auth.POST("/mfa/totp/provision", h.provisionTOTP)
	auth.POST("/mfa/totp/verify", h.verifyTOTP)
	auth.POST("/mfa/disable", h.disableMFA)
	auth.GET("/mfa/status", h.mfaStatus)
	auth.POST("/password/reset", h.requestPasswordReset)
	auth.POST("/password/reset/confirm", h.confirmPasswordReset)
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	TOTPCode   string `json:"totpCode"`
}

type tokenRequest struct {
	Token string `json:"token"`
}

type passwordResetRequestBody struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func hasQueryParameter(c *gin.Context, keys ...string) bool {
	if len(keys) == 0 {
		return false
	}

	values := c.Request.URL.Query()
	for _, key := range keys {
		if _, ok := values[key]; ok {
			return true
		}
	}

	return false
}

func (h *handler) register(c *gin.Context) {
	if hasQueryParameter(c, "password", "email", "confirmPassword") {
		respondError(c, http.StatusBadRequest, "credentials_in_query", "sensitive credentials must not be sent in the query string")
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)

	if name == "" {
		respondError(c, http.StatusBadRequest, "name_required", "name is required")
		return
	}

	if email == "" || password == "" {
		respondError(c, http.StatusBadRequest, "missing_credentials", "email and password are required")
		return
	}

	if !strings.Contains(email, "@") {
		respondError(c, http.StatusBadRequest, "invalid_email", "email must be a valid address")
		return
	}

	if len(password) < 8 {
		respondError(c, http.StatusBadRequest, "password_too_short", "password must be at least 8 characters")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "hash_failure", "failed to secure password")
		return
	}

	user := &store.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hashed),
	}

	if !h.emailVerificationEnabled {
		user.EmailVerified = true
	}

	if err := h.store.CreateUser(c.Request.Context(), user); err != nil {
		switch {
		case errors.Is(err, store.ErrEmailExists):
			respondError(c, http.StatusConflict, "email_already_exists", "user with this email already exists")
			return
		case errors.Is(err, store.ErrNameExists):
			respondError(c, http.StatusConflict, "name_already_exists", "user with this name already exists")
			return
		case errors.Is(err, store.ErrInvalidName):
			respondError(c, http.StatusBadRequest, "invalid_name", "name is invalid")
			return
		default:
			respondError(c, http.StatusInternalServerError, "user_creation_failed", "failed to create user")
			return
		}
	}

	message := "registration successful"
	if h.emailVerificationEnabled {
		if err := h.enqueueEmailVerification(c.Request.Context(), user); err != nil {
			slog.Error("failed to send verification email", "err", err, "email", user.Email)
			respondError(c, http.StatusInternalServerError, "verification_email_failed", "failed to send verification email")
			return
		}
		message = "verification email sent"
	}

	response := gin.H{
		"message": message,
		"user":    sanitizeUser(user, nil),
	}
	c.JSON(http.StatusCreated, response)
}

func (h *handler) verifyEmail(c *gin.Context) {
	if hasQueryParameter(c, "token") {
		respondError(c, http.StatusBadRequest, "token_in_query", "verification token must be sent in the request body")
		return
	}

	var req tokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		respondError(c, http.StatusBadRequest, "invalid_token", "verification token is required")
		return
	}

	verification, ok := h.lookupEmailVerification(token)
	if !ok {
		respondError(c, http.StatusBadRequest, "invalid_token", "verification token is invalid or expired")
		return
	}

	user, err := h.store.GetUserByID(c.Request.Context(), verification.userID)
	if err != nil {
		slog.Error("failed to load user for email verification", "err", err, "userID", verification.userID)
		respondError(c, http.StatusInternalServerError, "verification_failed", "failed to verify email")
		return
	}

	if !strings.EqualFold(strings.TrimSpace(user.Email), verification.email) {
		h.removeEmailVerification(token)
		respondError(c, http.StatusBadRequest, "invalid_token", "verification token is invalid or expired")
		return
	}

	if !user.EmailVerified {
		user.EmailVerified = true
		if err := h.store.UpdateUser(c.Request.Context(), user); err != nil {
			slog.Error("failed to update user during email verification", "err", err, "userID", user.ID)
			respondError(c, http.StatusInternalServerError, "verification_failed", "failed to verify email")
			return
		}
	}

	h.removeEmailVerification(token)

	sessionToken, expiresAt, err := h.createSession(user.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "session_creation_failed", "failed to create session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "email verified",
		"token":     sessionToken,
		"expiresAt": expiresAt.UTC(),
		"user":      sanitizeUser(user, nil),
	})
}

func (h *handler) requestPasswordReset(c *gin.Context) {
	if hasQueryParameter(c, "email") {
		respondError(c, http.StatusBadRequest, "email_in_query", "email must be sent in the request body")
		return
	}

	var req passwordResetRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		respondError(c, http.StatusBadRequest, "email_required", "email is required")
		return
	}

	user, err := h.store.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			c.JSON(http.StatusAccepted, gin.H{"message": "if the account exists a reset email will be sent"})
			return
		}
		respondError(c, http.StatusInternalServerError, "password_reset_failed", "failed to initiate password reset")
		return
	}

	if strings.TrimSpace(user.Email) == "" || !user.EmailVerified {
		c.JSON(http.StatusAccepted, gin.H{"message": "if the account exists a reset email will be sent"})
		return
	}

	if err := h.enqueuePasswordReset(c.Request.Context(), user); err != nil {
		slog.Error("failed to send password reset email", "err", err, "email", user.Email)
		respondError(c, http.StatusInternalServerError, "password_reset_failed", "failed to initiate password reset")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "if the account exists a reset email will be sent"})
}

func (h *handler) confirmPasswordReset(c *gin.Context) {
	if hasQueryParameter(c, "token", "password") {
		respondError(c, http.StatusBadRequest, "credentials_in_query", "sensitive credentials must not be sent in the query string")
		return
	}

	var req passwordResetConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	token := strings.TrimSpace(req.Token)
	password := strings.TrimSpace(req.Password)

	if token == "" || password == "" {
		respondError(c, http.StatusBadRequest, "invalid_request", "token and password are required")
		return
	}

	if len(password) < 8 {
		respondError(c, http.StatusBadRequest, "password_too_short", "password must be at least 8 characters")
		return
	}

	reset, ok := h.lookupPasswordReset(token)
	if !ok {
		respondError(c, http.StatusBadRequest, "invalid_token", "reset token is invalid or expired")
		return
	}

	user, err := h.store.GetUserByID(c.Request.Context(), reset.userID)
	if err != nil {
		slog.Error("failed to load user for password reset", "err", err, "userID", reset.userID)
		respondError(c, http.StatusInternalServerError, "password_reset_failed", "failed to reset password")
		return
	}

	if !strings.EqualFold(strings.TrimSpace(user.Email), reset.email) {
		h.removePasswordReset(token)
		respondError(c, http.StatusBadRequest, "invalid_token", "reset token is invalid or expired")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "password_reset_failed", "failed to reset password")
		return
	}

	user.PasswordHash = string(hashed)
	user.EmailVerified = true
	if err := h.store.UpdateUser(c.Request.Context(), user); err != nil {
		slog.Error("failed to update user during password reset", "err", err, "userID", user.ID)
		respondError(c, http.StatusInternalServerError, "password_reset_failed", "failed to reset password")
		return
	}

	h.removePasswordReset(token)

	sessionToken, expiresAt, err := h.createSession(user.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "session_creation_failed", "failed to create session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "password reset successful",
		"token":     sessionToken,
		"expiresAt": expiresAt.UTC(),
		"user":      sanitizeUser(user, nil),
	})
}

func (h *handler) login(c *gin.Context) {
	if hasQueryParameter(c, "username", "password", "identifier", "totp") {
		respondError(c, http.StatusBadRequest, "credentials_in_query", "sensitive credentials must not be sent in the query string")
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Username)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}

	password := strings.TrimSpace(req.Password)
	totpCode := strings.TrimSpace(req.TOTPCode)

	if identifier == "" {
		respondError(c, http.StatusBadRequest, "missing_credentials", "identifier is required")
		return
	}

	user, err := h.findUserByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			respondError(c, http.StatusNotFound, "user_not_found", "user not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "authentication_failed", "failed to authenticate user")
		return
	}

	if password != "" {
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			respondError(c, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}
	} else {
		if totpCode == "" {
			respondError(c, http.StatusBadRequest, "missing_credentials", "totp code is required")
			return
		}
		if !strings.EqualFold(strings.TrimSpace(user.Email), identifier) {
			respondError(c, http.StatusUnauthorized, "password_required", "password required for this identifier")
			return
		}
	}

	if strings.TrimSpace(user.Email) != "" && !user.EmailVerified {
		respondError(c, http.StatusUnauthorized, "email_not_verified", "email must be verified before login")
		return
	}

	if user.MFAEnabled {
		if totpCode == "" {
			respondError(c, http.StatusBadRequest, "mfa_code_required", "totp code is required")
			return
		}

		valid, err := totp.ValidateCustom(totpCode, user.MFATOTPSecret, time.Now().UTC(), totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, "invalid_mfa_code", "invalid totp code")
			return
		}
		if !valid {
			respondError(c, http.StatusUnauthorized, "invalid_mfa_code", "invalid totp code")
			return
		}

		token, expiresAt, err := h.createSession(user.ID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "session_creation_failed", "failed to create session")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "login successful",
			"token":     token,
			"expiresAt": expiresAt.UTC(),
			"user":      sanitizeUser(user, nil),
		})
		return
	}

	token, expiresAt, err := h.createSession(user.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "session_creation_failed", "failed to create session")
		return
	}

	response := gin.H{
		"message":   "login successful",
		"token":     token,
		"expiresAt": expiresAt.UTC(),
		"user":      sanitizeUser(user, nil),
	}

	if challengeToken, err := h.createMFAChallenge(user.ID); err != nil {
		slog.Error("failed to create mfa challenge during login", "err", err, "userID", user.ID)
	} else {
		response["mfaToken"] = challengeToken
	}

	c.JSON(http.StatusOK, response)
}

func (h *handler) findUserByIdentifier(ctx context.Context, identifier string) (*store.User, error) {
	user, err := h.store.GetUserByName(ctx, identifier)
	if err == nil {
		return user, nil
	}
	if err != nil && !errors.Is(err, store.ErrUserNotFound) {
		return nil, err
	}
	return h.store.GetUserByEmail(ctx, identifier)
}

func (h *handler) session(c *gin.Context) {
	token := extractToken(c.GetHeader("Authorization"))
	if token == "" {
		if value := c.Query("token"); value != "" {
			token = value
		}
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session token required"})
		return
	}

	sess, ok := h.lookupSession(token)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session not found"})
		return
	}

	user, err := h.store.GetUserByID(c.Request.Context(), sess.userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load session user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user, nil)})
}

func (h *handler) deleteSession(c *gin.Context) {
	token := extractToken(c.GetHeader("Authorization"))
	if token == "" {
		if value := c.Query("token"); value != "" {
			token = value
		}
	}
	if token == "" {
		c.Status(http.StatusNoContent)
		return
	}

	h.removeSession(token)
	c.Status(http.StatusNoContent)
}

func (h *handler) createSession(userID string) (string, time.Time, error) {
	token, err := h.newRandomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	ttl := h.sessionTTL
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	expiresAt := time.Now().Add(ttl)

	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[token] = session{userID: userID, expiresAt: expiresAt}
	return token, expiresAt, nil
}

func (h *handler) lookupSession(token string) (session, bool) {
	h.mu.RLock()
	sess, ok := h.sessions[token]
	h.mu.RUnlock()
	if !ok {
		return session{}, false
	}
	if time.Now().After(sess.expiresAt) {
		h.removeSession(token)
		return session{}, false
	}
	return sess, true
}

func (h *handler) removeSession(token string) {
	h.mu.Lock()
	delete(h.sessions, token)
	h.mu.Unlock()
}

func (h *handler) newRandomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func (h *handler) effectiveMFAChallengeTTL() time.Duration {
	ttl := h.mfaChallengeTTL
	if ttl <= 0 {
		ttl = defaultMFAChallengeTTL
	}
	return ttl
}

func clearMFAChallenge(ch *mfaChallenge) {
	if ch == nil {
		return
	}
	ch.totpSecret = ""
	ch.totpIssuer = ""
	ch.totpAccount = ""
	ch.totpIssuedAt = time.Time{}
	ch.failedAttempts = 0
	ch.lockedUntil = time.Time{}
}

func (h *handler) updateMFAChallenge(token string, update func(*mfaChallenge) bool) (mfaChallenge, bool) {
	h.mfaMu.Lock()
	defer h.mfaMu.Unlock()

	challenge, ok := h.mfaChallenges[token]
	if !ok {
		return mfaChallenge{}, false
	}

	if time.Now().After(challenge.expiresAt) {
		clearMFAChallenge(&challenge)
		delete(h.mfaChallenges, token)
		return mfaChallenge{}, false
	}

	if update != nil {
		if !update(&challenge) {
			clearMFAChallenge(&challenge)
			delete(h.mfaChallenges, token)
			return mfaChallenge{}, false
		}
		h.mfaChallenges[token] = challenge
	}

	return challenge, true
}

func (h *handler) createMFAChallenge(userID string) (string, error) {
	token, err := h.newRandomToken()
	if err != nil {
		return "", err
	}
	ttl := h.effectiveMFAChallengeTTL()
	challenge := mfaChallenge{userID: userID, expiresAt: time.Now().Add(ttl)}
	h.mfaMu.Lock()
	h.mfaChallenges[token] = challenge
	h.mfaMu.Unlock()
	return token, nil
}

func (h *handler) lookupMFAChallenge(token string) (mfaChallenge, bool) {
	return h.updateMFAChallenge(token, nil)
}

func (h *handler) refreshMFAChallenge(token string) (mfaChallenge, bool) {
	ttl := h.effectiveMFAChallengeTTL()
	return h.updateMFAChallenge(token, func(ch *mfaChallenge) bool {
		ch.expiresAt = time.Now().Add(ttl)
		return true
	})
}

func (h *handler) enqueueEmailVerification(ctx context.Context, user *store.User) error {
	email := strings.TrimSpace(user.Email)
	if email == "" {
		return errors.New("user email is empty")
	}

	token, err := h.newRandomToken()
	if err != nil {
		return err
	}

	ttl := h.verificationTTL
	if ttl <= 0 {
		ttl = defaultEmailVerificationTTL
	}

	expiresAt := time.Now().Add(ttl)
	verification := emailVerification{
		userID:    user.ID,
		email:     strings.ToLower(email),
		expiresAt: expiresAt,
	}

	h.verificationMu.Lock()
	h.verifications[token] = verification
	h.verificationMu.Unlock()

	name := strings.TrimSpace(user.Name)
	if name == "" {
		name = "there"
	}

	subject := "Verify your XControl account"
	plainBody := fmt.Sprintf("Hello %s,\n\nUse the following token to verify your XControl account: %s\n\nThis token expires at %s UTC.\nIf you did not request this email you can ignore it.\n", name, token, expiresAt.UTC().Format(time.RFC3339))
	htmlBody := fmt.Sprintf("<p>Hello %s,</p><p>Use the following token to verify your XControl account:</p><p><strong>%s</strong></p><p>This token expires at %s UTC.</p><p>If you did not request this email you can ignore it.</p>", html.EscapeString(name), token, expiresAt.UTC().Format(time.RFC3339))

	msg := EmailMessage{
		To:        []string{email},
		Subject:   subject,
		PlainBody: plainBody,
		HTMLBody:  htmlBody,
	}

	if err := h.emailSender.Send(ctx, msg); err != nil {
		h.removeEmailVerification(token)
		return err
	}

	return nil
}

func (h *handler) lookupEmailVerification(token string) (emailVerification, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return emailVerification{}, false
	}

	h.verificationMu.RLock()
	verification, ok := h.verifications[token]
	h.verificationMu.RUnlock()
	if !ok {
		return emailVerification{}, false
	}

	if time.Now().After(verification.expiresAt) {
		h.removeEmailVerification(token)
		return emailVerification{}, false
	}

	return verification, true
}

func (h *handler) removeEmailVerification(token string) {
	h.verificationMu.Lock()
	delete(h.verifications, strings.TrimSpace(token))
	h.verificationMu.Unlock()
}

func (h *handler) enqueuePasswordReset(ctx context.Context, user *store.User) error {
	email := strings.TrimSpace(user.Email)
	if email == "" {
		return errors.New("user email is empty")
	}

	token, err := h.newRandomToken()
	if err != nil {
		return err
	}

	ttl := h.resetTTL
	if ttl <= 0 {
		ttl = defaultPasswordResetTTL
	}

	expiresAt := time.Now().Add(ttl)
	reset := passwordReset{
		userID:    user.ID,
		email:     strings.ToLower(email),
		expiresAt: expiresAt,
	}

	h.resetMu.Lock()
	h.passwordResets[token] = reset
	h.resetMu.Unlock()

	name := strings.TrimSpace(user.Name)
	if name == "" {
		name = "there"
	}

	subject := "Reset your XControl password"
	plainBody := fmt.Sprintf("Hello %s,\n\nUse the following token to reset your XControl account password: %s\n\nThis token expires at %s UTC.\nIf you did not request a reset you can ignore this email.\n", name, token, expiresAt.UTC().Format(time.RFC3339))
	htmlBody := fmt.Sprintf("<p>Hello %s,</p><p>Use the following token to reset your XControl account password:</p><p><strong>%s</strong></p><p>This token expires at %s UTC.</p><p>If you did not request a reset you can ignore this email.</p>", html.EscapeString(name), token, expiresAt.UTC().Format(time.RFC3339))

	msg := EmailMessage{
		To:        []string{email},
		Subject:   subject,
		PlainBody: plainBody,
		HTMLBody:  htmlBody,
	}

	if err := h.emailSender.Send(ctx, msg); err != nil {
		h.removePasswordReset(token)
		return err
	}

	return nil
}

func (h *handler) lookupPasswordReset(token string) (passwordReset, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return passwordReset{}, false
	}

	h.resetMu.RLock()
	reset, ok := h.passwordResets[token]
	h.resetMu.RUnlock()
	if !ok {
		return passwordReset{}, false
	}

	if time.Now().After(reset.expiresAt) {
		h.removePasswordReset(token)
		return passwordReset{}, false
	}

	return reset, true
}

func (h *handler) removePasswordReset(token string) {
	h.resetMu.Lock()
	delete(h.passwordResets, strings.TrimSpace(token))
	h.resetMu.Unlock()
}

func (h *handler) removeMFAChallenge(token string) {
	h.mfaMu.Lock()
	if challenge, ok := h.mfaChallenges[token]; ok {
		clearMFAChallenge(&challenge)
		delete(h.mfaChallenges, token)
	}
	h.mfaMu.Unlock()
}

func (h *handler) removeMFAChallengesForUser(userID string) {
	if userID == "" {
		return
	}
	h.mfaMu.Lock()
	for token, challenge := range h.mfaChallenges {
		if challenge.userID == userID {
			clearMFAChallenge(&challenge)
			delete(h.mfaChallenges, token)
		}
	}
	h.mfaMu.Unlock()
}

func (h *handler) provisionTOTP(c *gin.Context) {
	var req struct {
		Token   string `json:"token"`
		Issuer  string `json:"issuer"`
		Account string `json:"account"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		respondError(c, http.StatusBadRequest, "mfa_token_required", "mfa token is required")
		return
	}

	challenge, ok := h.refreshMFAChallenge(token)
	if !ok {
		respondError(c, http.StatusUnauthorized, "invalid_mfa_token", "mfa token is invalid or expired")
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByID(ctx, challenge.userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "mfa_user_lookup_failed", "failed to load user for mfa provisioning")
		return
	}

	if user.MFAEnabled {
		respondError(c, http.StatusBadRequest, "mfa_already_enabled", "mfa already enabled for this account")
		return
	}

	issuer := strings.TrimSpace(req.Issuer)
	if issuer == "" {
		issuer = strings.TrimSpace(h.totpIssuer)
		if issuer == "" {
			issuer = defaultTOTPIssuer
		}
	}

	accountName := strings.TrimSpace(req.Account)
	if accountName == "" {
		accountName = deriveDefaultAccountLabel(user, issuer)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "mfa_secret_generation_failed", "failed to generate totp secret")
		return
	}

	issuedAt := time.Now().UTC()
	ttl := h.effectiveMFAChallengeTTL()
	pendingChallenge, ok := h.updateMFAChallenge(token, func(ch *mfaChallenge) bool {
		if ch.userID != user.ID {
			return false
		}
		ch.totpSecret = key.Secret()
		ch.totpIssuer = issuer
		ch.totpAccount = accountName
		ch.totpIssuedAt = issuedAt
		ch.failedAttempts = 0
		ch.lockedUntil = time.Time{}
		ch.expiresAt = time.Now().Add(ttl)
		return true
	})
	if !ok {
		respondError(c, http.StatusUnauthorized, "invalid_mfa_token", "mfa token is invalid or expired")
		return
	}

	state := buildMFAState(user, &pendingChallenge)
	sanitized := sanitizeUser(user, &pendingChallenge)
	c.JSON(http.StatusOK, gin.H{
		"secret":      key.Secret(),
		"otpauth_url": key.URL(),
		"issuer":      issuer,
		"account":     accountName,
		"mfaToken":    token,
		"mfa":         state,
		"user":        sanitized,
	})
}

func deriveDefaultAccountLabel(user *store.User, issuer string) string {
	if user == nil {
		if issuer == "" {
			return "account"
		}
		return fmt.Sprintf("%s account", issuer)
	}

	identifier := strings.TrimSpace(user.ID)
	if identifier == "" {
		if issuer == "" {
			return "account"
		}
		return fmt.Sprintf("%s account", issuer)
	}

	sum := sha1.Sum([]byte(identifier))
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(sum[:]))
	if len(encoded) > 10 {
		encoded = encoded[:10]
	}
	return fmt.Sprintf("user-%s", encoded)
}

func (h *handler) verifyTOTP(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		respondError(c, http.StatusBadRequest, "mfa_token_required", "mfa token is required")
		return
	}

	challenge, ok := h.lookupMFAChallenge(token)
	if !ok {
		respondError(c, http.StatusUnauthorized, "invalid_mfa_token", "mfa token is invalid or expired")
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByID(ctx, challenge.userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "mfa_user_lookup_failed", "failed to load user for verification")
		return
	}

	challenge, ok = h.updateMFAChallenge(token, func(ch *mfaChallenge) bool {
		if ch.userID != user.ID {
			return false
		}
		ch.expiresAt = time.Now().Add(h.effectiveMFAChallengeTTL())
		return true
	})
	if !ok {
		respondError(c, http.StatusUnauthorized, "invalid_mfa_token", "mfa token is invalid or expired")
		return
	}

	now := time.Now()
	if !challenge.lockedUntil.IsZero() && now.Before(challenge.lockedUntil) {
		retryAt := challenge.lockedUntil.UTC()
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":    "mfa_challenge_locked",
			"message":  "too many invalid mfa attempts, try again later",
			"retryAt":  retryAt,
			"mfaToken": token,
		})
		return
	}

	secret := strings.TrimSpace(user.MFATOTPSecret)
	if secret == "" {
		secret = strings.TrimSpace(challenge.totpSecret)
	}
	if secret == "" {
		respondError(c, http.StatusBadRequest, "mfa_secret_missing", "mfa secret has not been provisioned")
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		respondError(c, http.StatusBadRequest, "mfa_code_required", "totp code is required")
		return
	}

	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "invalid_mfa_code", "invalid totp code")
		return
	}
	if !valid {
		ttl := h.effectiveMFAChallengeTTL()
		updatedChallenge, ok := h.updateMFAChallenge(token, func(ch *mfaChallenge) bool {
			if ch.userID != user.ID {
				return false
			}
			if now.Before(ch.lockedUntil) {
				return true
			}
			ch.failedAttempts++
			if ch.failedAttempts >= maxMFAVerificationAttempts {
				ch.failedAttempts = 0
				ch.lockedUntil = now.Add(defaultMFALockoutDuration)
			}
			ch.expiresAt = time.Now().Add(ttl)
			return true
		})
		if ok {
			challenge = updatedChallenge
		}

		if !challenge.lockedUntil.IsZero() && now.Before(challenge.lockedUntil) {
			retryAt := challenge.lockedUntil.UTC()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":    "mfa_challenge_locked",
				"message":  "too many invalid mfa attempts, try again later",
				"retryAt":  retryAt,
				"mfaToken": token,
			})
			return
		}

		respondError(c, http.StatusUnauthorized, "invalid_mfa_code", "invalid totp code")
		return
	}

	confirmationTime := time.Now().UTC()
	issuedAt := challenge.totpIssuedAt
	if issuedAt.IsZero() {
		issuedAt = confirmationTime
	}

	if strings.TrimSpace(user.MFATOTPSecret) == "" {
		user.MFATOTPSecret = secret
	}
	if user.MFASecretIssuedAt.IsZero() {
		user.MFASecretIssuedAt = issuedAt
	}
	user.MFAEnabled = true
	user.MFAConfirmedAt = confirmationTime

	if err := h.store.UpdateUser(ctx, user); err != nil {
		respondError(c, http.StatusInternalServerError, "mfa_update_failed", "failed to enable mfa")
		return
	}

	h.removeMFAChallenge(token)

	sessionToken, expiresAt, err := h.createSession(user.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "session_creation_failed", "failed to create session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "mfa_verified",
		"token":     sessionToken,
		"expiresAt": expiresAt.UTC(),
		"user":      sanitizeUser(user, nil),
	})
}

func (h *handler) mfaStatus(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = strings.TrimSpace(c.GetHeader("X-MFA-Token"))
	}

	identifier := strings.TrimSpace(c.Query("identifier"))
	if identifier == "" {
		identifier = strings.TrimSpace(c.Query("email"))
	}

	authToken := extractToken(c.GetHeader("Authorization"))

	var (
		user      *store.User
		err       error
		challenge *mfaChallenge
	)

	ctx := c.Request.Context()

	if authToken != "" {
		if sess, ok := h.lookupSession(authToken); ok {
			user, err = h.store.GetUserByID(ctx, sess.userID)
			if err != nil {
				respondError(c, http.StatusInternalServerError, "mfa_status_failed", "failed to load user for status")
				return
			}
		} else if token == "" {
			token = authToken
		}
	}

	if token != "" {
		if refreshed, ok := h.refreshMFAChallenge(token); ok {
			if user != nil && user.ID != refreshed.userID {
				challenge = nil
			} else {
				challenge = &refreshed
				if user == nil {
					user, err = h.store.GetUserByID(ctx, refreshed.userID)
					if err != nil {
						respondError(c, http.StatusInternalServerError, "mfa_status_failed", "failed to load user for status")
						return
					}
				}
			}
		}
	}

	if user == nil && identifier != "" {
		user, err = h.findUserByIdentifier(ctx, identifier)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				respondError(c, http.StatusNotFound, "user_not_found", "user not found")
				return
			}
			respondError(c, http.StatusInternalServerError, "mfa_status_failed", "failed to load user for status")
			return
		}
	}

	if user == nil {
		respondError(c, http.StatusUnauthorized, "mfa_token_required", "valid session or mfa token is required")
		return
	}

	state := buildMFAState(user, challenge)
	c.JSON(http.StatusOK, gin.H{
		"enabled": user.MFAEnabled,
		"mfa":     state,
		"user":    sanitizeUser(user, challenge),
	})
}

func sanitizeUser(user *store.User, challenge *mfaChallenge) gin.H {
	identifier := strings.TrimSpace(user.ID)
	return gin.H{
		"id":            identifier,
		"uuid":          identifier,
		"name":          user.Name,
		"username":      user.Name,
		"email":         user.Email,
		"emailVerified": user.EmailVerified,
		"mfaEnabled":    user.MFAEnabled,
		"mfa":           buildMFAState(user, challenge),
	}
}

func buildMFAState(user *store.User, challenge *mfaChallenge) gin.H {
	pending := strings.TrimSpace(user.MFATOTPSecret) != "" && !user.MFAEnabled
	issuedAt := user.MFASecretIssuedAt

	if challenge != nil && !user.MFAEnabled {
		if strings.TrimSpace(challenge.totpSecret) != "" {
			pending = true
		}
		if issuedAt.IsZero() && !challenge.totpIssuedAt.IsZero() {
			issuedAt = challenge.totpIssuedAt
		}
	}

	state := gin.H{
		"totpEnabled": user.MFAEnabled,
		"totpPending": pending,
	}
	if !issuedAt.IsZero() {
		state["totpSecretIssuedAt"] = issuedAt.UTC()
	}
	if !user.MFAConfirmedAt.IsZero() {
		state["totpConfirmedAt"] = user.MFAConfirmedAt.UTC()
	}
	if challenge != nil && !challenge.lockedUntil.IsZero() && time.Now().Before(challenge.lockedUntil) {
		state["totpLockedUntil"] = challenge.lockedUntil.UTC()
	}
	return state
}

func (h *handler) disableMFA(c *gin.Context) {
	token := extractToken(c.GetHeader("Authorization"))
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	if token == "" {
		respondError(c, http.StatusUnauthorized, "session_token_required", "session token is required")
		return
	}

	sess, ok := h.lookupSession(token)
	if !ok {
		respondError(c, http.StatusUnauthorized, "invalid_session", "session token is invalid or expired")
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByID(ctx, sess.userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "mfa_disable_failed", "failed to load user for mfa disable")
		return
	}

	hasSecret := strings.TrimSpace(user.MFATOTPSecret) != ""
	if !user.MFAEnabled && !hasSecret {
		respondError(c, http.StatusBadRequest, "mfa_not_enabled", "multi-factor authentication is not enabled")
		return
	}

	user.MFATOTPSecret = ""
	user.MFAEnabled = false
	user.MFASecretIssuedAt = time.Time{}
	user.MFAConfirmedAt = time.Time{}

	if err := h.store.UpdateUser(ctx, user); err != nil {
		respondError(c, http.StatusInternalServerError, "mfa_disable_failed", "failed to disable mfa")
		return
	}

	h.removeMFAChallengesForUser(user.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "mfa_disabled",
		"user":    sanitizeUser(user, nil),
	})
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error":   code,
		"message": message,
	})
}

func extractToken(header string) string {
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		header = header[len(prefix):]
	}
	return strings.TrimSpace(header)
}
