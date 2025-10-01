package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"xcontrol/account/internal/store"
)

const defaultSessionTTL = 24 * time.Hour

type session struct {
	userID    string
	expiresAt time.Time
}

type handler struct {
	store      store.Store
	sessions   map[string]session
	mu         sync.RWMutex
	sessionTTL time.Duration
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

// RegisterRoutes attaches account service endpoints to the router.
func RegisterRoutes(r *gin.Engine, opts ...Option) {
	h := &handler{
		store:      store.NewMemoryStore(),
		sessions:   make(map[string]session),
		sessionTTL: defaultSessionTTL,
	}

	for _, opt := range opts {
		opt(h)
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	auth := r.Group("/api/auth")
	auth.POST("/register", h.register)
	auth.POST("/login", h.login)
	auth.GET("/session", h.session)
	auth.DELETE("/session", h.deleteSession)
}

type registerRequest struct {
	Name     string `json:"name" form:"name"`
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

type loginRequest struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

func (h *handler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBind(&req); err != nil {
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

	response := gin.H{
		"message": "user registered successfully",
		"user":    sanitizeUser(user),
	}
	c.JSON(http.StatusCreated, response)
}

func (h *handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBind(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		respondError(c, http.StatusBadRequest, "missing_credentials", "username and password are required")
		return
	}

	user, err := h.store.GetUserByName(c.Request.Context(), username)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			respondError(c, http.StatusNotFound, "user_not_found", "user not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "authentication_failed", "failed to authenticate user")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		respondError(c, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
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
		"user":      sanitizeUser(user),
	})
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

	c.JSON(http.StatusOK, gin.H{"user": sanitizeUser(user)})
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
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(buffer)
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

func sanitizeUser(user *store.User) gin.H {
	return gin.H{
		"id":       user.ID,
		"name":     user.Name,
		"username": user.Name,
		"email":    user.Email,
	}
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
