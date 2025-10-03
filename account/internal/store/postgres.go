package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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

	query := `INSERT INTO users (username, email, password, email_verified)
          VALUES ($1, $2, $3, $4)
          RETURNING uuid, coalesce(created_at, now()), coalesce(updated_at, now()), email_verified`

	var idValue any
	var createdAt time.Time
	var updatedAt time.Time
	var emailVerified sql.NullBool
	err = s.db.QueryRowContext(ctx, query, normalizedName, normalizedEmail, user.PasswordHash, user.EmailVerified).Scan(&idValue, &createdAt, &updatedAt, &emailVerified)
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
	user.UpdatedAt = updatedAt.UTC()
	user.EmailVerified = emailVerified.Bool
	return nil
}

func (s *postgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return nil, ErrUserNotFound
	}

	query := `SELECT uuid, username, email, email_verified, password, mfa_totp_secret, coalesce(mfa_enabled, false),
          mfa_secret_issued_at, mfa_confirmed_at, coalesce(created_at, now()), coalesce(updated_at, now())
          FROM users WHERE lower(email) = $1 LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, normalized)
	return scanUser(row)
}

func (s *postgresStore) GetUserByName(ctx context.Context, name string) (*User, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return nil, ErrUserNotFound
	}

	query := `SELECT uuid, username, email, email_verified, password, mfa_totp_secret, coalesce(mfa_enabled, false),
          mfa_secret_issued_at, mfa_confirmed_at, coalesce(created_at, now()), coalesce(updated_at, now())
          FROM users WHERE lower(username) = lower($1) LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, normalized)
	return scanUser(row)
}

func (s *postgresStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT uuid, username, email, email_verified, password, mfa_totp_secret, coalesce(mfa_enabled, false),
          mfa_secret_issued_at, mfa_confirmed_at, coalesce(created_at, now()), coalesce(updated_at, now())
          FROM users WHERE uuid = $1`

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
		idValue         any
		username        sql.NullString
		email           sql.NullString
		emailVerified   sql.NullBool
		password        sql.NullString
		mfaSecret       sql.NullString
		mfaEnabled      sql.NullBool
		mfaSecretIssued sql.NullTime
		mfaConfirmed    sql.NullTime
		createdAt       time.Time
		updatedAt       time.Time
	)

	if err := row.Scan(&idValue, &username, &email, &emailVerified, &password, &mfaSecret, &mfaEnabled, &mfaSecretIssued, &mfaConfirmed, &createdAt, &updatedAt); err != nil {
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
		ID:                identifier,
		Name:              strings.TrimSpace(username.String),
		Email:             strings.ToLower(strings.TrimSpace(email.String)),
		EmailVerified:     emailVerified.Bool,
		PasswordHash:      password.String,
		MFATOTPSecret:     strings.TrimSpace(mfaSecret.String),
		MFAEnabled:        mfaEnabled.Bool,
		MFASecretIssuedAt: toUTCTime(mfaSecretIssued),
		MFAConfirmedAt:    toUTCTime(mfaConfirmed),
		CreatedAt:         createdAt.UTC(),
		UpdatedAt:         updatedAt.UTC(),
	}
	return user, nil
}

func (s *postgresStore) UpdateUser(ctx context.Context, user *User) error {
	normalizedName := strings.TrimSpace(user.Name)
	if normalizedName == "" {
		return ErrInvalidName
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))
	var issuedAt any
	if !user.MFASecretIssuedAt.IsZero() {
		issuedAt = user.MFASecretIssuedAt.UTC()
	}
	var confirmedAt any
	if !user.MFAConfirmedAt.IsZero() {
		confirmedAt = user.MFAConfirmedAt.UTC()
	}

	query := `UPDATE users
          SET username = $1,
              email = $2,
              email_verified = $3,
              password = $4,
              mfa_totp_secret = $5,
              mfa_enabled = $6,
              mfa_secret_issued_at = $7,
              mfa_confirmed_at = $8,
              updated_at = now()
          WHERE uuid = $9
          RETURNING coalesce(created_at, now()), coalesce(updated_at, now())`

	var createdAt time.Time
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, query, normalizedName, normalizedEmail, user.EmailVerified, user.PasswordHash, nullForEmpty(user.MFATOTPSecret), user.MFAEnabled, issuedAt, confirmedAt, user.ID).Scan(&createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
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

	user.Name = normalizedName
	user.Email = normalizedEmail
	user.CreatedAt = createdAt.UTC()
	user.UpdatedAt = updatedAt.UTC()
	return nil
}

func nullForEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func toUTCTime(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}

func formatIdentifier(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", errors.New("user id is nil")
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case [16]byte:
		id := uuid.UUID(v)
		return id.String(), nil
	case *[16]byte:
		if v == nil {
			return "", errors.New("user id is nil")
		}
		id := uuid.UUID(*v)
		return id.String(), nil
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
	case pgtype.UUID:
		if !v.Valid {
			return "", errors.New("user id is nil")
		}
		return v.String(), nil
	case *pgtype.UUID:
		if v == nil || !v.Valid {
			return "", errors.New("user id is nil")
		}
		return v.String(), nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return "", fmt.Errorf("unsupported identifier type %T", value)
	}
}
