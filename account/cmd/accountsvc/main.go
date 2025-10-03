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
	"github.com/spf13/cobra"

	"xcontrol/account/api"
	"xcontrol/account/config"
	"xcontrol/account/internal/mailer"
	"xcontrol/account/internal/store"
)

var (
	configPath string
	logLevel   string
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

		ctx := context.Background()
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

		options := []api.Option{
			api.WithStore(st),
			api.WithSessionTTL(cfg.Session.TTL),
		}
		if emailSender != nil {
			options = append(options, api.WithEmailSender(emailSender))
		}
		options = append(options, api.WithEmailVerification(emailVerificationEnabled))
		api.RegisterRoutes(r, options...)

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
	},
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
