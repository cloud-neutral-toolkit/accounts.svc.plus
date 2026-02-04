package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"account/api"
	"account/config"
	"account/internal/agentmode"
	"account/internal/agentproto"
	"account/internal/agentserver"
	"account/internal/auth"
	"account/internal/mailer"
	"account/internal/model"
	"account/internal/service"
	"account/internal/store"
	"account/internal/xrayconfig"
)

var (
	configPath string
	logLevel   string
)

const (
	demoUsername = "Demo"
	demoPassword = "Demo"
	demoEmail    = "demo@svc.plus"
	demoGroup    = "ReadOnly Role"
	demoUUIDTTL  = time.Hour

	rootUsername             = "admin"
	rootBootstrapPasswordEnv = "ROOT_BOOTSTRAP_PASSWORD"
)

type mailerAdapter struct {
	sender mailer.Sender
}

func (m mailerAdapter) Send(ctx context.Context, msg api.EmailMessage) error {
	if m.sender == nil {
		return nil
	}
	mail := mailer.Message{
		To:        append([]string(nil), msg.To...),
		Subject:   msg.Subject,
		PlainBody: msg.PlainBody,
		HTMLBody:  msg.HTMLBody,
	}
	return m.sender.Send(ctx, mail)
}

type metricsAdapter struct {
	st store.Store
}

func (a *metricsAdapter) ListUsers(ctx context.Context) ([]service.UserRecord, error) {
	users, err := a.st.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]service.UserRecord, 0, len(users))
	for _, u := range users {
		records = append(records, service.UserRecord{
			ID:        u.ID,
			CreatedAt: u.CreatedAt,
			Active:    u.Active,
		})
	}
	return records, nil
}

func (a *metricsAdapter) FetchSubscriptionStates(ctx context.Context, userIDs []string) (map[string]service.SubscriptionState, error) {
	states := make(map[string]service.SubscriptionState)
	for _, userID := range userIDs {
		subs, err := a.st.ListSubscriptionsByUser(ctx, userID)
		if err != nil {
			continue
		}
		active := false
		var expiresAt *time.Time
		for _, sub := range subs {
			if strings.ToLower(sub.Status) == "active" {
				active = true
				if t, ok := sub.Meta["expiresAt"].(time.Time); ok {
					if expiresAt == nil || t.After(*expiresAt) {
						expiresAt = &t
					}
				}
			}
		}
		states[userID] = service.SubscriptionState{
			Active:    active,
			ExpiresAt: expiresAt,
		}
	}
	return states, nil
}

func ensureDemoUser(ctx context.Context, st store.Store, logger *slog.Logger) error {
	demoUser, err := findDemoUser(ctx, st)
	if err != nil {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	expiresAt := time.Now().UTC().Add(demoUUIDTTL)
	if demoUser == nil {
		user := &store.User{
			Name:               demoUsername,
			Email:              demoEmail,
			EmailVerified:      true,
			PasswordHash:       string(hashed),
			MFATOTPSecret:      "",
			MFAEnabled:         false,
			MFASecretIssuedAt:  time.Time{},
			MFAConfirmedAt:     time.Time{},
			Level:              store.LevelUser,
			Role:               store.RoleReadOnly,
			Groups:             []string{demoGroup},
			Permissions:        []string{},
			Active:             true,
			ProxyUUID:          uuid.NewString(),
			ProxyUUIDExpiresAt: &expiresAt,
		}
		if err := st.CreateUser(ctx, user); err != nil {
			return fmt.Errorf("create demo user: %w", err)
		}
		if logger != nil {
			logger.Info("demo read-only user created", "username", demoUsername, "email", demoEmail)
		}
		return nil
	}

	demoUser.Name = demoUsername
	demoUser.Email = demoEmail
	demoUser.EmailVerified = true
	demoUser.PasswordHash = string(hashed)
	demoUser.MFATOTPSecret = ""
	demoUser.MFAEnabled = false
	demoUser.MFASecretIssuedAt = time.Time{}
	demoUser.MFAConfirmedAt = time.Time{}
	demoUser.Level = store.LevelUser
	demoUser.Role = store.RoleReadOnly
	demoUser.Groups = []string{demoGroup}
	demoUser.Permissions = []string{}
	demoUser.Active = true
	demoUser.ProxyUUID = uuid.NewString()
	demoUser.ProxyUUIDExpiresAt = &expiresAt
	if err := st.UpdateUser(ctx, demoUser); err != nil {
		return fmt.Errorf("update demo user: %w", err)
	}
	if logger != nil {
		logger.Info("demo read-only user ensured", "username", demoUsername, "email", demoEmail)
	}
	return nil
}

func findDemoUser(ctx context.Context, st store.Store) (*store.User, error) {
	userByName, errByName := st.GetUserByName(ctx, demoUsername)
	if errByName != nil && !errors.Is(errByName, store.ErrUserNotFound) {
		return nil, fmt.Errorf("get demo by name: %w", errByName)
	}
	userByEmail, errByEmail := st.GetUserByEmail(ctx, demoEmail)
	if errByEmail != nil && !errors.Is(errByEmail, store.ErrUserNotFound) {
		return nil, fmt.Errorf("get demo by email: %w", errByEmail)
	}

	if userByName != nil && userByEmail != nil && userByName.ID != userByEmail.ID {
		return nil, fmt.Errorf("demo account conflict: username %q and email %q belong to different users", demoUsername, demoEmail)
	}
	if userByName != nil {
		return userByName, nil
	}
	if userByEmail != nil {
		return userByEmail, nil
	}
	return nil, nil
}

func startDemoUUIDRotator(ctx context.Context, st store.Store, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				user, err := findDemoUser(context.Background(), st)
				if err != nil {
					if logger != nil {
						logger.Warn("demo uuid rotation skipped: lookup failed", "err", err)
					}
					continue
				}
				if user == nil {
					if err := ensureDemoUser(context.Background(), st, logger); err != nil && logger != nil {
						logger.Warn("demo uuid rotation failed to recreate user", "err", err)
					}
					continue
				}

				expiresAt := time.Now().UTC().Add(demoUUIDTTL)
				user.ProxyUUID = uuid.NewString()
				user.ProxyUUIDExpiresAt = &expiresAt
				if err := st.UpdateUser(context.Background(), user); err != nil {
					if logger != nil {
						logger.Warn("demo uuid rotation failed", "err", err)
					}
					continue
				}
				if logger != nil {
					logger.Info("demo uuid rotated", "userID", user.ID, "expiresAt", expiresAt)
				}
			}
		}
	}()
}

func ensureRootUser(ctx context.Context, st store.Store, logger *slog.Logger) error {
	users, err := st.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("list users for root check: %w", err)
	}

	var rootUser *store.User
	for i := range users {
		user := users[i]
		if strings.EqualFold(strings.TrimSpace(user.Email), store.RootAdminEmail) {
			candidate := user
			rootUser = &candidate
			break
		}
	}

	if rootUser == nil {
		bootstrapPassword := strings.TrimSpace(os.Getenv(rootBootstrapPasswordEnv))
		if bootstrapPassword == "" {
			return fmt.Errorf("root account %q missing: set %s to bootstrap it", store.RootAdminEmail, rootBootstrapPasswordEnv)
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash root bootstrap password: %w", err)
		}

		root := &store.User{
			Name:          rootUsername,
			Email:         store.RootAdminEmail,
			PasswordHash:  string(hashed),
			EmailVerified: true,
			Role:          store.RoleRoot,
			Level:         store.LevelAdmin,
			Groups:        []string{"Admin"},
			Permissions:   []string{"*"},
			Active:        true,
		}
		if err := st.CreateUser(ctx, root); err != nil {
			return fmt.Errorf("create root user: %w", err)
		}
		rootUser = root
		if logger != nil {
			logger.Warn("root account bootstrapped from environment variable", "email", store.RootAdminEmail)
		}
	}

	if rootUser != nil {
		updatedRoot := *rootUser
		if enforceRootProfile(&updatedRoot) {
			if err := st.UpdateUser(ctx, &updatedRoot); err != nil {
				return fmt.Errorf("enforce root profile: %w", err)
			}
			rootUser = &updatedRoot
			if logger != nil {
				logger.Info("root profile normalized", "email", store.RootAdminEmail, "userID", rootUser.ID)
			}
		}
	}

	for i := range users {
		user := users[i]
		if rootUser != nil && user.ID == rootUser.ID {
			continue
		}
		if !store.IsAdminRole(user.Role) {
			continue
		}

		updated := user
		updated.Role = store.RoleOperator
		updated.Level = store.LevelOperator
		updated.Permissions = dropPermission(updated.Permissions, "*")
		updated.Groups = dropGroup(updated.Groups, "Admin")
		if len(updated.Groups) == 0 {
			updated.Groups = []string{"Operator"}
		}

		if err := st.UpdateUser(ctx, &updated); err != nil {
			return fmt.Errorf("demote legacy root/admin user %q: %w", user.Email, err)
		}
		if logger != nil {
			logger.Warn("demoted legacy root/admin account to operator", "userID", updated.ID, "email", updated.Email)
		}
	}

	return nil
}

func enforceRootProfile(user *store.User) bool {
	if user == nil {
		return false
	}

	changed := false
	if !strings.EqualFold(strings.TrimSpace(user.Email), store.RootAdminEmail) {
		user.Email = store.RootAdminEmail
		changed = true
	}
	if strings.ToLower(strings.TrimSpace(user.Role)) != store.RoleRoot {
		user.Role = store.RoleRoot
		changed = true
	}
	if user.Level != store.LevelAdmin {
		user.Level = store.LevelAdmin
		changed = true
	}
	if !user.Active {
		user.Active = true
		changed = true
	}
	if !user.EmailVerified {
		user.EmailVerified = true
		changed = true
	}
	if !containsCaseInsensitive(user.Groups, "Admin") {
		user.Groups = append(user.Groups, "Admin")
		changed = true
	}
	if !containsExactValue(user.Permissions, "*") {
		user.Permissions = append(user.Permissions, "*")
		changed = true
	}
	return changed
}

func dropPermission(values []string, permission string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == permission {
			continue
		}
		result = append(result, value)
	}
	return result
}

func dropGroup(values []string, group string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), group) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func containsCaseInsensitive(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func containsExactValue(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func applyRBACSchema(ctx context.Context, db *gorm.DB, driver string) error {
	if db == nil {
		return errors.New("database is nil")
	}

	normalized := strings.ToLower(strings.TrimSpace(driver))
	if normalized != "postgres" && normalized != "postgresql" && normalized != "pgx" {
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS public.rbac_roles (
  role_key TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT '',
  priority INTEGER NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`,
		`CREATE TABLE IF NOT EXISTS public.rbac_permissions (
  permission_key TEXT PRIMARY KEY,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`,
		`CREATE TABLE IF NOT EXISTS public.rbac_role_permissions (
  role_key TEXT NOT NULL REFERENCES public.rbac_roles(role_key) ON DELETE CASCADE,
  permission_key TEXT NOT NULL REFERENCES public.rbac_permissions(permission_key) ON DELETE CASCADE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (role_key, permission_key)
)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS users_single_root_role_uk ON public.users ((lower(role))) WHERE lower(role) = 'root'`,
		`DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'users_root_email_ck'
  ) THEN
    ALTER TABLE public.users
      ADD CONSTRAINT users_root_email_ck
      CHECK (lower(role) <> 'root' OR lower(email) = 'admin@svc.plus');
  END IF;
END
$$`,
	}

	for _, stmt := range statements {
		if err := db.WithContext(ctx).Exec(stmt).Error; err != nil {
			return err
		}
	}

	seedStatements := []string{
		`INSERT INTO public.rbac_roles (role_key, description, priority)
VALUES
  ('root', 'single root account', 0),
  ('operator', 'operation role with configurable permissions', 10),
  ('user', 'standard subscription user', 20),
  ('readonly', 'read-only experience account', 30)
ON CONFLICT (role_key) DO NOTHING`,
		`INSERT INTO public.rbac_permissions (permission_key, description)
VALUES
  ('admin.settings.read', 'read admin matrix settings'),
  ('admin.settings.write', 'update admin matrix settings'),
  ('admin.users.metrics.read', 'read user metrics'),
  ('admin.users.list.read', 'read user list'),
  ('admin.agents.status.read', 'read agent status'),
  ('admin.users.pause.write', 'pause users'),
  ('admin.users.resume.write', 'resume users'),
  ('admin.users.delete.write', 'delete users'),
  ('admin.users.renew_uuid.write', 'renew user proxy uuid'),
  ('admin.users.role.write', 'update/reset user role'),
  ('admin.blacklist.read', 'read blacklist'),
  ('admin.blacklist.write', 'update blacklist')
ON CONFLICT (permission_key) DO NOTHING`,
		`INSERT INTO public.rbac_role_permissions (role_key, permission_key, enabled)
SELECT 'operator', permission_key, true
FROM public.rbac_permissions
ON CONFLICT (role_key, permission_key) DO NOTHING`,
	}

	for _, stmt := range seedStatements {
		if err := db.WithContext(ctx).Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}

func runServer(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return errors.New("config is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}

	r := gin.New()
	corsConfig := buildCORSConfig(logger, cfg.Server)
	if corsConfig.AllowAllOrigins {
		logger.Info("configured cors", "allowAllOrigins", true)
	} else {
		logger.Info("configured cors", "allowedOrigins", corsConfig.AllowOrigins)
	}
	r.Use(cors.New(corsConfig))
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request", "method", c.Request.Method, "path", c.FullPath(), "status", c.Writer.Status(), "latency", time.Since(start))
	})

	storeCfg := store.Config{
		Driver:       cfg.Store.Driver,
		DSN:          cfg.Store.DSN,
		MaxOpenConns: cfg.Store.MaxOpenConns,
		MaxIdleConns: cfg.Store.MaxIdleConns,
	}

	st, cleanup, err := store.New(ctx, storeCfg)
	if err != nil {
		return err
	}
	defer func() {
		if cleanup == nil {
			return
		}
		if err := cleanup(context.Background()); err != nil {
			logger.Error("failed to close store", "err", err)
		}
	}()

	if err := ensureRootUser(ctx, st, logger); err != nil {
		return err
	}

	if err := ensureDemoUser(ctx, st, logger); err != nil {
		return err
	}
	startDemoUUIDRotator(ctx, st, logger)

	var emailSender api.EmailSender
	emailVerificationEnabled := true
	smtpHost := strings.TrimSpace(cfg.SMTP.Host)
	if smtpHost == "" {
		emailVerificationEnabled = false
	}
	if smtpHost != "" && isExampleDomain(smtpHost) {
		emailVerificationEnabled = false
		logger.Warn("smtp host is a placeholder; disabling email delivery", "host", smtpHost)
		smtpHost = ""
	}
	if smtpHost != "" {
		tlsMode := mailer.ParseTLSMode(cfg.SMTP.TLS.Mode)
		sender, err := mailer.New(mailer.Config{
			Host:               smtpHost,
			Port:               cfg.SMTP.Port,
			Username:           cfg.SMTP.Username,
			Password:           cfg.SMTP.Password,
			From:               cfg.SMTP.From,
			ReplyTo:            cfg.SMTP.ReplyTo,
			Timeout:            cfg.SMTP.Timeout,
			TLSMode:            tlsMode,
			InsecureSkipVerify: cfg.SMTP.TLS.InsecureSkipVerify,
		})
		if err != nil {
			return err
		}
		emailSender = mailerAdapter{sender: sender}
	}
	if emailSender == nil {
		emailVerificationEnabled = false
	}

	// Initialize TokenService for authentication
	var tokenService *auth.TokenService
	if cfg.Auth.Enable {
		accessExpiry := cfg.Auth.Token.AccessExpiry
		if accessExpiry <= 0 {
			accessExpiry = 1 * time.Hour
		}
		refreshExpiry := cfg.Auth.Token.RefreshExpiry
		if refreshExpiry <= 0 {
			refreshExpiry = 168 * time.Hour // 7 days
		}

		tokenService = auth.NewTokenService(auth.TokenConfig{
			PublicToken:   cfg.Auth.Token.PublicToken,
			RefreshSecret: cfg.Auth.Token.RefreshSecret,
			AccessSecret:  cfg.Auth.Token.AccessSecret,
			AccessExpiry:  accessExpiry,
			RefreshExpiry: refreshExpiry,
		})
		logger.Info("token service initialized", "auth_enabled", cfg.Auth.Enable)
	}

	gormDB, gormCleanup, err := openAdminSettingsDB(cfg.Store)
	if err != nil {
		return err
	}
	defer func() {
		if gormCleanup != nil {
			if err := gormCleanup(context.Background()); err != nil {
				logger.Error("failed to close admin settings db", "err", err)
			}
		}
	}()
	service.SetDB(gormDB)

	if err := applyRBACSchema(ctx, gormDB, cfg.Store.Driver); err != nil {
		return fmt.Errorf("apply rbac schema: %w", err)
	}

	gormSource, err := xrayconfig.NewGormClientSource(gormDB)
	if err != nil {
		return err
	}

	var agentRegistry *agentserver.Registry
	if len(cfg.Agents.Credentials) > 0 {
		creds := make([]agentserver.Credential, 0, len(cfg.Agents.Credentials))
		for _, c := range cfg.Agents.Credentials {
			creds = append(creds, agentserver.Credential{
				ID:     c.ID,
				Name:   c.Name,
				Token:  c.Token,
				Groups: append([]string(nil), c.Groups...),
			})
		}
		agentRegistry, err = agentserver.NewRegistry(agentserver.Config{Credentials: creds})
		if err != nil {
			return err
		}
	} else if token := os.Getenv("INTERNAL_SERVICE_TOKEN"); token != "" {
		// Fallback: if no credentials configured but we have an internal token,
		// register a default internal agent.
		agentRegistry, err = agentserver.NewRegistry(agentserver.Config{
			Credentials: []agentserver.Credential{{
				ID:     "internal-agent",
				Name:   "Internal Agent",
				Token:  token,
				Groups: []string{"internal"},
			}},
		})
		if err != nil {
			return err
		}
	}

	var stopXraySync func(context.Context) error
	if cfg.Xray.Sync.Enabled {
		syncInterval := cfg.Xray.Sync.Interval
		if syncInterval <= 0 {
			syncInterval = 5 * time.Minute
		}
		outputPath := strings.TrimSpace(cfg.Xray.Sync.OutputPath)
		if outputPath == "" {
			outputPath = "/usr/local/etc/xray/config.json"
		}
		syncer, err := xrayconfig.NewPeriodicSyncer(xrayconfig.PeriodicOptions{
			Logger:   logger.With("component", "xray-sync"),
			Interval: syncInterval,
			Source:   gormSource,
			Generators: []xrayconfig.Generator{
				{
					Definition: xrayconfig.XHTTPDefinition(),
					OutputPath: "/usr/local/etc/xray/config.json", // Match user's xhttp config path
					Domain:     cfg.Xray.Sync.Domain,
				},
				{
					Definition: xrayconfig.TCPDefinition(),
					OutputPath: "/usr/local/etc/xray/tcp-config.json", // Match user's tcp config path
					Domain:     cfg.Xray.Sync.Domain,
				},
			},
			ValidateCommand: cfg.Xray.Sync.ValidateCommand,
			RestartCommand:  cfg.Xray.Sync.RestartCommand,
		})
		if err != nil {
			return err
		}
		stop, err := syncer.Start(ctx)
		if err != nil {
			return err
		}
		logger.Info("xray periodic sync enabled", "interval", syncInterval, "output", outputPath)
		stopXraySync = stop
	}

	if stopXraySync != nil {
		defer func() {
			waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := stopXraySync(waitCtx); err != nil {
				logger.Warn("xray syncer shutdown", "err", err)
			}
		}()
	}

	options := []api.Option{
		api.WithStore(st),
		api.WithSessionTTL(cfg.Session.TTL),
		api.WithEmailSender(emailSender),
		api.WithEmailVerification(emailVerificationEnabled),
		api.WithTokenService(tokenService),
		api.WithOAuthFrontendURL(cfg.Auth.OAuth.FrontendURL),
		api.WithServerPublicURL(cfg.Server.PublicURL),
	}

	if agentRegistry != nil {
		options = append(options, api.WithAgentStatusReader(agentRegistry))
	}

	// Initialize User Metrics Service
	metricsSvc := &service.UserMetricsService{
		Users:         &metricsAdapter{st: st},
		Subscriptions: &metricsAdapter{st: st},
	}
	options = append(options, api.WithUserMetricsProvider(metricsSvc))

	// Initialize OAuth providers
	oauthProviders := make(map[string]auth.OAuthProvider)
	if cfg.Auth.Enable {
		if cfg.Auth.OAuth.GitHub.ClientID != "" {
			redirectURL := cfg.Auth.OAuth.GitHub.RedirectURL
			if redirectURL == "" {
				redirectURL = cfg.Auth.OAuth.RedirectURL
			}
			oauthProviders["github"] = auth.NewGitHubProvider(
				cfg.Auth.OAuth.GitHub.ClientID,
				cfg.Auth.OAuth.GitHub.ClientSecret,
				redirectURL,
			)
		}
		if cfg.Auth.OAuth.Google.ClientID != "" {
			redirectURL := cfg.Auth.OAuth.Google.RedirectURL
			if redirectURL == "" {
				redirectURL = cfg.Auth.OAuth.RedirectURL
			}
			oauthProviders["google"] = auth.NewGoogleProvider(
				cfg.Auth.OAuth.Google.ClientID,
				cfg.Auth.OAuth.Google.ClientSecret,
				redirectURL,
			)
		}
	}
	options = append(options, api.WithOAuthProviders(oauthProviders))

	api.RegisterRoutes(r, options...)

	if agentRegistry != nil {
		registerAgentAPIRoutes(r, agentRegistry, gormSource, logger)
	}

	addr := strings.TrimSpace(cfg.Server.Addr)
	if addr == "" {
		addr = ":8080"
	}

	tlsSettings := cfg.Server.TLS
	certFile := strings.TrimSpace(tlsSettings.CertFile)
	keyFile := strings.TrimSpace(tlsSettings.KeyFile)
	caFile := strings.TrimSpace(tlsSettings.CAFile)
	clientCAFile := strings.TrimSpace(tlsSettings.ClientCAFile)

	useTLS := tlsSettings.IsEnabled()

	var tlsConfig *tls.Config
	if useTLS {
		if certFile == "" || keyFile == "" {
			return fmt.Errorf("tls is enabled but certFile (%q) or keyFile (%q) is empty", certFile, keyFile)
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load tls certificate: %w", err)
		}

		if caFile != "" {
			caPEM, err := os.ReadFile(caFile)
			if err != nil {
				return fmt.Errorf("failed to read ca file %q: %w", caFile, err)
			}

			var block *pem.Block
			existing := make(map[string]struct{}, len(cert.Certificate))
			for _, c := range cert.Certificate {
				existing[string(c)] = struct{}{}
			}

			for len(caPEM) > 0 {
				block, caPEM = pem.Decode(caPEM)
				if block == nil {
					break
				}
				if block.Type != "CERTIFICATE" || len(block.Bytes) == 0 {
					continue
				}
				if _, ok := existing[string(block.Bytes)]; ok {
					continue
				}
				cert.Certificate = append(cert.Certificate, block.Bytes)
			}

			if len(cert.Certificate) == 0 {
				return fmt.Errorf("ca file %q did not contain any certificates", caFile)
			}
		}

		tlsConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}

		if clientCAFile != "" {
			caBytes, err := os.ReadFile(clientCAFile)
			if err != nil {
				return err
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caBytes) {
				return errors.New("failed to parse client CA file")
			}
			tlsConfig.ClientCAs = pool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	} else {
		if certFile != "" || keyFile != "" {
			logger.Info("TLS disabled; certificate paths will be ignored", "certFile", certFile, "keyFile", keyFile)
		}
		if clientCAFile != "" {
			logger.Warn("client CA configured but TLS is disabled; ignoring", "clientCAFile", clientCAFile)
		}
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	if useTLS {
		srv.TLSConfig = tlsConfig
	}

	logger.Info("starting account service", "addr", addr, "tls", useTLS)

	var listenCertFile, listenKeyFile string
	if useTLS {
		if tlsSettings.RedirectHTTP {
			go func() {
				redirectAddr := deriveRedirectAddr(addr)
				redirectSrv := &http.Server{
					Addr: redirectAddr,
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						host := r.Host
						if host == "" {
							host = redirectAddr
						}
						target := "https://" + host + r.URL.RequestURI()
						http.Redirect(w, r, target, http.StatusPermanentRedirect)
					}),
				}
				if err := redirectSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("http redirect listener exited", "err", err)
				}
			}()
		}

		if tlsConfig != nil && len(tlsConfig.Certificates) > 0 {
			listenCertFile = ""
			listenKeyFile = ""
		} else {
			listenCertFile = certFile
			listenKeyFile = keyFile
		}

		if err := srv.ListenAndServeTLS(listenCertFile, listenKeyFile); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logger.Error("account service shutdown", "err", err)
				return err
			}
		}
	} else {
		if err := srv.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logger.Error("account service shutdown", "err", err)
				return err
			}
		}
	}
	return nil
}

func runServerAndAgent(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return errors.New("config is nil")
	}

	agentCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	agentErrCh := make(chan error, 1)
	go func() {
		agentErrCh <- runAgent(agentCtx, cfg, logger)
	}()

	agentPending := true

	select {
	case err := <-agentErrCh:
		agentPending = false
		if err == nil {
			err = errors.New("agent exited unexpectedly")
		}
		return fmt.Errorf("agent startup failed: %w", err)
	default:
	}

	serverErr := runServer(ctx, cfg, logger)
	cancel()

	var agentErr error
	if agentPending {
		agentErr = <-agentErrCh
	}

	if serverErr != nil {
		return serverErr
	}
	if agentErr != nil {
		return agentErr
	}
	return nil
}

func runAgent(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if !cfg.Xray.Sync.Enabled {
		logger.Warn("xray sync is disabled in configuration; agent mode will still attempt to manage xray config")
	}
	options := agentmode.Options{
		Logger: logger.With("component", "agent"),
		Agent:  cfg.Agent,
		Xray:   cfg.Xray,
	}
	return agentmode.Run(ctx, options)
}

const agentIdentityContextKey = "xcontrol-account-agent-identity"

func registerAgentAPIRoutes(r *gin.Engine, registry *agentserver.Registry, source xrayconfig.ClientSource, logger *slog.Logger) {
	if registry == nil {
		return
	}
	// Use /api/agent-server/v1 to avoid conflict with /api/agent prefix used by admin/user API
	group := r.Group("/api/agent-server/v1")
	group.Use(agentAuthMiddleware(registry))
	group.GET("/users", agentListUsersHandler(source))
	group.POST("/status", agentReportStatusHandler(registry, logger))
}

func agentAuthMiddleware(registry *agentserver.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		if registry == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "agent_registry_unavailable", "message": "agent registry not configured"})
			return
		}
		token := extractBearerToken(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "agent_token_required", "message": "agent token is required"})
			return
		}
		identity, ok := registry.Authenticate(token)
		if !ok || identity == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid_agent_token", "message": "invalid agent token"})
			return
		}
		c.Set(agentIdentityContextKey, *identity)
		c.Next()
	}
}

func agentListUsersHandler(source xrayconfig.ClientSource) gin.HandlerFunc {
	return func(c *gin.Context) {
		if source == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "client_source_unavailable", "message": "client source not configured"})
			return
		}
		clients, err := source.ListClients(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "list_clients_failed", "message": "failed to list clients"})
			return
		}
		response := agentproto.ClientListResponse{
			Clients:     clients,
			Total:       len(clients),
			GeneratedAt: time.Now().UTC(),
		}
		c.JSON(http.StatusOK, response)
	}
}

func agentReportStatusHandler(registry *agentserver.Registry, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(agentIdentityContextKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "agent_identity_missing", "message": "agent identity missing"})
			return
		}
		identity, ok := value.(agentserver.Identity)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "agent_identity_invalid", "message": "agent identity malformed"})
			return
		}
		var report agentproto.StatusReport
		if err := c.ShouldBindJSON(&report); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid_status_payload", "message": "invalid status payload"})
			return
		}
		registry.ReportStatus(identity, report)
		if logger != nil {
			logger.Info("agent status updated", "agent", identity.ID, "healthy", report.Healthy, "clients", report.Xray.Clients)
		}
		c.Status(http.StatusNoContent)
	}
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		header = header[len(prefix):]
	}
	return strings.TrimSpace(header)
}

var rootCmd = &cobra.Command{
	Use:   "xcontrol-account",
	Short: "Start the xcontrol account service",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		if logLevel != "" {
			cfg.Log.Level = logLevel
		}

		level := slog.LevelInfo
		switch strings.ToLower(strings.TrimSpace(cfg.Log.Level)) {
		case "debug":
			level = slog.LevelDebug
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
		slog.SetDefault(logger)

		ctx := context.Background()
		mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
		if mode == "" {
			mode = "server"
		}

		switch mode {
		case "server":
			return runServer(ctx, cfg, logger)
		case "agent":
			return runAgent(ctx, cfg, logger)
		case "server-agent", "all", "combined":
			return runServerAndAgent(ctx, cfg, logger)
		default:
			return fmt.Errorf("unsupported mode %q", cfg.Mode)
		}
	},
}

func openAdminSettingsDB(cfg config.Store) (*gorm.DB, func(context.Context) error, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	var (
		db  *gorm.DB
		err error
	)
	switch driver {
	case "", "memory":
		db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	case "postgres", "postgresql", "pgx":
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, nil, errors.New("admin settings database requires a dsn")
		}
		db, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	default:
		return nil, nil, fmt.Errorf("unsupported admin settings driver %q", cfg.Driver)
	}
	if err != nil {
		return nil, nil, err
	}

	if err := db.AutoMigrate(&model.AdminSetting{}); err != nil {
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	cleanup := func(context.Context) error {
		return sqlDB.Close()
	}
	return db, cleanup, nil
}

func init() {
	rootCmd.Flags().StringVar(&configPath, "config", "", "path to xcontrol account configuration file")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "", "log level (debug, info, warn, error)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func isExampleDomain(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	if normalized == "" {
		return false
	}
	if h, _, ok := strings.Cut(normalized, ":"); ok {
		normalized = h
	}
	if normalized == "example.com" {
		return true
	}
	return strings.HasSuffix(normalized, ".example.com")
}

func buildCORSConfig(logger *slog.Logger, serverCfg config.Server) cors.Config {
	allowOrigins, allowAll := resolveAllowedOrigins(logger, serverCfg)

	cfg := cors.Config{
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Authorization",
			"Content-Type",
			"Accept",
			"Origin",
			"X-Requested-With",
			"Cookie",
		},
		ExposeHeaders: []string{
			"Content-Length",
		},
		MaxAge: 12 * time.Hour,
	}

	if allowAll {
		cfg.AllowAllOrigins = true
		cfg.AllowCredentials = false
	} else {
		cfg.AllowOrigins = allowOrigins
		cfg.AllowCredentials = true
	}

	return cfg
}

func resolveAllowedOrigins(logger *slog.Logger, serverCfg config.Server) ([]string, bool) {
	rawOrigins := serverCfg.AllowedOrigins
	seen := make(map[string]struct{}, len(rawOrigins))
	origins := make([]string, 0, len(rawOrigins))
	allowAll := false

	for _, origin := range rawOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			allowAll = true
			continue
		}

		normalized, err := parseOrigin(trimmed)
		if err != nil {
			logger.Warn("ignoring invalid cors origin", "origin", origin, "err", err)
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		origins = append(origins, normalized)
	}

	if allowAll {
		return nil, true
	}

	if len(origins) == 0 {
		publicURL := strings.TrimSpace(serverCfg.PublicURL)
		if publicURL != "" {
			normalized, err := parseOrigin(publicURL)
			if err != nil {
				logger.Warn("invalid server public url; falling back to defaults", "publicUrl", publicURL, "err", err)
			} else {
				origins = append(origins, normalized)
			}
		}
	}

	if len(origins) == 0 {
		origins = []string{
			"http://localhost:3001",
			"http://127.0.0.1:3001",
		}
	}

	return origins, false
}

func parseOrigin(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("origin is empty")
	}

	normalized := trimmed
	if !strings.Contains(normalized, "://") {
		normalized = "https://" + normalized
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}

	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme == "" {
		return "", fmt.Errorf("origin must include a scheme")
	}

	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" {
		return "", fmt.Errorf("origin must include a host")
	}

	host := hostname
	if port := strings.TrimSpace(parsed.Port()); port != "" {
		host = net.JoinHostPort(hostname, port)
	}

	return scheme + "://" + host, nil
}

func deriveRedirectAddr(addr string) string {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		trimmed := strings.TrimSpace(addr)
		if strings.HasPrefix(trimmed, ":") {
			port = strings.TrimPrefix(trimmed, ":")
			if port == "" || port == "443" {
				return ":80"
			}
			return ":" + port
		}
		return ":80"
	}
	if port == "" || port == "443" {
		port = "80"
	}
	return net.JoinHostPort(host, port)
}
