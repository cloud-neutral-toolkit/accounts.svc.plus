package main

import (
	"context"
	"errors"
	"log/slog"
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

		srv := &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		}

		logger.Info("starting account service", "addr", addr)
		if err := srv.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logger.Error("account service shutdown", "err", err)
				return err
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
