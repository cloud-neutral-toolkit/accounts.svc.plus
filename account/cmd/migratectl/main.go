package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"xcontrol/account/internal/migrate"
)

const (
	defaultMigrationDir = "account/sql/migrations"
	defaultSchemaFile   = "account/sql/schema.sql"
)

func main() {
	ctx := context.Background()
	rootCmd := newRootCmd()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var migrationDir string
	cmd := &cobra.Command{
		Use:   "migratectl",
		Short: "XControl database migration orchestrator",
	}

	migrationDir = defaultMigrationDir
	cmd.PersistentFlags().StringVar(&migrationDir, "dir", migrationDir, "directory containing migration files")

	cmd.AddCommand(newMigrateCmd(&migrationDir))
	cmd.AddCommand(newCleanCmd())
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newVerifyCmd())
	cmd.AddCommand(newResetCmd(&migrationDir))
	cmd.AddCommand(newVersionCmd(&migrationDir))

	return cmd
}

func newMigrateCmd(dir *string) *cobra.Command {
	var dsn string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dsn == "" {
				return errors.New("--dsn is required")
			}
			runner := migrate.NewRunner(*dir)
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			return runner.Up(ctx, dsn)
		},
	}
	cmd.Flags().StringVar(&dsn, "dsn", "", "PostgreSQL connection string")
	return cmd
}

func newCleanCmd() *cobra.Command {
	var (
		dsn   string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean leftover database structures",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dsn == "" {
				return errors.New("--dsn is required")
			}
			cleaner := migrate.NewCleaner()
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			return cleaner.Clean(ctx, dsn, force)
		},
	}
	cmd.Flags().StringVar(&dsn, "dsn", "", "PostgreSQL connection string")
	cmd.Flags().BoolVar(&force, "force", false, "Confirm clean-up actions")
	return cmd
}

func newCheckCmd() *cobra.Command {
	var (
		cnDSN     string
		globalDSN string
		autoFix   bool
	)
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Compare CN and Global schemas",
		RunE: func(cmd *cobra.Command, args []string) error {
			checker := migrate.NewChecker()
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()
			return checker.Check(ctx, cnDSN, globalDSN, autoFix)
		},
	}
	cmd.Flags().StringVar(&cnDSN, "cn", "", "CN region PostgreSQL DSN")
	cmd.Flags().StringVar(&globalDSN, "global", "", "Global region PostgreSQL DSN")
	cmd.Flags().BoolVar(&autoFix, "auto-fix", false, "Automatically apply missing statements to CN")
	return cmd
}

func newVerifyCmd() *cobra.Command {
	var (
		dsn        string
		schemaPath string
	)
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify that the database matches schema.sql",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dsn == "" {
				return errors.New("--dsn is required")
			}
			if schemaPath == "" {
				schemaPath = defaultSchemaFile
			}
			verifier := migrate.NewVerifier()
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			return verifier.Verify(ctx, dsn, schemaPath)
		},
	}
	cmd.Flags().StringVar(&dsn, "dsn", "", "PostgreSQL connection string")
	cmd.Flags().StringVar(&schemaPath, "schema", defaultSchemaFile, "Path to schema.sql reference file")
	return cmd
}

func newResetCmd(dir *string) *cobra.Command {
	var dsn string
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Drop public schema and re-run migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dsn == "" {
				return errors.New("--dsn is required")
			}
			runner := migrate.NewRunner(*dir)
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()
			return runner.Reset(ctx, dsn)
		},
	}
	cmd.Flags().StringVar(&dsn, "dsn", "", "PostgreSQL connection string")
	return cmd
}

func newVersionCmd(dir *string) *cobra.Command {
	var dsn string
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show current migration version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dsn == "" {
				return errors.New("--dsn is required")
			}
			runner := migrate.NewRunner(*dir)
			version, dirty, err := runner.Version(dsn)
			if err != nil {
				return err
			}
			if dirty {
				fmt.Printf("Current migration version: %d (dirty)\n", version)
			} else {
				fmt.Printf("Current migration version: %d\n", version)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dsn, "dsn", "", "PostgreSQL connection string")
	return cmd
}
