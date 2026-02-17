package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"account/internal/store"
	"account/internal/xrayconfig"
)

type syncConfigAckRequest struct {
	Version   int64  `json:"version"`
	DeviceID  string `json:"device_id"`
	AppliedAt string `json:"applied_at"`
}

func (h *handler) syncConfigSnapshot(c *gin.Context) {
	h.respondSyncConfigSnapshot(c)
}

func (h *handler) syncConfig(c *gin.Context) {
	// Backward-compatible endpoint: old clients call POST /api/auth/config/sync.
	h.respondSyncConfigSnapshot(c)
}

func (h *handler) respondSyncConfigSnapshot(c *gin.Context) {
	user, ok := h.requireAuthenticatedUser(c)
	if !ok {
		return
	}

	sinceVersion := int64(0)
	if raw := strings.TrimSpace(c.Query("since_version")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			respondError(c, http.StatusBadRequest, "invalid_since_version", "since_version must be a non-negative integer")
			return
		}
		sinceVersion = v
	}

	version := deriveSyncVersion(user)
	updatedAt := time.Now().UTC()
	if !user.UpdatedAt.IsZero() {
		updatedAt = user.UpdatedAt.UTC()
	}

	renderedJSON, digest, warnings, err := h.renderUserXrayConfig(user)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "config_render_failed", "failed to render xray config")
		return
	}

	changed := sinceVersion < version
	profiles := []gin.H{}
	nodes := []gin.H{}
	if changed {
		profiles = append(profiles, gin.H{
			"id":      strings.TrimSpace(user.ID),
			"remark":  strings.TrimSpace(user.Name),
			"address": extractHostFromPublicURL(h.publicURL),
			"port":    1443,
			"uuid":    strings.TrimSpace(user.ProxyUUID),
			"flow":    xrayconfig.DefaultFlow,
			"source":  "server",
		})
		nodes = append(nodes, gin.H{
			"id":         strings.TrimSpace(user.ID),
			"name":       strings.TrimSpace(user.Name),
			"protocol":   "vless",
			"transport":  "tcp",
			"security":   "tls",
			"address":    extractHostFromPublicURL(h.publicURL),
			"port":       1443,
			"uuid":       strings.TrimSpace(user.ProxyUUID),
			"flow":       xrayconfig.DefaultFlow,
			"source":     "server",
			"updated_at": updatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"schema_version": 1,
		"changed":        changed,
		"version":        version,
		"updated_at":     updatedAt,
		"profiles":       profiles,
		"nodes":          nodes,
		"routes":         []gin.H{},
		"dns": gin.H{
			"mode":    "secure_tunnel",
			"servers": []string{},
		},
		"meta": gin.H{
			"digest":   digest,
			"warnings": warnings,
		},
		"rendered_json": renderedJSON,
		"digest":        digest,
		"warnings":      warnings,
	})
}

func (h *handler) syncConfigAck(c *gin.Context) {
	user, ok := h.requireAuthenticatedUser(c)
	if !ok {
		return
	}

	var req syncConfigAckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	if req.Version <= 0 {
		respondError(c, http.StatusBadRequest, "invalid_version", "version must be positive")
		return
	}
	if strings.TrimSpace(req.DeviceID) == "" {
		respondError(c, http.StatusBadRequest, "device_id_required", "device_id is required")
		return
	}
	if strings.TrimSpace(req.AppliedAt) == "" {
		respondError(c, http.StatusBadRequest, "applied_at_required", "applied_at is required")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"acked":       true,
		"version":     req.Version,
		"device_id":   strings.TrimSpace(req.DeviceID),
		"user_id":     strings.TrimSpace(user.ID),
		"received_at": time.Now().UTC(),
	})
}

func deriveSyncVersion(user *store.User) int64 {
	if user == nil {
		return time.Now().UTC().Unix()
	}
	if !user.UpdatedAt.IsZero() {
		return user.UpdatedAt.UTC().Unix()
	}
	if !user.CreatedAt.IsZero() {
		return user.CreatedAt.UTC().Unix()
	}
	return time.Now().UTC().Unix()
}

func (h *handler) renderUserXrayConfig(user *store.User) (string, string, []string, error) {
	domain := extractHostFromPublicURL(h.publicURL)
	if domain == "" {
		domain = "accounts.svc.plus"
	}

	clientID := strings.TrimSpace(user.ProxyUUID)
	if clientID == "" {
		clientID = strings.TrimSpace(user.ID)
	}
	clients := []xrayconfig.Client{{
		ID:    clientID,
		Email: strings.TrimSpace(user.Email),
		Flow:  xrayconfig.DefaultFlow,
	}}

	gen := xrayconfig.Generator{
		Definition: xrayconfig.TCPDefinition(),
		Domain:     domain,
	}
	buf, err := gen.Render(clients)
	if err != nil {
		return "", "", nil, err
	}
	sum := sha256.Sum256(buf)
	return string(buf), hex.EncodeToString(sum[:]), []string{}, nil
}

func extractHostFromPublicURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Hostname())
}
