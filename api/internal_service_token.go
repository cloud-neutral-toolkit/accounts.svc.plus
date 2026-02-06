package api

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const internalServiceTokenHeader = "X-Service-Token"

func isInternalServiceRequest(c *gin.Context) bool {
	if c == nil {
		return false
	}
	token := strings.TrimSpace(c.GetHeader(internalServiceTokenHeader))
	if token == "" {
		return false
	}
	expected := strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN"))
	if expected == "" {
		return false
	}
	return token == expected
}

