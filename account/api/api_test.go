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

	if id, ok := response.User["id"].(string); !ok || id == "" {
		t.Fatalf("expected user id in response, got %#v", response.User["id"])
	} else {
		if uuid, ok := response.User["uuid"].(string); !ok || uuid != id {
			t.Fatalf("expected uuid to match id, got id=%q uuid=%#v", id, response.User["uuid"])
		}
	}

	if response.Message == "" {
		t.Fatalf("expected success message in response")
	}

	if _, exists := response.User["password"]; exists {
		t.Fatalf("response should not include password field")
	}
}

func TestLoginEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router)

	registerPayload := map[string]string{
		"name":     "Login User",
		"email":    "login@example.com",
		"password": "supersecure",
	}

	registerBody, err := json.Marshal(registerPayload)
	if err != nil {
		t.Fatalf("failed to marshal registration payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected registration to succeed, got %d", rr.Code)
	}

	loginPayload := map[string]string{
		"username": "Login User",
		"password": registerPayload["password"],
	}

	loginBody, err := json.Marshal(loginPayload)
	if err != nil {
		t.Fatalf("failed to marshal login payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected login success, got %d: %s", rr.Code, rr.Body.String())
	}

	var loginResponse struct {
		Message string                 `json:"message"`
		Token   string                 `json:"token"`
		User    map[string]interface{} `json:"user"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	if id, ok := loginResponse.User["id"].(string); !ok || id == "" {
		t.Fatalf("expected user id in login response, got %#v", loginResponse.User["id"])
	} else {
		if uuid, ok := loginResponse.User["uuid"].(string); !ok || uuid != id {
			t.Fatalf("expected login uuid to match id, got id=%q uuid=%#v", id, loginResponse.User["uuid"])
		}
	}

	if loginResponse.Message == "" {
		t.Fatalf("expected login success message")
	}
	if loginResponse.Token == "" {
		t.Fatalf("expected session token in login response")
	}
	if username, ok := loginResponse.User["username"].(string); !ok || username != registerPayload["name"] {
		t.Fatalf("expected username %q in response, got %#v", registerPayload["name"], loginResponse.User["username"])
	}

	// Wrong password
	loginPayload["password"] = "wrongpass"
	loginBody, err = json.Marshal(loginPayload)
	if err != nil {
		t.Fatalf("failed to marshal invalid login payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for wrong password, got %d", rr.Code)
	}

	var errorResponse struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("failed to decode wrong password response: %v", err)
	}
	if errorResponse.Error != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials error, got %q", errorResponse.Error)
	}

	// Unknown user
	loginPayload["username"] = "missing-user"
	loginPayload["password"] = registerPayload["password"]
	loginBody, err = json.Marshal(loginPayload)
	if err != nil {
		t.Fatalf("failed to marshal missing user payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected not found for missing user, got %d", rr.Code)
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("failed to decode missing user response: %v", err)
	}
	if errorResponse.Error != "user_not_found" {
		t.Fatalf("expected user_not_found error, got %q", errorResponse.Error)
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
