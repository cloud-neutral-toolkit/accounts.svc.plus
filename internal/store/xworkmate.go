package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	SharedPublicTenantEdition = "shared_public"
	TenantPrivateEdition      = "tenant_private"

	TenantMembershipRoleAdmin = "admin"
	TenantMembershipRoleUser  = "user"

	TenantDomainKindGenerated = "generated"
	TenantDomainKindCustom    = "custom"

	TenantDomainStatusPending  = "pending"
	TenantDomainStatusVerified = "verified"

	XWorkmateProfileScopeTenantShared = "tenant-shared"
	XWorkmateProfileScopeUserPrivate  = "user-private"

	SharedXWorkmateTenantID   = "svc-plus-xworkmate"
	SharedXWorkmateTenantName = "svc.plus XWorkmate"
	SharedXWorkmateDomain     = "svc.plus"
)

var (
	ErrTenantNotFound           = errors.New("tenant not found")
	ErrTenantMembershipNotFound = errors.New("tenant membership not found")
	ErrXWorkmateProfileNotFound = errors.New("xworkmate profile not found")
)

type Tenant struct {
	ID        string
	Name      string
	Edition   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TenantDomain struct {
	ID        string
	TenantID  string
	Domain    string
	Kind      string
	IsPrimary bool
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TenantMembership struct {
	TenantID      string
	UserID        string
	Role          string
	TenantName    string
	TenantEdition string
	Domain        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type XWorkmateProfile struct {
	ID              string
	TenantID        string
	UserID          string
	Scope           string
	OpenclawURL     string
	OpenclawOrigin  string
	VaultURL        string
	VaultNamespace  string
	VaultSecretPath string
	VaultSecretKey  string
	ApisixURL       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NormalizeTenantEdition(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SharedPublicTenantEdition:
		return SharedPublicTenantEdition
	default:
		return TenantPrivateEdition
	}
}

func NormalizeTenantMembershipRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TenantMembershipRoleAdmin:
		return TenantMembershipRoleAdmin
	default:
		return TenantMembershipRoleUser
	}
}

func NormalizeTenantDomainKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TenantDomainKindCustom:
		return TenantDomainKindCustom
	default:
		return TenantDomainKindGenerated
	}
}

func NormalizeTenantDomainStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TenantDomainStatusPending:
		return TenantDomainStatusPending
	default:
		return TenantDomainStatusVerified
	}
}

func NormalizeXWorkmateProfileScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case XWorkmateProfileScopeTenantShared:
		return XWorkmateProfileScopeTenantShared
	default:
		return XWorkmateProfileScopeUserPrivate
	}
}

func NormalizeHostname(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if comma := strings.Index(trimmed, ","); comma >= 0 {
		trimmed = strings.TrimSpace(trimmed[:comma])
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		trimmed = parsed.Host
	}

	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		trimmed = host
	}

	return strings.Trim(strings.ToLower(trimmed), ".")
}

func IsSharedTenantHost(host string) bool {
	normalized := NormalizeHostname(host)
	if normalized == "" {
		return true
	}
	switch normalized {
	case "svc.plus", "www.svc.plus", "console.svc.plus", "localhost", "127.0.0.1", "[::1]":
		return true
	default:
		return false
	}
}

func GenerateRandomTenantDomain() (string, error) {
	buffer := make([]byte, 4)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate tenant domain: %w", err)
	}
	return fmt.Sprintf("xw-%s.svc.plus", hex.EncodeToString(buffer)), nil
}

func NormalizeTenant(tenant *Tenant) {
	if tenant == nil {
		return
	}
	tenant.ID = strings.TrimSpace(tenant.ID)
	tenant.Name = strings.TrimSpace(tenant.Name)
	tenant.Edition = NormalizeTenantEdition(tenant.Edition)
}

func NormalizeTenantDomain(domain *TenantDomain) {
	if domain == nil {
		return
	}
	domain.ID = strings.TrimSpace(domain.ID)
	domain.TenantID = strings.TrimSpace(domain.TenantID)
	domain.Domain = NormalizeHostname(domain.Domain)
	domain.Kind = NormalizeTenantDomainKind(domain.Kind)
	domain.Status = NormalizeTenantDomainStatus(domain.Status)
}

func NormalizeTenantMembership(membership *TenantMembership) {
	if membership == nil {
		return
	}
	membership.TenantID = strings.TrimSpace(membership.TenantID)
	membership.UserID = strings.TrimSpace(membership.UserID)
	membership.Role = NormalizeTenantMembershipRole(membership.Role)
	membership.TenantName = strings.TrimSpace(membership.TenantName)
	membership.TenantEdition = NormalizeTenantEdition(membership.TenantEdition)
	membership.Domain = NormalizeHostname(membership.Domain)
}

func NormalizeXWorkmateProfile(profile *XWorkmateProfile) {
	if profile == nil {
		return
	}
	profile.ID = strings.TrimSpace(profile.ID)
	profile.TenantID = strings.TrimSpace(profile.TenantID)
	profile.UserID = strings.TrimSpace(profile.UserID)
	profile.Scope = NormalizeXWorkmateProfileScope(profile.Scope)
	profile.OpenclawURL = strings.TrimSpace(profile.OpenclawURL)
	profile.OpenclawOrigin = strings.TrimSpace(profile.OpenclawOrigin)
	profile.VaultURL = strings.TrimSpace(profile.VaultURL)
	profile.VaultNamespace = strings.TrimSpace(profile.VaultNamespace)
	profile.VaultSecretPath = strings.Trim(strings.TrimSpace(profile.VaultSecretPath), "/")
	profile.VaultSecretKey = strings.TrimSpace(profile.VaultSecretKey)
	profile.ApisixURL = strings.TrimSpace(profile.ApisixURL)
}

type TenantResolver interface {
	EnsureTenant(ctx context.Context, tenant *Tenant) error
	EnsureTenantDomain(ctx context.Context, domain *TenantDomain) error
	UpsertTenantMembership(ctx context.Context, membership *TenantMembership) error
	ResolveTenantByHost(ctx context.Context, host string) (*Tenant, *TenantDomain, error)
	ListTenantMembershipsByUser(ctx context.Context, userID string) ([]TenantMembership, error)
	GetTenantMembership(ctx context.Context, tenantID, userID string) (*TenantMembership, error)
	GetXWorkmateProfile(ctx context.Context, tenantID, userID, scope string) (*XWorkmateProfile, error)
	UpsertXWorkmateProfile(ctx context.Context, profile *XWorkmateProfile) error
}
