package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router)

	payload := map[string]string{
		"name":     "Test User",
		"email":    "user@example.com",
		"password": "supersecure",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var response struct {
		Message string         `json:"message"`
		User    map[string]any `json:"user"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.User == nil {
		t.Fatalf("expected user object in response")
	}

	if email, ok := response.User["email"].(string); !ok || email != payload["email"] {
		t.Fatalf("expected email %q, got %#v", payload["email"], response.User["email"])
	}

	if response.Message == "" {
		t.Fatalf("expected success message in response")
	}

	if _, exists := response.User["password"]; exists {
		t.Fatalf("response should not include password field")
	}
}

func TestRegisterRejectsDuplicateIdentifiers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router)

	basePayload := map[string]string{
		"name":     "Existing User",
		"email":    "existing@example.com",
		"password": "supersecure",
	}

	body, err := json.Marshal(basePayload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected initial registration to succeed, got %d", rr.Code)
	}

	// Duplicate email
	payload := map[string]string{
		"name":     "Another User",
		"email":    basePayload["email"],
		"password": "supersecure",
	}
	body, err = json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected conflict for duplicate email, got %d", rr.Code)
	}

	var conflictResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &conflictResp); err != nil {
		t.Fatalf("failed to decode duplicate email response: %v", err)
	}
	if conflictResp.Error != "email_already_exists" {
		t.Fatalf("expected email_already_exists error, got %q", conflictResp.Error)
	}

	// Duplicate name
	payload = map[string]string{
		"name":     basePayload["name"],
		"email":    "unique@example.com",
		"password": "supersecure",
	}
	body, err = json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected conflict for duplicate name, got %d", rr.Code)
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &conflictResp); err != nil {
		t.Fatalf("failed to decode duplicate name response: %v", err)
	}
	if conflictResp.Error != "name_already_exists" {
		t.Fatalf("expected name_already_exists error, got %q", conflictResp.Error)
	}
}
