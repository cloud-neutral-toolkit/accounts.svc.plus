package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"account/internal/auth"
	"account/internal/store"
)

const (
	bridgeBootstrapScheme   = "xworkmate-bridge-bootstrap"
	bridgeBootstrapScopeA   = "connect"
	bridgeBootstrapScopeB   = "pairing.bootstrap"
	bridgeBootstrapShortLen = 8
)

type bridgeBootstrapTicket struct {
	TicketID     string
	ShortCode    string
	TenantID     string
	UserID       string
	ProfileScope string
	TargetBridge string
	Scopes       []string
	ExpiresAt    time.Time
	OneTime      bool
	IssuedAt     time.Time
	ConsumedAt   time.Time
	RevokedAt    time.Time
}

type bridgeBootstrapIssueResponse struct {
	Ticket    string   `json:"ticket"`
	ShortCode string   `json:"shortCode"`
	Bridge    string   `json:"bridge"`
	Scheme    string   `json:"scheme"`
	ExpiresAt string   `json:"expiresAt"`
	Scopes    []string `json:"scopes"`
	OneTime   bool     `json:"oneTime"`
	QRPayload string   `json:"qrPayload"`
}

type bridgeBootstrapConsumeResponse struct {
	TicketID      string   `json:"ticketId"`
	TargetBridge  string   `json:"targetBridge"`
	OpenclawURL   string   `json:"openclawUrl"`
	AuthMode      string   `json:"authMode"`
	ExchangeToken string   `json:"exchangeToken"`
	ExpiresAt     string   `json:"expiresAt"`
	Scopes        []string `json:"scopes"`
}

func sanitizeBridgeTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultBridgeBootstrapTarget
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return defaultBridgeBootstrapTarget
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

func newBridgeBootstrapQRCode(ticketID, bridge string) string {
	payload, _ := json.Marshal(gin.H{
		"scheme": bridgeBootstrapScheme,
		"ticket": ticketID,
		"bridge": bridge,
	})
	return string(payload)
}

func newBridgeBootstrapShortCode() string {
	for {
		raw := make([]byte, 6)
		_, _ = rand.Read(raw)
		code := strings.ToUpper(
			strings.TrimRight(base64.RawURLEncoding.EncodeToString(raw), "="),
		)
		code = strings.NewReplacer("-", "A", "_", "B").Replace(code)
		if len(code) >= bridgeBootstrapShortLen {
			return code[:bridgeBootstrapShortLen]
		}
	}
}

func (h *handler) storeBridgeBootstrapTicket(ticket bridgeBootstrapTicket) {
	h.bridgeBootstrapMu.Lock()
	defer h.bridgeBootstrapMu.Unlock()
	h.bridgeBootstrapTickets[ticket.TicketID] = ticket
	h.bridgeBootstrapByCode[strings.ToUpper(ticket.ShortCode)] = ticket.TicketID
}

func (h *handler) loadBridgeBootstrapTicketByID(ticketID string) (bridgeBootstrapTicket, bool) {
	h.bridgeBootstrapMu.RLock()
	defer h.bridgeBootstrapMu.RUnlock()
	ticket, ok := h.bridgeBootstrapTickets[strings.TrimSpace(ticketID)]
	return ticket, ok
}

func (h *handler) loadBridgeBootstrapTicketByCode(shortCode string) (bridgeBootstrapTicket, bool) {
	h.bridgeBootstrapMu.RLock()
	defer h.bridgeBootstrapMu.RUnlock()
	ticketID, ok := h.bridgeBootstrapByCode[strings.ToUpper(strings.TrimSpace(shortCode))]
	if !ok {
		return bridgeBootstrapTicket{}, false
	}
	ticket, ok := h.bridgeBootstrapTickets[ticketID]
	return ticket, ok
}

func (h *handler) updateBridgeBootstrapTicket(ticket bridgeBootstrapTicket) {
	h.bridgeBootstrapMu.Lock()
	defer h.bridgeBootstrapMu.Unlock()
	h.bridgeBootstrapTickets[ticket.TicketID] = ticket
	if ticket.ShortCode != "" {
		h.bridgeBootstrapByCode[strings.ToUpper(ticket.ShortCode)] = ticket.TicketID
	}
}

func bridgeBootstrapExpired(ticket bridgeBootstrapTicket, now time.Time) bool {
	return ticket.ExpiresAt.IsZero() || now.After(ticket.ExpiresAt)
}

func requireBridgeBootstrapAccess(c *gin.Context, h *handler) (*store.User, *xworkmateAccessContext, bool) {
	user, ok := h.currentAuthenticatedUser(c)
	if !ok {
		return nil, nil, false
	}
	access, err := h.resolveXWorkmateAccess(c.Request.Context(), h.resolveTenantHost(c), user)
	if err != nil {
		respondError(c, http.StatusForbidden, "tenant_access_denied", "failed to resolve tenant access")
		return nil, nil, false
	}
	return user, access, true
}

func (h *handler) createXWorkmateBridgeBootstrapTicket(c *gin.Context) {
	user, access, ok := requireBridgeBootstrapAccess(c, h)
	if !ok {
		return
	}
	if !auth.IsMFAVerified(c) {
		respondError(c, http.StatusForbidden, "mfa_required", "mfa verification required")
		return
	}

	var req struct {
		TargetBridge string `json:"targetBridge"`
	}
	_ = c.ShouldBindJSON(&req)

	now := time.Now().UTC()
	ticket := bridgeBootstrapTicket{
		TicketID:     generateRandomState(),
		ShortCode:    newBridgeBootstrapShortCode(),
		TenantID:     strings.TrimSpace(access.Tenant.ID),
		UserID:       strings.TrimSpace(user.ID),
		ProfileScope: access.ProfileScope,
		TargetBridge: sanitizeBridgeTarget(req.TargetBridge),
		Scopes:       []string{bridgeBootstrapScopeA, bridgeBootstrapScopeB},
		ExpiresAt:    now.Add(h.bridgeBootstrapTTL),
		OneTime:      true,
		IssuedAt:     now,
	}
	h.storeBridgeBootstrapTicket(ticket)

	c.JSON(http.StatusOK, bridgeBootstrapIssueResponse{
		Ticket:    ticket.TicketID,
		ShortCode: ticket.ShortCode,
		Bridge:    ticket.TargetBridge,
		Scheme:    bridgeBootstrapScheme,
		ExpiresAt: ticket.ExpiresAt.Format(time.RFC3339),
		Scopes:    append([]string(nil), ticket.Scopes...),
		OneTime:   ticket.OneTime,
		QRPayload: newBridgeBootstrapQRCode(ticket.TicketID, ticket.TargetBridge),
	})
}

func (h *handler) lookupXWorkmateBridgeBootstrapTicket(c *gin.Context) {
	user, access, ok := requireBridgeBootstrapAccess(c, h)
	if !ok {
		return
	}
	ticket, found := h.loadBridgeBootstrapTicketByCode(c.Param("shortCode"))
	if !found || ticket.UserID != strings.TrimSpace(user.ID) || ticket.TenantID != strings.TrimSpace(access.Tenant.ID) {
		respondError(c, http.StatusNotFound, "bridge_bootstrap_not_found", "bridge bootstrap ticket not found")
		return
	}
	if bridgeBootstrapExpired(ticket, time.Now().UTC()) {
		respondError(c, http.StatusGone, "bridge_bootstrap_expired", "bridge bootstrap ticket expired")
		return
	}
	if !ticket.RevokedAt.IsZero() {
		respondError(c, http.StatusGone, "bridge_bootstrap_revoked", "bridge bootstrap ticket revoked")
		return
	}
	c.JSON(http.StatusOK, bridgeBootstrapIssueResponse{
		Ticket:    ticket.TicketID,
		ShortCode: ticket.ShortCode,
		Bridge:    ticket.TargetBridge,
		Scheme:    bridgeBootstrapScheme,
		ExpiresAt: ticket.ExpiresAt.Format(time.RFC3339),
		Scopes:    append([]string(nil), ticket.Scopes...),
		OneTime:   ticket.OneTime,
		QRPayload: newBridgeBootstrapQRCode(ticket.TicketID, ticket.TargetBridge),
	})
}

func (h *handler) revokeXWorkmateBridgeBootstrapTicket(c *gin.Context) {
	user, access, ok := requireBridgeBootstrapAccess(c, h)
	if !ok {
		return
	}
	ticket, found := h.loadBridgeBootstrapTicketByID(c.Param("ticketId"))
	if !found || ticket.UserID != strings.TrimSpace(user.ID) || ticket.TenantID != strings.TrimSpace(access.Tenant.ID) {
		respondError(c, http.StatusNotFound, "bridge_bootstrap_not_found", "bridge bootstrap ticket not found")
		return
	}
	ticket.RevokedAt = time.Now().UTC()
	h.updateBridgeBootstrapTicket(ticket)
	c.JSON(http.StatusOK, gin.H{"ok": true, "ticketId": ticket.TicketID, "revokedAt": ticket.RevokedAt.Format(time.RFC3339)})
}

func (h *handler) internalConsumeXWorkmateBridgeBootstrapTicket(c *gin.Context) {
	if !h.ensureXWorkmateVaultService(c) {
		return
	}
	var req struct {
		Ticket string `json:"ticket"`
		Bridge string `json:"bridge"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid bridge bootstrap consume request")
		return
	}
	ticket, found := h.loadBridgeBootstrapTicketByID(req.Ticket)
	if !found {
		respondError(c, http.StatusNotFound, "bridge_bootstrap_not_found", "bridge bootstrap ticket not found")
		return
	}
	now := time.Now().UTC()
	if bridgeBootstrapExpired(ticket, now) {
		respondError(c, http.StatusGone, "bridge_bootstrap_expired", "bridge bootstrap ticket expired")
		return
	}
	if !ticket.RevokedAt.IsZero() {
		respondError(c, http.StatusGone, "bridge_bootstrap_revoked", "bridge bootstrap ticket revoked")
		return
	}
	if ticket.OneTime && !ticket.ConsumedAt.IsZero() {
		respondError(c, http.StatusConflict, "bridge_bootstrap_consumed", "bridge bootstrap ticket already consumed")
		return
	}
	if sanitizeBridgeTarget(req.Bridge) != ticket.TargetBridge {
		respondError(c, http.StatusForbidden, "bridge_bootstrap_target_mismatch", "bridge bootstrap target mismatch")
		return
	}

	ctx := c.Request.Context()
	profile, err := h.loadXWorkmateProfile(
		ctx,
		&xworkmateAccessContext{
			ProfileScope: ticket.ProfileScope,
			Tenant: &store.Tenant{
				ID: ticket.TenantID,
			},
		},
		&store.User{
			ID: ticket.UserID,
		},
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "xworkmate_profile_unavailable", "failed to load xworkmate profile")
		return
	}
	if profile == nil {
		respondError(c, http.StatusNotFound, "xworkmate_profile_not_found", "xworkmate profile not found")
		return
	}
	locator, ok := findStoredXWorkmateSecretLocator(profile, store.XWorkmateSecretLocatorTargetOpenclawGatewayToken)
	if !ok {
		respondError(c, http.StatusConflict, "gateway_token_not_configured", "gateway token is not configured")
		return
	}
	gatewayToken, err := h.xworkmateVaultService.ReadSecret(ctx, locator)
	if err != nil || strings.TrimSpace(gatewayToken) == "" {
		respondError(c, http.StatusConflict, "gateway_token_unavailable", "gateway token is unavailable")
		return
	}
	openclawURL := strings.TrimSpace(profile.OpenclawURL)
	if openclawURL == "" {
		respondError(c, http.StatusConflict, "gateway_endpoint_not_configured", "gateway endpoint is not configured")
		return
	}

	ticket.ConsumedAt = now
	h.updateBridgeBootstrapTicket(ticket)

	c.JSON(http.StatusOK, bridgeBootstrapConsumeResponse{
		TicketID:      ticket.TicketID,
		TargetBridge:  ticket.TargetBridge,
		OpenclawURL:   openclawURL,
		AuthMode:      "shared-token",
		ExchangeToken: gatewayToken,
		ExpiresAt:     ticket.ExpiresAt.Format(time.RFC3339),
		Scopes:        append([]string(nil), ticket.Scopes...),
	})
}
