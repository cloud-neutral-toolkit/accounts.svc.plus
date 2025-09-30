package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterEndpointAvailableUnderMultiplePrefixes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	paths := []string{"/v1/register", "/api/auth/register"}

	for _, path := range paths {
		router := gin.New()
		RegisterRoutes(router)

		payload := map[string]string{
			"name":     "Test User",
			"email":    "user" + path + "@example.com",
			"password": "supersecure",
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("%s: expected status %d, got %d, body: %s", path, http.StatusCreated, rr.Code, rr.Body.String())
		}

		var response struct {
			User map[string]any `json:"user"`
		}

		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("%s: failed to decode response: %v", path, err)
		}

		if response.User == nil {
			t.Fatalf("%s: expected user object in response", path)
		}

		if email, ok := response.User["email"].(string); !ok || email != payload["email"] {
			t.Fatalf("%s: expected email %q, got %#v", path, payload["email"], response.User["email"])
		}

		if _, exists := response.User["password"]; exists {
			t.Fatalf("%s: response should not include password field", path)
		}
	}
}
