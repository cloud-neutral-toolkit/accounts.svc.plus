package main

import (
	"context"
	"log/slog"
	"testing"

	"account/config"
	"account/internal/store"
)

func TestEnsureSharedReviewXWorkmateProfileBootstrapsManagedBridgeContract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewMemoryStore()
	if err := st.EnsureTenant(ctx, &store.Tenant{
		ID:      store.SharedXWorkmateTenantID,
		Name:    store.SharedXWorkmateTenantName,
		Edition: store.SharedPublicTenantEdition,
	}); err != nil {
		t.Fatalf("ensure tenant: %v", err)
	}

	writes := make([]struct {
		locator store.XWorkmateSecretLocator
		value   string
	}, 0, 1)
	err := ensureSharedReviewXWorkmateProfile(
		ctx,
		st,
		config.ReviewAccount{Enabled: true},
		sharedXWorkmateBootstrapConfig{
			BridgeServerURL: SharedXWorkmateBridgeServerURL,
			BridgeAuthToken: "bridge-token",
		},
		func(
			ctx context.Context,
			locator store.XWorkmateSecretLocator,
			value string,
		) error {
			writes = append(writes, struct {
				locator store.XWorkmateSecretLocator
				value   string
			}{locator: locator, value: value})
			return nil
		},
		slog.Default(),
	)
	if err != nil {
		t.Fatalf("ensure shared review xworkmate profile: %v", err)
	}

	profile, err := st.GetXWorkmateProfile(
		ctx,
		store.SharedXWorkmateTenantID,
		"",
		store.XWorkmateProfileScopeTenantShared,
	)
	if err != nil {
		t.Fatalf("load shared profile: %v", err)
	}

	if got := profile.BridgeServerURL; got != SharedXWorkmateBridgeServerURL {
		t.Fatalf("expected bridge server url %q, got %q", SharedXWorkmateBridgeServerURL, got)
	}
	if got := profile.BridgeServerOrigin; got != SharedXWorkmateBridgeServerURL {
		t.Fatalf("expected bridge server origin %q, got %q", SharedXWorkmateBridgeServerURL, got)
	}
	if len(profile.SecretLocators) != 1 {
		t.Fatalf("expected 1 secret locator, got %d", len(profile.SecretLocators))
	}
	locator := profile.SecretLocators[0]
	if locator.Target != store.XWorkmateSecretLocatorTargetBridgeAuthToken {
		t.Fatalf("expected bridge auth token locator, got %#v", locator)
	}
	if locator.SecretPath != "xworkmate/tenants/svc-plus-xworkmate/shared" {
		t.Fatalf("expected managed shared secret path, got %#v", locator)
	}
	if len(writes) != 1 {
		t.Fatalf("expected 1 secret write, got %d", len(writes))
	}
	if writes[0].value != "bridge-token" {
		t.Fatalf("expected secret value bridge-token, got %q", writes[0].value)
	}
}

func TestEnsureSharedReviewXWorkmateProfileRequiresBridgeContract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewMemoryStore()
	if err := st.EnsureTenant(ctx, &store.Tenant{
		ID:      store.SharedXWorkmateTenantID,
		Name:    store.SharedXWorkmateTenantName,
		Edition: store.SharedPublicTenantEdition,
	}); err != nil {
		t.Fatalf("ensure tenant: %v", err)
	}

	err := ensureSharedReviewXWorkmateProfile(
		ctx,
		st,
		config.ReviewAccount{Enabled: true},
		sharedXWorkmateBootstrapConfig{
			BridgeServerURL: SharedXWorkmateBridgeServerURL,
		},
		func(context.Context, store.XWorkmateSecretLocator, string) error {
			return nil
		},
		nil,
	)
	if err == nil || err.Error() != "shared xworkmate bridge auth token is required" {
		t.Fatalf("expected missing bridge token error, got %v", err)
	}
}
