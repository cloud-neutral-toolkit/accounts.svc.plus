package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"xcontrol/account/api"
	"xcontrol/account/config"
	"xcontrol/account/internal/store"
)

var (
	configPath string
	logLevel   string
)

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

		api.RegisterRoutes(r,
			api.WithStore(st),
			api.WithSessionTTL(cfg.Session.TTL),
		)

		addr := strings.TrimSpace(cfg.Server.Addr)
		if addr == "" {
			addr = ":8080"
		}

		tlsSettings := cfg.Server.TLS
		certFile := strings.TrimSpace(tlsSettings.CertFile)
		keyFile := strings.TrimSpace(tlsSettings.KeyFile)
		clientCAFile := strings.TrimSpace(tlsSettings.ClientCAFile)

		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
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

		useTLS := certFile != "" && keyFile != ""

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

			if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil {
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
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
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
