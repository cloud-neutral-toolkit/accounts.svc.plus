package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config describes how to construct a Store implementation.
type Config struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// New creates a Store implementation based on the provided configuration.
func New(ctx context.Context, cfg Config) (Store, func(context.Context) error, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" || driver == "memory" {
		return NewMemoryStore(), func(context.Context) error { return nil }, nil
	}

	switch driver {
	case "postgres", "postgresql", "pgx":
		if strings.TrimSpace(cfg.DSN) == "" {
			return nil, nil, errors.New("store dsn is required for postgres driver")
		}

		db, err := sql.Open("pgx", cfg.DSN)
		if err != nil {
			return nil, nil, err
		}

		if cfg.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns > 0 {
			db.SetMaxIdleConns(cfg.MaxIdleConns)
		}
		if cfg.ConnMaxLifetime > 0 {
			db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
		}
		if cfg.ConnMaxIdleTime > 0 {
			db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
		}

		if err := db.PingContext(ctx); err != nil {
			db.Close()
			return nil, nil, err
		}

		cleanup := func(context.Context) error {
			return db.Close()
		}

		return &postgresStore{db: db}, cleanup, nil
	default:
		return nil, nil, fmt.Errorf("unsupported store driver %q", cfg.Driver)
	}
}

type postgresStore struct {
	db *sql.DB
}

func (s *postgresStore) CreateUser(ctx context.Context, user *User) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))
	normalizedName := strings.TrimSpace(user.Name)
	if normalizedName == "" {
		return ErrInvalidName
	}

	exists, err := s.userExistsByName(ctx, normalizedName)
	if err != nil {
		return err
	}
	if exists {
		return ErrNameExists
	}

	if normalizedEmail != "" {
		exists, err = s.userExistsByEmail(ctx, normalizedEmail)
		if err != nil {
			return err
		}
		if exists {
			return ErrEmailExists
		}
	}

	query := `INSERT INTO users (username, email, password)
              VALUES ($1, $2, $3)
              RETURNING id, coalesce(created_at, now())`

	var idValue any
	var createdAt time.Time
	err = s.db.QueryRowContext(ctx, query, normalizedName, normalizedEmail, user.PasswordHash).Scan(&idValue, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				switch {
				case strings.Contains(pgErr.ConstraintName, "email"):
					return ErrEmailExists
				case strings.Contains(pgErr.ConstraintName, "name") || strings.Contains(pgErr.ConstraintName, "username"):
					return ErrNameExists
				}
			}
		}
		return err
	}

	identifier, err := formatIdentifier(idValue)
	if err != nil {
		return err
	}

	user.ID = identifier
	user.Name = normalizedName
	user.Email = normalizedEmail
	user.CreatedAt = createdAt.UTC()
	return nil
}

func (s *postgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return nil, ErrUserNotFound
	}

	query := `SELECT id, username, email, password, coalesce(created_at, now())
              FROM users WHERE lower(email) = $1 LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, normalized)
	return scanUser(row)
}

func (s *postgresStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT id, username, email, password, coalesce(created_at, now())
              FROM users WHERE id = $1`

	row := s.db.QueryRowContext(ctx, query, id)
	return scanUser(row)
}

func (s *postgresStore) userExistsByEmail(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, nil
	}

	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE lower(email) = lower($1) LIMIT 1`, email).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (s *postgresStore) userExistsByName(ctx context.Context, name string) (bool, error) {
	if name == "" {
		return false, nil
	}

	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE lower(username) = lower($1) LIMIT 1`, name).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var (
		idValue   any
		username  sql.NullString
		email     sql.NullString
		password  sql.NullString
		createdAt time.Time
	)

	if err := row.Scan(&idValue, &username, &email, &password, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	identifier, err := formatIdentifier(idValue)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           identifier,
		Name:         strings.TrimSpace(username.String),
		Email:        strings.ToLower(strings.TrimSpace(email.String)),
		PasswordHash: password.String,
		CreatedAt:    createdAt.UTC(),
	}
	return user, nil
}

func formatIdentifier(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", errors.New("user id is nil")
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	default:
		return "", fmt.Errorf("unsupported identifier type %T", value)
	}
}
