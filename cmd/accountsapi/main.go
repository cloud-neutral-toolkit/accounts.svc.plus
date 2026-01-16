package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAddr            = "127.0.0.1:8080"
	defaultBodyLimit       = 1 << 20 // 1 MiB
	defaultSessionTTL      = 24 * time.Hour
	defaultRateLimitPerMin = 60
	cookieName             = "accounts_session"
)

type config struct {
	DBUser       string
	DBPassword   string
	DBName       string
	DBSSLMode    string
	BodyLimit    int64
	SessionTTL   time.Duration
	RateLimitRPM int
}

type server struct {
	log        *slog.Logger
	pool       *pgxpool.Pool
	sessions   *sessionStore
	bodyLimit  int64
	limiter    *rateLimiter
	sessionTTL time.Duration
}

type session struct {
	userID    int64
	expiresAt time.Time
}

type sessionStore struct {
	mu   sync.RWMutex
	data map[string]session
}

type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	clients  map[string]rateState
	disabled bool
}

type rateState struct {
	count   int
	resetAt time.Time
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	pool, err := openPool(cfg)
	if err != nil {
		logger.Error("db connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		logger.Error("db health check failed", "err", err)
		os.Exit(1)
	}

	srv := &server{
		log:        logger,
		pool:       pool,
		sessions:   newSessionStore(),
		bodyLimit:  cfg.BodyLimit,
		limiter:    newRateLimiter(cfg.RateLimitRPM, time.Minute),
		sessionTTL: cfg.SessionTTL,
	}

	httpServer := &http.Server{
		Addr:         defaultAddr,
		Handler:      srv.routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("accounts api listening", "addr", defaultAddr)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
		}
	}()

	waitForShutdown(logger, httpServer)
}

func loadConfig() (config, error) {
	cfg := config{
		DBUser:       strings.TrimSpace(os.Getenv("ACCOUNTS_DB_USER")),
		DBPassword:   os.Getenv("ACCOUNTS_DB_PASSWORD"),
		DBName:       strings.TrimSpace(os.Getenv("ACCOUNTS_DB_NAME")),
		DBSSLMode:    strings.TrimSpace(os.Getenv("ACCOUNTS_DB_SSLMODE")),
		BodyLimit:    defaultBodyLimit,
		SessionTTL:   defaultSessionTTL,
		RateLimitRPM: defaultRateLimitPerMin,
	}
	if cfg.DBSSLMode == "" {
		cfg.DBSSLMode = "disable"
	}
	if v := strings.TrimSpace(os.Getenv("ACCOUNTS_BODY_LIMIT")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return config{}, fmt.Errorf("invalid ACCOUNTS_BODY_LIMIT: %q", v)
		}
		cfg.BodyLimit = n
	}
	if v := strings.TrimSpace(os.Getenv("ACCOUNTS_SESSION_TTL")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return config{}, fmt.Errorf("invalid ACCOUNTS_SESSION_TTL: %q", v)
		}
		cfg.SessionTTL = d
	}
	if v := strings.TrimSpace(os.Getenv("ACCOUNTS_RATE_LIMIT_RPM")); v != "" {
		if v == "0" {
			cfg.RateLimitRPM = 0
		} else {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return config{}, fmt.Errorf("invalid ACCOUNTS_RATE_LIMIT_RPM: %q", v)
			}
			cfg.RateLimitRPM = n
		}
	}
	if cfg.DBUser == "" || cfg.DBName == "" {
		return config{}, errors.New("ACCOUNTS_DB_USER and ACCOUNTS_DB_NAME are required")
	}
	return cfg, nil
}

func openPool(cfg config) (*pgxpool.Pool, error) {
	dsn := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.DBUser, cfg.DBPassword),
		Host:     "127.0.0.1:15432",
		Path:     cfg.DBName,
		RawQuery: "sslmode=" + url.QueryEscape(cfg.DBSSLMode),
	}).String()
	pgxCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	pgxCfg.MaxConns = 10
	pgxCfg.MinConns = 2
	pgxCfg.MaxConnIdleTime = 5 * time.Minute
	pgxCfg.MaxConnLifetime = 30 * time.Minute
	return pgxpool.NewWithConfig(context.Background(), pgxCfg)
}

func waitForShutdown(logger *slog.Logger, httpServer *http.Server) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("shutting down")
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/me", s.handleMe)
	return s.middleware(mux)
}

func (s *server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		if s.limiter != nil {
			if ok := s.limiter.Allow(clientIP(r)); !ok {
				writeJSON(wrapped, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
				s.logRequest(r, wrapped.status, start)
				return
			}
		}

		if r.Body != nil && s.bodyLimit > 0 {
			r.Body = http.MaxBytesReader(wrapped, r.Body, s.bodyLimit)
		}

		next.ServeHTTP(wrapped, r)
		s.logRequest(r, wrapped.status, start)
	})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		if isBodyTooLarge(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "body_too_large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email_and_password_required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var (
		userID       int64
		passwordHash string
	)
	err := s.pool.QueryRow(ctx, "SELECT id, password_hash FROM users WHERE email=$1", email).Scan(&userID, &passwordHash)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.log.Error("login query failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
		return
	}

	sessionID, err := generateToken(32)
	if err != nil {
		s.log.Error("session token generation failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	expiresAt := time.Now().Add(s.sessionTTL)
	s.sessions.Set(sessionID, session{userID: userID, expiresAt: expiresAt})

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		s.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	sess, ok := s.sessions.Get(cookie.Value)
	if !ok || time.Now().After(sess.expiresAt) {
		s.sessions.Delete(cookie.Value)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var user userResponse
	err = s.pool.QueryRow(ctx, "SELECT id, email, created_at FROM users WHERE id=$1", sess.userID).
		Scan(&user.ID, &user.Email, &user.CreatedAt)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.log.Error("me query failed", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("extra json fields")
	}
	return nil
}

func isBodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func generateToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func newSessionStore() *sessionStore {
	return &sessionStore{data: make(map[string]session)}
}

func (s *sessionStore) Get(token string) (session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[token]
	return val, ok
}

func (s *sessionStore) Set(token string, sess session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[token] = sess
}

func (s *sessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, token)
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	if limit <= 0 {
		return &rateLimiter{disabled: true}
	}
	return &rateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]rateState),
	}
}

func (r *rateLimiter) Allow(ip string) bool {
	if r == nil || r.disabled {
		return true
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.clients[ip]
	if state.resetAt.IsZero() || now.After(state.resetAt) {
		state.resetAt = now.Add(r.window)
		state.count = 0
	}
	if state.count >= r.limit {
		r.clients[ip] = state
		return false
	}
	state.count++
	r.clients[ip] = state
	return true
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *server) logRequest(r *http.Request, status int, start time.Time) {
	s.log.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"latency", time.Since(start),
		"ip", clientIP(r),
	)
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
