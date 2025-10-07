package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"xcontrol/account/internal/model"
	"xcontrol/account/internal/service"
	"xcontrol/account/internal/store"
)

func setupAdminSettingsTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminSetting{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	service.SetDB(db)
	t.Cleanup(func() {
		service.SetDB(nil)
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	router := gin.New()
	RegisterRoutes(router, WithStore(store.NewMemoryStore()))
	return router
}

func TestAdminSettingsReadWrite(t *testing.T) {
	router := setupAdminSettingsTestRouter(t)

	payload := map[string]any{
		"version": 0,
		"matrix": map[string]map[string]bool{
			"registration": {
				"admin":    true,
				"operator": false,
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/admin/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (%s)", resp.Code, resp.Body.String())
	}

	var postResp struct {
		Version uint                       `json:"version"`
		Matrix  map[string]map[string]bool `json:"matrix"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &postResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if postResp.Version != 1 {
		t.Fatalf("expected version 1, got %d", postResp.Version)
	}
	if !postResp.Matrix["registration"]["admin"] {
		t.Fatalf("expected admin flag to be true")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/admin/settings", nil)
	req.Header.Set("X-Role", "operator")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	var getResp struct {
		Version uint                       `json:"version"`
		Matrix  map[string]map[string]bool `json:"matrix"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal get response: %v", err)
	}
	if getResp.Version != postResp.Version {
		t.Fatalf("expected version %d, got %d", postResp.Version, getResp.Version)
	}
	if getResp.Matrix["registration"]["operator"] {
		t.Fatalf("expected operator flag to remain false")
	}
}

func TestAdminSettingsUnauthorized(t *testing.T) {
	router := setupAdminSettingsTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/admin/settings", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.Code)
	}

	payload := map[string]any{
		"version": 0,
		"matrix":  map[string]map[string]bool{},
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/admin/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "user")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.Code)
	}
}

func TestAdminSettingsVersionConflict(t *testing.T) {
	router := setupAdminSettingsTestRouter(t)

	payload := map[string]any{
		"version": 0,
		"matrix": map[string]map[string]bool{
			"registration": {"admin": true},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/admin/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	// Replay the payload with the stale version.
	req = httptest.NewRequest(http.MethodPost, "/api/auth/admin/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Role", "admin")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.Code)
	}
	var conflict struct {
		Version uint `json:"version"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &conflict); err != nil {
		t.Fatalf("unmarshal conflict response: %v", err)
	}
	if conflict.Version != 1 {
		t.Fatalf("expected current version 1, got %d", conflict.Version)
	}
}
