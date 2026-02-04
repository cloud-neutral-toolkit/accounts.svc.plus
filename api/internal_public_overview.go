package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type internalPublicOverviewResponse struct {
	RegisteredUsers int       `json:"registeredUsers"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (h *handler) internalPublicOverview(c *gin.Context) {
	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_users_failed", "failed to fetch users")
		return
	}

	c.JSON(http.StatusOK, internalPublicOverviewResponse{
		RegisteredUsers: len(users),
		UpdatedAt:       time.Now().UTC(),
	})
}
