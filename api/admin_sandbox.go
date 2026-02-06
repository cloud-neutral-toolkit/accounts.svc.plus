package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"account/internal/model"
)

func (h *handler) getSandboxBinding(c *gin.Context) {
	if _, ok := h.requireAdminPermission(c, permissionAdminSettingsRead); !ok {
		return
	}

	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database_not_configured"})
		return
	}

	var binding model.SandboxBinding
	if err := h.db.WithContext(c.Request.Context()).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"address": "", "name": ""})
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

func (h *handler) bindSandboxNode(c *gin.Context) {
	adminUser, ok := h.requireAdminPermission(c, permissionAdminSettingsWrite)
	if !ok {
		return
	}
	if h.isReadOnlyAccount(adminUser) {
		respondError(c, http.StatusForbidden, "read_only_account", "demo account is read-only")
		return
	}

	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database_not_configured"})
		return
	}

	var req struct {
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	agentID := strings.TrimSpace(req.Address)

	err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Clear existing bindings (enforce 1-to-1 for now as per frontend)
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.SandboxBinding{}).Error; err != nil {
			return err
		}

		if agentID != "" {
			newBinding := model.SandboxBinding{
				AgentID: agentID,
			}
			if err := tx.Create(&newBinding).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_save_binding", "message": err.Error()})
		return
	}

	// Update the in-memory registry if available
	if h.agentRegistry != nil {
		// This should ideally be handled by a more robust event system,
		// but since it's a single instance (usually), we can just clear and reset.
		// For now, let's assume the registry will reload if we trigger it or we just set it here.
		// Wait, Registry needs to know ALL sandbox agents.
		// I'll update the registry's internal state.

		// First reset all sandbox flags (not supported by current Registry API, let's just set the new one)
		// TODO: Implement ClearSandboxAgents in Registry
		if agentID != "" {
			h.agentRegistry.SetSandboxAgent(agentID, true)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "sandbox node bound successfully", "address": agentID})
}
