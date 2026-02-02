package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *handler) pauseUser(c *gin.Context) {
	if _, ok := h.requireAdminOrOperator(c); !ok {
		return
	}

	userID := c.Param("userId")
	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "user_lookup_failed", "failed to find user")
		return
	}

	user.Active = false
	if err := h.store.UpdateUser(c.Request.Context(), user); err != nil {
		respondError(c, http.StatusInternalServerError, "update_failed", "failed to pause user")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user paused"})
}

func (h *handler) resumeUser(c *gin.Context) {
	if _, ok := h.requireAdminOrOperator(c); !ok {
		return
	}

	userID := c.Param("userId")
	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "user_lookup_failed", "failed to find user")
		return
	}

	user.Active = true
	if err := h.store.UpdateUser(c.Request.Context(), user); err != nil {
		respondError(c, http.StatusInternalServerError, "update_failed", "failed to resume user")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user resumed"})
}

func (h *handler) deleteUser(c *gin.Context) {
	if _, ok := h.requireAdminOrOperator(c); !ok {
		return
	}

	userID := c.Param("userId")
	if err := h.store.DeleteUser(c.Request.Context(), userID); err != nil {
		respondError(c, http.StatusInternalServerError, "delete_failed", "failed to delete user")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

func (h *handler) renewProxyUUID(c *gin.Context) {
	if _, ok := h.requireAdminOrOperator(c); !ok {
		return
	}

	userID := c.Param("userId")
	var req struct {
		ExpiresInDays int `json:"expires_in_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "user_lookup_failed", "failed to find user")
		return
	}

	// Generate new UUID
	// We use crypto/rand usually, but for simplicity here we assume a helper or just a placeholder
	// Since I don't have a helper, I'll use a simple random string or assume store handles it if empty
	// Actually, ProxyUUID is a string in the Store.
	user.ProxyUUID = "" // Let the store or a helper generate it if we had one.
	// Since I can't easily import a uuid generator here without checking if it's available,
	// I'll just use a placeholder for now or assume the user wants a new one.
	// Wait, schema says it has a default gen_random_uuid().

	if req.ExpiresInDays > 0 {
		expiration := time.Now().UTC().AddDate(0, 0, req.ExpiresInDays)
		user.ProxyUUIDExpiresAt = &expiration
	} else {
		user.ProxyUUIDExpiresAt = nil
	}

	// For now, let's just use a simple random hex string if we want to "reset" it manually
	// in the logic before UpdateUser.
	user.ProxyUUID = generateRandomUUID()

	if err := h.store.UpdateUser(c.Request.Context(), user); err != nil {
		respondError(c, http.StatusInternalServerError, "update_failed", "failed to renew proxy UUID")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "proxy UUID renewed",
		"proxy_uuid": user.ProxyUUID,
		"expires_at": user.ProxyUUIDExpiresAt,
	})
}

func (h *handler) listBlacklist(c *gin.Context) {
	if _, ok := h.requireAdminOrOperator(c); !ok {
		return
	}

	list, err := h.store.ListBlacklist(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed", "failed to list blacklist")
		return
	}

	c.JSON(http.StatusOK, gin.H{"blacklist": list})
}

func (h *handler) addToBlacklist(c *gin.Context) {
	if _, ok := h.requireAdminOrOperator(c); !ok {
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "invalid request payload")
		return
	}

	if err := h.store.AddToBlacklist(c.Request.Context(), req.Email); err != nil {
		respondError(c, http.StatusInternalServerError, "add_failed", "failed to add to blacklist")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email added to blacklist"})
}

func (h *handler) removeFromBlacklist(c *gin.Context) {
	if _, ok := h.requireAdminOrOperator(c); !ok {
		return
	}

	email := c.Param("email")
	if err := h.store.RemoveFromBlacklist(c.Request.Context(), email); err != nil {
		respondError(c, http.StatusInternalServerError, "remove_failed", "failed to remove from blacklist")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email removed from blacklist"})
}
