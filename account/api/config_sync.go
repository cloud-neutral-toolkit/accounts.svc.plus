package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// syncConfig handles POST /api/config/sync requests. The endpoint currently
// verifies that the caller has a valid authenticated session (using the
// xc_session cookie or Authorization header) and returns a placeholder
// response indicating that the desktop sync feature is not yet implemented.
//
// The full implementation is outlined in docs/account-xstream-desktop-integration.md
// and will be wired in subsequent iterations.
func (h *handler) syncConfig(c *gin.Context) {
	token := extractToken(c.GetHeader("Authorization"))
	if token == "" {
		if cookie, err := c.Cookie(sessionCookieName); err == nil {
			token = strings.TrimSpace(cookie)
		}
	}

	if token == "" {
		respondError(c, http.StatusUnauthorized, "session_token_required", "session token is required")
		return
	}

	if _, ok := h.lookupSession(token); !ok {
		respondError(c, http.StatusUnauthorized, "invalid_session", "session token is invalid or expired")
		return
	}

	respondError(c, http.StatusNotImplemented, "desktop_sync_unavailable", "desktop configuration sync is not yet available")
}
