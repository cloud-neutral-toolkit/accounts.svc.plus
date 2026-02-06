package api

import (
	"context"
	"strings"
	"time"

	"account/internal/store"
)

const (
	sandboxUserEmail          = "sandbox@svc.plus"
	sandboxUUIDRotationWindow = time.Hour
)

// ensureSandboxProxyUUID enforces hourly rotation of the sandbox user's ProxyUUID.
// It is intentionally strict: only the hard-coded sandbox email is eligible.
func (h *handler) ensureSandboxProxyUUID(ctx context.Context, user *store.User) error {
	if h == nil || user == nil {
		return nil
	}
	email := strings.ToLower(strings.TrimSpace(user.Email))
	if email != sandboxUserEmail {
		return nil
	}

	now := time.Now().UTC()
	needsRotation := strings.TrimSpace(user.ProxyUUID) == "" ||
		user.ProxyUUIDExpiresAt == nil ||
		!now.Before(*user.ProxyUUIDExpiresAt)

	if !needsRotation {
		return nil
	}

	exp := now.Add(sandboxUUIDRotationWindow)
	user.ProxyUUID = generateRandomUUID()
	user.ProxyUUIDExpiresAt = &exp
	return h.store.UpdateUser(ctx, user)
}
