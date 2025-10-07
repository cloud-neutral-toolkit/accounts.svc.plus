package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// User represents an account within the account service domain.
type User struct {
	ID                string
	Name              string
	Email             string
	Level             int
	Role              string
	Groups            []string
	Permissions       []string
	EmailVerified     bool
	PasswordHash      string
	MFATOTPSecret     string
	MFAEnabled        bool
	MFASecretIssuedAt time.Time
	MFAConfirmedAt    time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Store provides persistence operations for users.
type Store interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByName(ctx context.Context, name string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
}

// Domain level errors returned by the store implementation.
var (
	ErrEmailExists                = errors.New("email already exists")
	ErrNameExists                 = errors.New("name already exists")
	ErrInvalidName                = errors.New("invalid user name")
	ErrUserNotFound               = errors.New("user not found")
	ErrMFANotSupported            = errors.New("mfa is not supported by the current store schema")
	ErrSuperAdminCountingDisabled = errors.New("super administrator counting is disabled")
)

// memoryStore provides an in-memory implementation of Store. It is suitable for
// unit tests and local development where a persistent database is not yet
// configured.
type memoryStore struct {
	mu                      sync.RWMutex
	allowSuperAdminCounting bool
	byID                    map[string]*User
	byEmail                 map[string]*User
	byName                  map[string]*User
}

// NewMemoryStore creates a new in-memory store implementation with super
// administrator counting disabled by default to avoid accidental exposure of
// privileged metadata in environments where the caller has not explicitly
// opted-in.
func NewMemoryStore() Store {
	return newMemoryStore(false)
}

// NewMemoryStoreWithSuperAdminCounting creates a new in-memory store with
// explicit permission to count super administrators. This is primarily used by
// internal tooling that needs to enforce singleton guarantees.
func NewMemoryStoreWithSuperAdminCounting() Store {
	return newMemoryStore(true)
}

func newMemoryStore(allowSuperAdminCounting bool) Store {
	return &memoryStore{
		allowSuperAdminCounting: allowSuperAdminCounting,
		byID:                    make(map[string]*User),
		byEmail:                 make(map[string]*User),
		byName:                  make(map[string]*User),
	}
}

// CreateUser persists a user in the in-memory store.
func (s *memoryStore) CreateUser(ctx context.Context, user *User) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	loweredEmail := strings.ToLower(strings.TrimSpace(user.Email))
	normalizedName := strings.TrimSpace(user.Name)

	if normalizedName == "" {
		return ErrInvalidName
	}

	normalizeUserRoleFields(user)

	if _, exists := s.byEmail[loweredEmail]; exists {
		return ErrEmailExists
	}
	if _, exists := s.byName[strings.ToLower(normalizedName)]; exists {
		return ErrNameExists
	}
	userCopy := *user
	if userCopy.ID == "" {
		userCopy.ID = uuid.NewString()
	}
	if userCopy.CreatedAt.IsZero() {
		now := time.Now().UTC()
		userCopy.CreatedAt = now
		if userCopy.UpdatedAt.IsZero() {
			userCopy.UpdatedAt = now
		}
	}
	if userCopy.UpdatedAt.IsZero() {
		userCopy.UpdatedAt = time.Now().UTC()
	}
	userCopy.Email = loweredEmail
	userCopy.Name = normalizedName
	stored := userCopy
	normalizeUserRoleFields(&stored)
	stored.Groups = cloneStringSlice(stored.Groups)
	stored.Permissions = cloneStringSlice(stored.Permissions)
	s.byID[userCopy.ID] = &stored
	if loweredEmail != "" {
		s.byEmail[loweredEmail] = &stored
	}
	s.byName[strings.ToLower(normalizedName)] = &stored
	assignUser(user, &stored)
	return nil
}

// GetUserByEmail fetches a user by email, returning ErrUserNotFound when the
// user does not exist.
func (s *memoryStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.byEmail[strings.ToLower(email)]
	if !ok {
		return nil, ErrUserNotFound
	}
	return cloneUser(user), nil
}

// GetUserByID fetches a user by unique identifier, returning ErrUserNotFound
// when absent.
func (s *memoryStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.byID[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return cloneUser(user), nil
}

// GetUserByName fetches a user by case-insensitive username, returning
// ErrUserNotFound when absent.
func (s *memoryStore) GetUserByName(ctx context.Context, name string) (*User, error) {
	_ = ctx
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return nil, ErrUserNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.byName[normalized]
	if !ok {
		return nil, ErrUserNotFound
	}

	return cloneUser(user), nil
}

// UpdateUser replaces the persisted user representation in memory.
func (s *memoryStore) UpdateUser(ctx context.Context, user *User) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.byID[user.ID]
	if !ok {
		return ErrUserNotFound
	}

	normalizedName := strings.TrimSpace(user.Name)
	loweredEmail := strings.ToLower(strings.TrimSpace(user.Email))

	if normalizedName == "" {
		return ErrInvalidName
	}

	// Re-index username if it changed.
	oldNameKey := strings.ToLower(existing.Name)
	newNameKey := strings.ToLower(normalizedName)
	if oldNameKey != newNameKey {
		if _, exists := s.byName[newNameKey]; exists {
			return ErrNameExists
		}
		delete(s.byName, oldNameKey)
	}

	// Re-index email if it changed.
	oldEmailKey := strings.ToLower(existing.Email)
	if oldEmailKey != loweredEmail {
		if loweredEmail != "" {
			if _, exists := s.byEmail[loweredEmail]; exists {
				return ErrEmailExists
			}
		}
		if oldEmailKey != "" {
			delete(s.byEmail, oldEmailKey)
		}
	}

	updated := *existing
	updated.Name = normalizedName
	updated.Email = loweredEmail
	updated.EmailVerified = user.EmailVerified
	updated.PasswordHash = user.PasswordHash
	updated.MFATOTPSecret = user.MFATOTPSecret
	updated.MFAEnabled = user.MFAEnabled
	updated.MFASecretIssuedAt = user.MFASecretIssuedAt
	updated.MFAConfirmedAt = user.MFAConfirmedAt
	updated.Level = user.Level
	updated.Role = user.Role
	updated.Groups = cloneStringSlice(user.Groups)
	updated.Permissions = cloneStringSlice(user.Permissions)
	normalizeUserRoleFields(&updated)
	if user.CreatedAt.IsZero() {
		updated.CreatedAt = existing.CreatedAt
	} else {
		updated.CreatedAt = user.CreatedAt
	}
	if user.UpdatedAt.IsZero() {
		updated.UpdatedAt = time.Now().UTC()
	} else {
		updated.UpdatedAt = user.UpdatedAt
	}

	s.byID[user.ID] = &updated
	s.byName[newNameKey] = &updated
	if loweredEmail != "" {
		s.byEmail[loweredEmail] = &updated
	}

	assignUser(user, &updated)
	return nil
}

// CountSuperAdmins returns the number of users configured as super administrators.
func (s *memoryStore) CountSuperAdmins(ctx context.Context) (int, error) {
	_ = ctx
	if !s.allowSuperAdminCounting {
		return 0, ErrSuperAdminCountingDisabled
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, user := range s.byID {
		if isSuperAdmin(user) {
			count++
		}
	}
	return count, nil
}

const (
	// LevelAdmin is the numeric level for administrator accounts.
	LevelAdmin = 0
	// LevelOperator is the numeric level for operator accounts.
	LevelOperator = 10
	// LevelUser is the numeric level for standard user accounts.
	LevelUser = 20
)

const (
	// RoleAdmin identifies administrator accounts.
	RoleAdmin = "admin"
	// RoleOperator identifies operator accounts.
	RoleOperator = "operator"
	// RoleUser identifies standard user accounts.
	RoleUser = "user"
)

var (
	roleToLevel = map[string]int{
		RoleAdmin:    LevelAdmin,
		RoleOperator: LevelOperator,
		RoleUser:     LevelUser,
	}
	levelToRole = map[int]string{
		LevelAdmin:    RoleAdmin,
		LevelOperator: RoleOperator,
		LevelUser:     RoleUser,
	}
)

func normalizeUserRoleFields(user *User) {
	if user == nil {
		return
	}

	normalizedRole := strings.ToLower(strings.TrimSpace(user.Role))
	if level, ok := roleToLevel[normalizedRole]; ok {
		user.Role = normalizedRole
		user.Level = level
	} else if role, ok := levelToRole[user.Level]; ok {
		user.Role = role
	} else {
		user.Role = RoleUser
		user.Level = LevelUser
	}

	user.Groups = normalizeStringSlice(user.Groups)
	user.Permissions = normalizeStringSlice(user.Permissions)
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	clone := make([]string, len(values))
	copy(clone, values)
	return clone
}

func cloneUser(user *User) *User {
	if user == nil {
		return nil
	}
	clone := *user
	clone.Groups = cloneStringSlice(user.Groups)
	clone.Permissions = cloneStringSlice(user.Permissions)
	normalizeUserRoleFields(&clone)
	return &clone
}

func assignUser(dst, src *User) {
	*dst = *src
	dst.Groups = cloneStringSlice(src.Groups)
	dst.Permissions = cloneStringSlice(src.Permissions)
	normalizeUserRoleFields(dst)
}

func isSuperAdmin(user *User) bool {
	if user == nil {
		return false
	}
	if strings.ToLower(strings.TrimSpace(user.Role)) != RoleAdmin && user.Level != LevelAdmin {
		return false
	}

	hasWildcard := false
	for _, permission := range user.Permissions {
		if strings.TrimSpace(permission) == "*" {
			hasWildcard = true
			break
		}
	}
	if !hasWildcard {
		return false
	}

	for _, group := range user.Groups {
		if strings.EqualFold(strings.TrimSpace(group), "Admin") {
			return true
		}
	}

	return false
}
