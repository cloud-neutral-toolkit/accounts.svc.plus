package api

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"account/internal/auth"
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
	users := []string{}
	if userID != "" {
		users = append(users, userID)
	}

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
