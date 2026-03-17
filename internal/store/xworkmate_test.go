package store

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeHostname(t *testing.T) {
	t.Parallel()

	got := NormalizeHostname("https://XW-ABCD.svc.plus:443/path?q=1")
	if got != "xw-abcd.svc.plus" {
		t.Fatalf("expected normalized host, got %q", got)
	}
}

func TestGenerateRandomTenantDomain(t *testing.T) {
	t.Parallel()

	got, err := GenerateRandomTenantDomain()
	if err != nil {
		t.Fatalf("expected domain generation to succeed: %v", err)
	}
	if !strings.HasPrefix(got, "xw-") || !strings.HasSuffix(got, ".svc.plus") {
		t.Fatalf("expected generated svc.plus tenant domain, got %q", got)
	}
}

func TestMemoryStoreResolveTenantAndProfile(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStore()

	if err := st.EnsureTenant(ctx, &Tenant{
		ID:      SharedXWorkmateTenantID,
		Name:    SharedXWorkmateTenantName,
		Edition: SharedPublicTenantEdition,
	}); err != nil {
		t.Fatalf("ensure shared tenant: %v", err)
	}
	if err := st.EnsureTenantDomain(ctx, &TenantDomain{
		TenantID:  SharedXWorkmateTenantID,
		Domain:    SharedXWorkmateDomain,
		Kind:      TenantDomainKindGenerated,
		IsPrimary: true,
		Status:    TenantDomainStatusVerified,
	}); err != nil {
		t.Fatalf("ensure shared domain: %v", err)
	}

	tenant, domain, err := st.ResolveTenantByHost(ctx, "console.svc.plus")
	if err != nil {
		t.Fatalf("resolve shared tenant: %v", err)
	}
	if tenant.ID != SharedXWorkmateTenantID {
		t.Fatalf("expected shared tenant id, got %q", tenant.ID)
	}
	if domain == nil || domain.Domain != SharedXWorkmateDomain {
		t.Fatalf("expected shared primary domain, got %#v", domain)
	}

	privateTenant := &Tenant{
		ID:      "tenant-private-1",
		Name:    "Tenant One",
		Edition: TenantPrivateEdition,
	}
	if err := st.EnsureTenant(ctx, privateTenant); err != nil {
		t.Fatalf("ensure private tenant: %v", err)
	}
	if err := st.EnsureTenantDomain(ctx, &TenantDomain{
		TenantID:  privateTenant.ID,
		Domain:    "xw-tenant-one.svc.plus",
		Kind:      TenantDomainKindGenerated,
		IsPrimary: true,
		Status:    TenantDomainStatusVerified,
	}); err != nil {
		t.Fatalf("ensure private domain: %v", err)
	}
	if err := st.UpsertTenantMembership(ctx, &TenantMembership{
		TenantID: privateTenant.ID,
		UserID:   "user-1",
		Role:     TenantMembershipRoleAdmin,
	}); err != nil {
		t.Fatalf("ensure private membership: %v", err)
	}
	if err := st.UpsertXWorkmateProfile(ctx, &XWorkmateProfile{
		TenantID:        privateTenant.ID,
		UserID:          "user-1",
		Scope:           XWorkmateProfileScopeUserPrivate,
		OpenclawURL:     "wss://openclaw.tenant-one.svc.plus",
		VaultSecretPath: "kv/openclaw",
	}); err != nil {
		t.Fatalf("upsert private profile: %v", err)
	}

	tenant, domain, err = st.ResolveTenantByHost(ctx, "https://xw-tenant-one.svc.plus")
	if err != nil {
		t.Fatalf("resolve private tenant: %v", err)
	}
	if tenant.ID != privateTenant.ID {
		t.Fatalf("expected tenant %q, got %q", privateTenant.ID, tenant.ID)
	}
	if domain == nil || domain.Domain != "xw-tenant-one.svc.plus" {
		t.Fatalf("expected tenant domain, got %#v", domain)
	}

	profile, err := st.GetXWorkmateProfile(ctx, privateTenant.ID, "user-1", XWorkmateProfileScopeUserPrivate)
	if err != nil {
		t.Fatalf("get private profile: %v", err)
	}
	if profile.OpenclawURL != "wss://openclaw.tenant-one.svc.plus" {
		t.Fatalf("expected persisted openclaw url, got %q", profile.OpenclawURL)
	}

	memberships, err := st.ListTenantMembershipsByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("list memberships: %v", err)
	}
	if len(memberships) != 1 {
		t.Fatalf("expected 1 tenant membership, got %d", len(memberships))
	}
	if memberships[0].TenantName != "Tenant One" {
		t.Fatalf("expected tenant name to be populated, got %q", memberships[0].TenantName)
	}
}
