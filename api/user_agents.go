package api

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"account/internal/auth"
	"account/internal/store"
)

type vlessNode struct {
	Name      string   `json:"name"`
	Address   string   `json:"address"`
	Port      int      `json:"port,omitempty"`
	Users     []string `json:"users,omitempty"`
	Transport string   `json:"transport,omitempty"`
	Path      string   `json:"path,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	Security  string   `json:"security,omitempty"`
	Flow      string   `json:"flow,omitempty"`
}

func (h *handler) listAgentNodes(c *gin.Context) {
	// For now, valid nodes are derived from the server's public URL.
	// We currently assume the server itself exposes a VLESS/XHTTP endpoint.
	// In the future, we might retrieve this from the agent registry if agents report their public IPs.

	// Get current user ID to use as VLESS UUID
	userID := auth.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "account_paused",
			"message": "account is paused",
		})
		return
	}

	proxyUUID := user.ProxyUUID
	if proxyUUID == "" {
		proxyUUID = user.ID
	}

	if user.ProxyUUIDExpiresAt != nil && time.Now().UTC().After(*user.ProxyUUIDExpiresAt) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "proxy_uuid_expired",
			"message": "proxy access has expired, please renew",
		})
		return
	}

	users := []string{proxyUUID}
	nodes := make([]vlessNode, 0)

	if h.publicURL != "" {
		u, err := url.Parse(h.publicURL)
		if err == nil {
			hostname := u.Hostname()
			portStr := u.Port()
			port := 443
			if portStr != "" {
				if p, err := strconv.Atoi(portStr); err == nil {
					port = p
				}
			} else if u.Scheme == "http" {
				port = 80
			}

			// Add "Global Acceleration (XHTTP)" node
			nodes = append(nodes, vlessNode{
				Name:      "Global Acceleration (XHTTP)",
				Address:   hostname,
				Port:      port, // Default port, client will adjust based on transport if needed
				Users:     users,
				Transport: "xhttp",
				Path:      "/split",
				Security:  "tls",
				Mode:      "auto",
			})

			// Add "Global Acceleration (TCP)" node
			nodes = append(nodes, vlessNode{
				Name:      "Global Acceleration (TCP)",
				Address:   hostname,
				Port:      1443, // Fixed TCP port from template_tcp.json
				Users:     users,
				Transport: "tcp",
				Security:  "tls",
				Flow:      "xtls-rprx-vision",
			})
		}
	}

	c.JSON(http.StatusOK, nodes)
}
