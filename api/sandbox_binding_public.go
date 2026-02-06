package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"account/internal/model"
)

// getSandboxBindingPublic returns the currently bound sandbox agent (if any).
// It is intentionally readable by any authenticated user so demo/sandbox users
// do not depend on localStorage browser state.
func (h *handler) getSandboxBindingPublic(c *gin.Context) {
	if _, ok := h.requireAuthenticatedUser(c); !ok {
		return
	}

	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database_not_configured"})
		return
	}

	var binding model.SandboxBinding
	if err := h.db.WithContext(c.Request.Context()).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"address": "", "updatedAt": int64(0)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_query_binding", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"address":   binding.AgentID,
		"updatedAt": binding.UpdatedAt.UnixMilli(),
	})
}
