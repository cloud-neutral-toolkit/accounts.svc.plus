package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type internalNetworkIdentity struct {
	UUID        string `json:"uuid"`
	Email       string `json:"email"`
	AccountUUID string `json:"accountUuid"`
}

func (h *handler) internalNetworkIdentities(c *gin.Context) {
	if h.store == nil {
		respondError(c, http.StatusServiceUnavailable, "store_unavailable", "identity store is not available")
		return
	}

	users, err := h.store.ListUsers(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_users_failed", "failed to load network identities")
		return
	}

	identities := make([]internalNetworkIdentity, 0, len(users))
	for _, user := range users {
		if !user.Active {
			continue
		}
		uuid := strings.TrimSpace(user.ProxyUUID)
		if uuid == "" {
			uuid = strings.TrimSpace(user.ID)
		}
		if uuid == "" {
			continue
		}

		identities = append(identities, internalNetworkIdentity{
			UUID:        uuid,
			Email:       strings.ToLower(strings.TrimSpace(user.Email)),
			AccountUUID: strings.TrimSpace(user.ID),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"generatedAt": time.Now().UTC(),
		"identities":  identities,
	})
}
