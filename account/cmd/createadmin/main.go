package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"xcontrol/account/internal/store"
)

func main() {
	var (
		driver   = flag.String("driver", "postgres", "database driver (postgres, memory)")
		dsn      = flag.String("dsn", "", "database connection string")
		username = flag.String("username", "", "super administrator username")
		password = flag.String("password", "", "super administrator password")
		email    = flag.String("email", "", "super administrator email (optional)")
	)
	flag.Parse()

	if err := run(*driver, *dsn, *username, *password, *email); err != nil {
		log.Fatalf("failed to create super administrator: %v", err)
	}
}

func run(driver, dsn, username, password, email string) error {
	driver = strings.TrimSpace(driver)
	dsn = strings.TrimSpace(dsn)
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	email = strings.TrimSpace(email)

	if username == "" {
		return errors.New("username is required")
	}
	if password == "" {
		return errors.New("password is required")
	}
	if dsn == "" && !strings.EqualFold(driver, "memory") {
		return errors.New("dsn is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	storeConfig := store.Config{
		Driver: driver,
		DSN:    dsn,
	}

	s, cleanup, err := store.New(ctx, storeConfig)
	if err != nil {
		return err
	}
	defer func() {
		_ = cleanup(context.Background())
	}()

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	user := &store.User{
		Name:          username,
		Email:         email,
		PasswordHash:  string(hashed),
		Level:         store.LevelAdmin,
		Role:          store.RoleAdmin,
		Groups:        []string{"Admin"},
		Permissions:   []string{"*"},
		EmailVerified: true,
	}

	if err := s.CreateUser(ctx, user); err != nil {
		if errors.Is(err, store.ErrEmailExists) {
			return fmt.Errorf("email already exists: %w", err)
		}
		if errors.Is(err, store.ErrNameExists) {
			return fmt.Errorf("username already exists: %w", err)
		}
		return err
	}

	fmt.Fprintf(os.Stdout, "Created super administrator %s (id=%s)\n", user.Name, user.ID)
	return nil
}
