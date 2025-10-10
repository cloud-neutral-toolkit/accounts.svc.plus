package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// AccountDump represents the serialized snapshot of account-related tables.
type AccountDump struct {
	Users      []UserRecord     `yaml:"users"`
	Identities []IdentityRecord `yaml:"identities,omitempty"`
	Sessions   []SessionRecord  `yaml:"sessions,omitempty"`
}

// UserRecord captures the exported representation of a user row.
type UserRecord struct {
	UUID              string     `yaml:"uuid"`
	Username          string     `yaml:"username"`
	PasswordHash      string     `yaml:"password"`
	Email             string     `yaml:"email,omitempty"`
	EmailVerified     bool       `yaml:"emailVerified"`
	EmailVerifiedAt   *time.Time `yaml:"emailVerifiedAt,omitempty"`
	Level             int        `yaml:"level"`
	Role              string     `yaml:"role"`
	Groups            []string   `yaml:"groups,omitempty"`
	Permissions       []string   `yaml:"permissions,omitempty"`
	CreatedAt         time.Time  `yaml:"createdAt"`
	UpdatedAt         time.Time  `yaml:"updatedAt"`
	MFATOTPSecret     string     `yaml:"mfaTotpSecret,omitempty"`
	MFAEnabled        bool       `yaml:"mfaEnabled"`
	MFASecretIssuedAt *time.Time `yaml:"mfaSecretIssuedAt,omitempty"`
	MFAConfirmedAt    *time.Time `yaml:"mfaConfirmedAt,omitempty"`
}

// IdentityRecord captures a federated identity row associated with a user.
type IdentityRecord struct {
	UUID       string     `yaml:"uuid"`
	Provider   string     `yaml:"provider"`
	ExternalID string     `yaml:"externalId"`
	UserUUID   string     `yaml:"userUuid"`
	CreatedAt  *time.Time `yaml:"createdAt,omitempty"`
	UpdatedAt  *time.Time `yaml:"updatedAt,omitempty"`
}

// SessionRecord captures a session row associated with a user.
type SessionRecord struct {
	UUID      string     `yaml:"uuid"`
	Token     string     `yaml:"token"`
	ExpiresAt time.Time  `yaml:"expiresAt"`
	UserUUID  string     `yaml:"userUuid"`
	CreatedAt *time.Time `yaml:"createdAt,omitempty"`
	UpdatedAt *time.Time `yaml:"updatedAt,omitempty"`
}

// Exporter reads account data from a PostgreSQL database.
type Exporter struct{}

// NewExporter constructs an Exporter instance.
func NewExporter() *Exporter {
	return &Exporter{}
}

// Export fetches user-related data filtered by the provided email keyword. When
// emailKeyword is empty all users are included.
func (e *Exporter) Export(ctx context.Context, dsn, emailKeyword string) (*AccountDump, error) {
	db, err := openDB(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dump := &AccountDump{}

	users, err := loadUsers(ctx, db, emailKeyword)
	if err != nil {
		return nil, err
	}
	dump.Users = users

	if len(users) == 0 {
		return dump, nil
	}

	uuids := make([]string, len(users))
	for i, user := range users {
		uuids[i] = user.UUID
	}

	identities, err := loadIdentities(ctx, db, uuids)
	if err != nil {
		return nil, err
	}
	dump.Identities = identities

	sessions, err := loadSessions(ctx, db, uuids)
	if err != nil {
		return nil, err
	}
	dump.Sessions = sessions

	return dump, nil
}

// Importer writes account data into a PostgreSQL database.
type Importer struct{}

// NewImporter constructs an Importer instance.
func NewImporter() *Importer {
	return &Importer{}
}

// Import restores account data from a dump into the target database. Existing
// rows are replaced on conflict and related identities/sessions are refreshed.
func (i *Importer) Import(ctx context.Context, dsn string, dump *AccountDump) error {
	if dump == nil {
		return errors.New("dump is nil")
	}

	db, err := openDB(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	identityCaps, err := tableColumnCaps(ctx, db, "identities")
	if err != nil {
		return err
	}
	sessionCaps, err := tableColumnCaps(ctx, db, "sessions")
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	cleared := make(map[string]struct{})

	for _, user := range dump.Users {
		if err = upsertUser(ctx, tx, &user); err != nil {
			return err
		}
		cleared[user.UUID] = struct{}{}
	}

	for uuid := range cleared {
		if _, err = tx.ExecContext(ctx, `DELETE FROM identities WHERE user_uuid = $1`, uuid); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_uuid = $1`, uuid); err != nil {
			return err
		}
	}

	for _, identity := range dump.Identities {
		if err = upsertIdentity(ctx, tx, &identity, identityCaps); err != nil {
			return err
		}
	}

	for _, session := range dump.Sessions {
		if err = upsertSession(ctx, tx, &session, sessionCaps); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func loadUsers(ctx context.Context, db *sql.DB, emailKeyword string) ([]UserRecord, error) {
	var (
		query strings.Builder
		args  []any
	)

	query.WriteString(`SELECT uuid, username, password, email, email_verified, email_verified_at, level, role, groups, permissions, created_at, updated_at, mfa_totp_secret, mfa_enabled, mfa_secret_issued_at, mfa_confirmed_at FROM users`)
	if keyword := strings.TrimSpace(emailKeyword); keyword != "" {
		query.WriteString(` WHERE email ILIKE $1`)
		args = append(args, "%"+keyword+"%")
	}
	query.WriteString(` ORDER BY created_at ASC`)

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var (
			email           sql.NullString
			emailVerified   bool
			emailVerifiedAt sql.NullTime
			level           sql.NullInt64
			role            sql.NullString
			groupsRaw       []byte
			permissionsRaw  []byte
			createdAt       time.Time
			updatedAt       time.Time
			mfaSecret       sql.NullString
			mfaEnabled      sql.NullBool
			mfaIssuedAt     sql.NullTime
			mfaConfirmedAt  sql.NullTime
			user            UserRecord
		)

		if err := rows.Scan(
			&user.UUID,
			&user.Username,
			&user.PasswordHash,
			&email,
			&emailVerified,
			&emailVerifiedAt,
			&level,
			&role,
			&groupsRaw,
			&permissionsRaw,
			&createdAt,
			&updatedAt,
			&mfaSecret,
			&mfaEnabled,
			&mfaIssuedAt,
			&mfaConfirmedAt,
		); err != nil {
			return nil, err
		}

		if email.Valid {
			user.Email = email.String
		}
		user.EmailVerified = emailVerified
		if emailVerifiedAt.Valid {
			ts := emailVerifiedAt.Time
			user.EmailVerifiedAt = &ts
		}
		if level.Valid {
			user.Level = int(level.Int64)
		}
		if role.Valid {
			user.Role = role.String
		}
		if len(groupsRaw) > 0 {
			if err := json.Unmarshal(groupsRaw, &user.Groups); err != nil {
				return nil, fmt.Errorf("decode groups for user %s: %w", user.UUID, err)
			}
		}
		if len(permissionsRaw) > 0 {
			if err := json.Unmarshal(permissionsRaw, &user.Permissions); err != nil {
				return nil, fmt.Errorf("decode permissions for user %s: %w", user.UUID, err)
			}
		}
		user.CreatedAt = createdAt
		user.UpdatedAt = updatedAt
		if mfaSecret.Valid {
			user.MFATOTPSecret = mfaSecret.String
		}
		user.MFAEnabled = mfaEnabled.Bool
		if mfaIssuedAt.Valid {
			ts := mfaIssuedAt.Time
			user.MFASecretIssuedAt = &ts
		}
		if mfaConfirmedAt.Valid {
			ts := mfaConfirmedAt.Time
			user.MFAConfirmedAt = &ts
		}
		if user.Groups == nil {
			user.Groups = []string{}
		}
		if user.Permissions == nil {
			user.Permissions = []string{}
		}
		if user.Role == "" {
			user.Role = "user"
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func loadIdentities(ctx context.Context, db *sql.DB, uuids []string) ([]IdentityRecord, error) {
	if len(uuids) == 0 {
		return nil, nil
	}

	caps, err := tableColumnCaps(ctx, db, "identities")
	if err != nil {
		return nil, err
	}

	columns := []string{"uuid", "provider", "external_id", "user_uuid"}
	if caps.hasCreatedAt {
		columns = append(columns, "created_at")
	}
	if caps.hasUpdatedAt {
		columns = append(columns, "updated_at")
	}

	orderClause := " ORDER BY uuid ASC"
	if caps.hasCreatedAt {
		orderClause = " ORDER BY created_at ASC"
	}

	format := fmt.Sprintf("SELECT %s FROM identities WHERE user_uuid IN (%%s)%s", strings.Join(columns, ", "), orderClause)
	query, args := buildInQuery(format, uuids)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []IdentityRecord
	for rows.Next() {
		var identity IdentityRecord
		scanArgs := []any{
			&identity.UUID,
			&identity.Provider,
			&identity.ExternalID,
			&identity.UserUUID,
		}

		var createdAt, updatedAt sql.NullTime
		if caps.hasCreatedAt {
			scanArgs = append(scanArgs, &createdAt)
		}
		if caps.hasUpdatedAt {
			scanArgs = append(scanArgs, &updatedAt)
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}

		if caps.hasCreatedAt && createdAt.Valid {
			ts := createdAt.Time
			identity.CreatedAt = &ts
		}
		if caps.hasUpdatedAt && updatedAt.Valid {
			ts := updatedAt.Time
			identity.UpdatedAt = &ts
		}
		identities = append(identities, identity)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(identities, func(i, j int) bool {
		if identities[i].UserUUID == identities[j].UserUUID {
			switch {
			case identities[i].CreatedAt == nil && identities[j].CreatedAt == nil:
				return identities[i].UUID < identities[j].UUID
			case identities[i].CreatedAt == nil:
				return false
			case identities[j].CreatedAt == nil:
				return true
			default:
				return identities[i].CreatedAt.Before(*identities[j].CreatedAt)
			}
		}
		return identities[i].UserUUID < identities[j].UserUUID
	})

	return identities, nil
}

func loadSessions(ctx context.Context, db *sql.DB, uuids []string) ([]SessionRecord, error) {
	if len(uuids) == 0 {
		return nil, nil
	}

	caps, err := tableColumnCaps(ctx, db, "sessions")
	if err != nil {
		return nil, err
	}

	columns := []string{"uuid", "token", "expires_at", "user_uuid"}
	if caps.hasCreatedAt {
		columns = append(columns, "created_at")
	}
	if caps.hasUpdatedAt {
		columns = append(columns, "updated_at")
	}

	orderClause := " ORDER BY uuid ASC"
	if caps.hasCreatedAt {
		orderClause = " ORDER BY created_at ASC"
	}

	format := fmt.Sprintf("SELECT %s FROM sessions WHERE user_uuid IN (%%s)%s", strings.Join(columns, ", "), orderClause)
	query, args := buildInQuery(format, uuids)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionRecord
	for rows.Next() {
		var session SessionRecord
		scanArgs := []any{
			&session.UUID,
			&session.Token,
			&session.ExpiresAt,
			&session.UserUUID,
		}

		var createdAt, updatedAt sql.NullTime
		if caps.hasCreatedAt {
			scanArgs = append(scanArgs, &createdAt)
		}
		if caps.hasUpdatedAt {
			scanArgs = append(scanArgs, &updatedAt)
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}

		if caps.hasCreatedAt && createdAt.Valid {
			ts := createdAt.Time
			session.CreatedAt = &ts
		}
		if caps.hasUpdatedAt && updatedAt.Valid {
			ts := updatedAt.Time
			session.UpdatedAt = &ts
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].UserUUID == sessions[j].UserUUID {
			switch {
			case sessions[i].CreatedAt == nil && sessions[j].CreatedAt == nil:
				return sessions[i].UUID < sessions[j].UUID
			case sessions[i].CreatedAt == nil:
				return false
			case sessions[j].CreatedAt == nil:
				return true
			default:
				return sessions[i].CreatedAt.Before(*sessions[j].CreatedAt)
			}
		}
		return sessions[i].UserUUID < sessions[j].UserUUID
	})

	return sessions, nil
}

func buildInQuery(format string, uuids []string) (string, []any) {
	placeholders := make([]string, len(uuids))
	args := make([]any, len(uuids))
	for i, id := range uuids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	return fmt.Sprintf(format, strings.Join(placeholders, ", ")), args
}

func upsertUser(ctx context.Context, tx *sql.Tx, user *UserRecord) error {
	groupsJSON, err := json.Marshal(user.Groups)
	if err != nil {
		return fmt.Errorf("encode groups for user %s: %w", user.UUID, err)
	}
	permissionsJSON, err := json.Marshal(user.Permissions)
	if err != nil {
		return fmt.Errorf("encode permissions for user %s: %w", user.UUID, err)
	}

	if user.EmailVerifiedAt == nil && user.EmailVerified {
		ts := user.UpdatedAt
		if ts.IsZero() {
			ts = user.CreatedAt
		}
		user.EmailVerifiedAt = &ts
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO users (
        uuid, username, password, email, email_verified_at,
        level, role, groups, permissions, created_at, updated_at,
        mfa_totp_secret, mfa_enabled, mfa_secret_issued_at, mfa_confirmed_at
) VALUES (
        $1, $2, $3, $4, $5,
        $6, $7, $8::jsonb, $9::jsonb, $10, $11,
        $12, $13, $14, $15
)
ON CONFLICT (uuid) DO UPDATE SET
        username = EXCLUDED.username,
        password = EXCLUDED.password,
        email = EXCLUDED.email,
        email_verified_at = EXCLUDED.email_verified_at,
        level = EXCLUDED.level,
        role = EXCLUDED.role,
        groups = EXCLUDED.groups,
        permissions = EXCLUDED.permissions,
        created_at = EXCLUDED.created_at,
        updated_at = EXCLUDED.updated_at,
        mfa_totp_secret = EXCLUDED.mfa_totp_secret,
        mfa_enabled = EXCLUDED.mfa_enabled,
        mfa_secret_issued_at = EXCLUDED.mfa_secret_issued_at,
        mfa_confirmed_at = EXCLUDED.mfa_confirmed_at
`,
		user.UUID,
		user.Username,
		user.PasswordHash,
		nullableString(user.Email),
		nullableTime(user.EmailVerifiedAt),
		user.Level,
		user.Role,
		string(groupsJSON),
		string(permissionsJSON),
		user.CreatedAt,
		user.UpdatedAt,
		nullableString(user.MFATOTPSecret),
		user.MFAEnabled,
		nullableTime(user.MFASecretIssuedAt),
		nullableTime(user.MFAConfirmedAt),
	)
	return err
}

func upsertIdentity(ctx context.Context, tx *sql.Tx, identity *IdentityRecord, caps tableColumnCapabilities) error {
	columns := []string{"uuid", "provider", "external_id", "user_uuid"}
	placeholders := []string{"$1", "$2", "$3", "$4"}
	args := []any{identity.UUID, identity.Provider, identity.ExternalID, identity.UserUUID}

	nextIdx := 5
	if caps.hasCreatedAt {
		columns = append(columns, "created_at")
		placeholders = append(placeholders, fmt.Sprintf("$%d", nextIdx))
		args = append(args, nullableTime(identity.CreatedAt))
		nextIdx++
	}
	if caps.hasUpdatedAt {
		columns = append(columns, "updated_at")
		placeholders = append(placeholders, fmt.Sprintf("$%d", nextIdx))
		args = append(args, nullableTime(identity.UpdatedAt))
		nextIdx++
	}

	query := fmt.Sprintf(`
INSERT INTO identities (%s)
VALUES (%s)
ON CONFLICT (uuid) DO UPDATE SET
        provider = EXCLUDED.provider,
        external_id = EXCLUDED.external_id,
        user_uuid = EXCLUDED.user_uuid%s%s
`,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		updateColumnClause(caps.hasCreatedAt, "created_at"),
		updateColumnClause(caps.hasUpdatedAt, "updated_at"),
	)

	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func upsertSession(ctx context.Context, tx *sql.Tx, session *SessionRecord, caps tableColumnCapabilities) error {
	columns := []string{"uuid", "token", "expires_at", "user_uuid"}
	placeholders := []string{"$1", "$2", "$3", "$4"}
	args := []any{session.UUID, session.Token, session.ExpiresAt, session.UserUUID}

	nextIdx := 5
	if caps.hasCreatedAt {
		columns = append(columns, "created_at")
		placeholders = append(placeholders, fmt.Sprintf("$%d", nextIdx))
		args = append(args, nullableTime(session.CreatedAt))
		nextIdx++
	}
	if caps.hasUpdatedAt {
		columns = append(columns, "updated_at")
		placeholders = append(placeholders, fmt.Sprintf("$%d", nextIdx))
		args = append(args, nullableTime(session.UpdatedAt))
		nextIdx++
	}

	query := fmt.Sprintf(`
INSERT INTO sessions (%s)
VALUES (%s)
ON CONFLICT (uuid) DO UPDATE SET
        token = EXCLUDED.token,
        expires_at = EXCLUDED.expires_at,
        user_uuid = EXCLUDED.user_uuid%s%s
`,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		updateColumnClause(caps.hasCreatedAt, "created_at"),
		updateColumnClause(caps.hasUpdatedAt, "updated_at"),
	)

	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

type tableColumnCapabilities struct {
	hasCreatedAt bool
	hasUpdatedAt bool
}

func tableColumnCaps(ctx context.Context, db *sql.DB, table string) (tableColumnCapabilities, error) {
	query := `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = ANY (current_schemas(false))
  AND table_name = $1
  AND column_name IN ('created_at', 'updated_at')
`

	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return tableColumnCapabilities{}, err
	}
	defer rows.Close()

	caps := tableColumnCapabilities{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return tableColumnCapabilities{}, err
		}
		switch name {
		case "created_at":
			caps.hasCreatedAt = true
		case "updated_at":
			caps.hasUpdatedAt = true
		}
	}
	if err := rows.Err(); err != nil {
		return tableColumnCapabilities{}, err
	}

	return caps, nil
}

func updateColumnClause(enabled bool, column string) string {
	if !enabled {
		return ""
	}
	return fmt.Sprintf(", %s = EXCLUDED.%s", column, column)
}
