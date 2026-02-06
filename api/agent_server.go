package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"account/internal/agentproto"
	"account/internal/agentserver"
	"account/internal/store"
	"account/internal/xrayconfig"
)

const agentIDHeader = "X-Agent-ID"

func (h *handler) listAgentUsers(c *gin.Context) {
	if h.agentRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_registry_unavailable"})
		return
	}

	token := extractToken(c.GetHeader("Authorization"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_token"})
		return
	}

	credIdentity, ok := h.agentRegistry.Authenticate(token)
	if !ok || credIdentity == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}

	agentID := strings.TrimSpace(c.GetHeader(agentIDHeader))
	if agentID == "" {
		agentID = strings.TrimSpace(c.Query("agentId"))
	}
	if agentID == "" {
		agentID = credIdentity.ID
	}

	identity := *credIdentity
	if agentID != "" && agentID != identity.ID {
		// Shared token scenario: register a concrete agent id so sandbox bindings can target it.
		identity = h.agentRegistry.RegisterAgent(agentID, identity.Groups)
	}

	now := time.Now().UTC()
	clients := make([]xrayconfig.Client, 0, 1)

	if h.agentRegistry.IsSandboxAgent(identity.ID) {
		sandboxUser, err := h.store.GetUserByEmail(c.Request.Context(), sandboxUserEmail)
		if err != nil {
			if err == store.ErrUserNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "sandbox_missing"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "sandbox_lookup_failed"})
			return
		}
		if err := h.ensureSandboxProxyUUID(c.Request.Context(), sandboxUser); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "sandbox_uuid_rotation_failed"})
			return
		}

		uuid := strings.TrimSpace(sandboxUser.ProxyUUID)
		if uuid == "" {
			uuid = strings.TrimSpace(sandboxUser.ID)
		}
		if uuid != "" {
			clients = append(clients, xrayconfig.Client{
				ID:    uuid,
				Email: sandboxUserEmail,
				Flow:  xrayconfig.DefaultFlow,
			})
		}

		c.JSON(http.StatusOK, agentproto.ClientListResponse{
			Clients:     clients,
			Total:       len(clients),
			GeneratedAt: now,
		})
		return
	}

	// Default demo behaviour: the special sandbox user is available on all nodes/regions.
	// This keeps the Guest/Demo experience consistent even when the user switches regions.
	// It is safe because sandbox@svc.plus is a read-only demo identity with a rotating proxy UUID.
	if h.store != nil {
		if sandboxUser, err := h.store.GetUserByEmail(c.Request.Context(), sandboxUserEmail); err == nil && sandboxUser != nil {
			_ = h.ensureSandboxProxyUUID(c.Request.Context(), sandboxUser)
			uuid := strings.TrimSpace(sandboxUser.ProxyUUID)
			if uuid == "" {
				uuid = strings.TrimSpace(sandboxUser.ID)
			}
			if uuid != "" {
				clients = append(clients, xrayconfig.Client{
					ID:    uuid,
					Email: sandboxUserEmail,
					Flow:  xrayconfig.DefaultFlow,
				})
			}
		}
	}

	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list_users_failed"})
		return
	}

	for _, u := range users {
		email := strings.ToLower(strings.TrimSpace(u.Email))
		if email == sandboxUserEmail || email == "demo@svc.plus" {
			continue
		}
		if !u.Active {
			continue
		}
		if !u.EmailVerified {
			continue
		}
		if u.ProxyUUIDExpiresAt != nil && now.After(*u.ProxyUUIDExpiresAt) {
			continue
		}

		id := strings.TrimSpace(u.ProxyUUID)
		if id == "" {
			id = strings.TrimSpace(u.ID)
		}
		if id == "" {
			continue
		}
		clients = append(clients, xrayconfig.Client{
			ID:    id,
			Email: strings.TrimSpace(u.Email),
			Flow:  xrayconfig.DefaultFlow,
		})
	}

	c.JSON(http.StatusOK, agentproto.ClientListResponse{
		Clients:     clients,
		Total:       len(clients),
		GeneratedAt: now,
	})
}

func (h *handler) reportAgentStatus(c *gin.Context) {
	if h.agentRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent_registry_unavailable"})
		return
	}

	token := extractToken(c.GetHeader("Authorization"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_token"})
		return
	}

	credIdentity, ok := h.agentRegistry.Authenticate(token)
	if !ok || credIdentity == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}

	var report agentproto.StatusReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	agentID := strings.TrimSpace(report.AgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(c.GetHeader(agentIDHeader))
	}
	if agentID == "" {
		agentID = credIdentity.ID
	}

	identity := *credIdentity
	if agentID != "" && agentID != identity.ID {
		identity = h.agentRegistry.RegisterAgent(agentID, identity.Groups)
	}

	// Ensure report uses the resolved agent id.
	report.AgentID = identity.ID
	h.agentRegistry.ReportStatus(identity, report)

	c.Status(http.StatusNoContent)
}

var _ = agentserver.Identity{}
