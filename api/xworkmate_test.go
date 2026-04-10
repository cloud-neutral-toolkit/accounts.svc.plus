package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"account/internal/auth"
	"account/internal/store"
)

func newXWorkmateTestHarness(t *testing.T) (*gin.Engine, *store.User, string) {
	t.Helper()
	return newXWorkmateTestHarnessForUser(t, &store.User{
		Name:          "XWorkmate Admin",
		Email:         "xworkmate-admin@example.com",
		EmailVerified: true,
		Role:          store.RoleAdmin,
		Level:         store.LevelAdmin,
		Active:        true,
	})
}

func newXWorkmateTestHarnessForUser(t *testing.T, user *store.User) (*gin.Engine, *store.User, string) {
	t.Helper()

	vaultService := newMemoryXWorkmateVaultService()
	return newXWorkmateTestHarnessWithVault(t, user, vaultService)
}

func newXWorkmateTestHarnessWithVault(t *testing.T, user *store.User, vaultService xworkmateVaultService) (*gin.Engine, *store.User, string) {
	t.Helper()

	ctx := context.Background()
	st := store.NewMemoryStore()
	if user == nil {
		user = &store.User{
			Name:          "XWorkmate Admin",
			Email:         "xworkmate-admin@example.com",
			EmailVerified: true,
			Role:          store.RoleAdmin,
			Level:         store.LevelAdmin,
			Active:        true,
		}
	}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	token := "xworkmate-session-token"
	if err := st.CreateSession(ctx, token, user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}

	router := gin.New()
	RegisterRoutes(
		router,
		WithStore(st),
		WithEmailVerification(false),
		WithTokenService(auth.NewTokenService(auth.TokenConfig{
			PublicToken:   "public-token",
			RefreshSecret: "refresh-secret",
			AccessSecret:  "access-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: time.Hour,
			Store:         st,
		})),
		WithXWorkmateVaultService(vaultService),
	)
	return router, user, token
}

func TestBuildXWorkmateTokenConfiguredUsesSecretLocators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		profile  *store.XWorkmateProfile
		openclaw bool
		vault    bool
		apisix   bool
	}{
		{
			name: "missing secret key stays false",
			profile: &store.XWorkmateProfile{
				VaultSecretPath: "kv/openclaw",
			},
		},
		{
			name: "legacy path and key mark openclaw configured",
			profile: &store.XWorkmateProfile{
				VaultSecretPath: "kv/openclaw",
				VaultSecretKey:  "token",
			},
			openclaw: true,
		},
		{
			name: "explicit openclaw locator marks openclaw configured",
			profile: &store.XWorkmateProfile{
				SecretLocators: []store.XWorkmateSecretLocator{
					{
						Provider:   "vault",
						SecretPath: "kv/openclaw",
						SecretKey:  "token",
						Target:     store.XWorkmateSecretLocatorTargetOpenclawGatewayToken,
					},
				},
			},
			openclaw: true,
		},
		{
			name: "other locator stays false",
			profile: &store.XWorkmateProfile{
				SecretLocators: []store.XWorkmateSecretLocator{
					{
						Provider:   "vault",
						SecretPath: "kv/ai",
						SecretKey:  "token",
						Target:     store.XWorkmateSecretLocatorTargetAIGatewayAccessToken,
					},
				},
			},
		},
		{
			name:    "blank profile stays false",
			profile: &store.XWorkmateProfile{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildXWorkmateTokenConfigured(tt.profile)
			if got := result["openclaw"].(bool); got != tt.openclaw {
				t.Fatalf("expected openclaw=%v, got %v", tt.openclaw, got)
			}
			if got := result["vault"].(bool); got != tt.vault {
				t.Fatalf("expected vault=%v, got %v", tt.vault, got)
			}
			if got := result["apisix"].(bool); got != tt.apisix {
				t.Fatalf("expected apisix=%v, got %v", tt.apisix, got)
			}
		})
	}
}

func TestUpdateAndGetXWorkmateProfileRoundTripsSecretLocators(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, _, token := newXWorkmateTestHarness(t)
	body, err := json.Marshal(map[string]any{
		"profile": map[string]any{
			"openclawUrl":    "wss://gateway.example.com",
			"openclawOrigin": "https://gateway.example.com",
			"vaultUrl":       "https://vault.example.com",
			"vaultNamespace": "team-a",
			"secretLocators": []map[string]any{
				{
					"id":         "locator-openclaw",
					"provider":   "vault",
					"secretPath": "kv/openclaw",
					"secretKey":  "token",
					"target":     store.XWorkmateSecretLocatorTargetOpenclawGatewayToken,
					"required":   true,
				},
				{
					"id":         "locator-ai-gateway",
					"provider":   "vault",
					"secretPath": "kv/ai",
					"secretKey":  "access-token",
					"target":     store.XWorkmateSecretLocatorTargetAIGatewayAccessToken,
				},
			},
			"apisixUrl": "https://apigw.example.com",
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/profile", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected update success, got %d: %s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/profile", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected profile fetch success, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var resp struct {
		Profile struct {
			OpenclawURL    string `json:"openclawUrl"`
			OpenclawOrigin string `json:"openclawOrigin"`
			VaultURL       string `json:"vaultUrl"`
			VaultNamespace string `json:"vaultNamespace"`
			SecretLocators []struct {
				ID         string `json:"id"`
				Provider   string `json:"provider"`
				SecretPath string `json:"secretPath"`
				SecretKey  string `json:"secretKey"`
				Target     string `json:"target"`
				Required   bool   `json:"required"`
			} `json:"secretLocators"`
			VaultSecretPath string `json:"vaultSecretPath"`
			VaultSecretKey  string `json:"vaultSecretKey"`
			ApisixURL       string `json:"apisixUrl"`
		} `json:"profile"`
		TokenConfigured struct {
			Openclaw bool `json:"openclaw"`
			Vault    bool `json:"vault"`
			Apisix   bool `json:"apisix"`
		} `json:"tokenConfigured"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}

	if resp.Profile.VaultSecretPath != "kv/openclaw" || resp.Profile.VaultSecretKey != "token" {
		t.Fatalf("expected compatibility fields to mirror openclaw locator, got %#v", resp.Profile)
	}
	if len(resp.Profile.SecretLocators) != 2 {
		t.Fatalf("expected 2 locators, got %#v", resp.Profile.SecretLocators)
	}
	if resp.Profile.SecretLocators[0].ID != "locator-openclaw" || !resp.Profile.SecretLocators[0].Required {
		t.Fatalf("expected openclaw locator to round-trip, got %#v", resp.Profile.SecretLocators[0])
	}
	if resp.Profile.SecretLocators[0].Target != store.XWorkmateSecretLocatorTargetOpenclawGatewayToken {
		t.Fatalf("expected openclaw target, got %#v", resp.Profile.SecretLocators[0])
	}
	if resp.Profile.SecretLocators[1].Target != store.XWorkmateSecretLocatorTargetAIGatewayAccessToken {
		t.Fatalf("expected ai gateway target, got %#v", resp.Profile.SecretLocators[1])
	}
	if resp.TokenConfigured.Openclaw {
		t.Fatalf("expected openclaw tokenConfigured=false until a vault-backed secret exists")
	}
	if resp.TokenConfigured.Vault {
		t.Fatalf("expected vault tokenConfigured=false without a vault-backed token locator")
	}
	if resp.TokenConfigured.Apisix {
		t.Fatalf("expected apisix tokenConfigured=false without a token locator")
	}
}

func TestUpdateXWorkmateProfileSynthesizesSecretLocatorsFromLegacyFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, _, token := newXWorkmateTestHarness(t)
	body, err := json.Marshal(map[string]any{
		"profile": map[string]any{
			"openclawUrl":     "wss://gateway.example.com",
			"openclawOrigin":  "https://gateway.example.com",
			"vaultUrl":        "https://vault.example.com",
			"vaultNamespace":  "team-a",
			"vaultSecretPath": "kv/openclaw",
			"vaultSecretKey":  "token",
			"apisixUrl":       "https://apigw.example.com",
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/profile", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected update success, got %d: %s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/profile", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected profile fetch success, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var resp struct {
		Profile struct {
			SecretLocators []struct {
				Provider   string `json:"provider"`
				SecretPath string `json:"secretPath"`
				SecretKey  string `json:"secretKey"`
				Target     string `json:"target"`
			} `json:"secretLocators"`
			VaultSecretPath string `json:"vaultSecretPath"`
			VaultSecretKey  string `json:"vaultSecretKey"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}

	if len(resp.Profile.SecretLocators) != 1 {
		t.Fatalf("expected synthesized single locator, got %#v", resp.Profile.SecretLocators)
	}
	if resp.Profile.SecretLocators[0].Provider != "vault" || resp.Profile.SecretLocators[0].Target != store.XWorkmateSecretLocatorTargetOpenclawGatewayToken {
		t.Fatalf("expected synthesized openclaw vault locator, got %#v", resp.Profile.SecretLocators[0])
	}
	if resp.Profile.VaultSecretPath != "kv/openclaw" || resp.Profile.VaultSecretKey != "token" {
		t.Fatalf("expected legacy fields to remain readable, got %#v", resp.Profile)
	}
}

func TestGetXWorkmateProfileFallsBackWhenVaultStatusReadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, _, token := newXWorkmateTestHarnessWithVault(
		t,
		nil,
		&flakyXWorkmateVaultService{failAfter: 4},
	)
	body, err := json.Marshal(map[string]any{
		"profile": map[string]any{
			"openclawUrl":    "wss://gateway.example.com",
			"openclawOrigin": "https://gateway.example.com",
			"vaultUrl":       "https://vault.example.com",
			"vaultNamespace": "team-a",
			"secretLocators": []map[string]any{
				{
					"id":         "locator-openclaw",
					"provider":   "vault",
					"secretPath": "kv/openclaw",
					"secretKey":  "token",
					"target":     store.XWorkmateSecretLocatorTargetOpenclawGatewayToken,
					"required":   true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/profile", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected update success, got %d: %s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/profile", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected profile fetch success, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var resp struct {
		Profile struct {
			OpenclawURL string `json:"openclawUrl"`
		} `json:"profile"`
		TokenConfigured struct {
			Openclaw bool `json:"openclaw"`
			Vault    bool `json:"vault"`
			Apisix   bool `json:"apisix"`
		} `json:"tokenConfigured"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}

	if resp.Profile.OpenclawURL != "wss://gateway.example.com" {
		t.Fatalf("expected profile payload to survive vault read failure, got %#v", resp.Profile)
	}
	if !resp.TokenConfigured.Openclaw {
		t.Fatalf("expected locator-derived openclaw tokenConfigured fallback, got %#v", resp.TokenConfigured)
	}
	if resp.TokenConfigured.Vault {
		t.Fatalf("expected vault tokenConfigured fallback to stay false, got %#v", resp.TokenConfigured)
	}
	if resp.TokenConfigured.Apisix {
		t.Fatalf("expected apisix tokenConfigured fallback to stay false, got %#v", resp.TokenConfigured)
	}
}

type flakyXWorkmateVaultService struct {
	hasSecretCalls int
	failAfter      int
}

func (f *flakyXWorkmateVaultService) WriteSecret(ctx context.Context, locator store.XWorkmateSecretLocator, value string) error {
	_ = ctx
	_ = locator
	_ = value
	return nil
}

func (f *flakyXWorkmateVaultService) DeleteSecret(ctx context.Context, locator store.XWorkmateSecretLocator) error {
	_ = ctx
	_ = locator
	return nil
}

func (f *flakyXWorkmateVaultService) HasSecret(ctx context.Context, locator store.XWorkmateSecretLocator) (bool, error) {
	_ = ctx
	_ = locator
	f.hasSecretCalls++
	if f.failAfter > 0 && f.hasSecretCalls > f.failAfter {
		return false, errors.New("vault unavailable")
	}
	return false, nil
}

func TestUpdateXWorkmateProfileRejectsNestedRawTokenFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, _, token := newXWorkmateTestHarness(t)

	body, err := json.Marshal(map[string]any{
		"profile": map[string]any{
			"openclawUrl": "wss://gateway.example.com",
			"security": map[string]any{
				"gatewayToken": "secret-value",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected raw token rejection, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "token_persistence_forbidden" {
		t.Fatalf("expected token_persistence_forbidden, got %q", resp.Error)
	}
}

func TestXWorkmateSecretsWriteReadDeleteAndKeepLocatorMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, _, token := newXWorkmateTestHarness(t)
	profileBody, err := json.Marshal(map[string]any{
		"profile": map[string]any{
			"openclawUrl":    "wss://gateway.example.com",
			"openclawOrigin": "https://gateway.example.com",
			"vaultUrl":       "https://vault.example.com",
			"vaultNamespace": "team-a",
			"apisixUrl":      "https://apigw.example.com",
		},
	})
	if err != nil {
		t.Fatalf("marshal profile payload: %v", err)
	}

	putProfileReq := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/profile", bytes.NewReader(profileBody))
	putProfileReq.Header.Set("Content-Type", "application/json")
	putProfileReq.Header.Set("Authorization", "Bearer "+token)
	putProfileReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	putProfileRec := httptest.NewRecorder()
	router.ServeHTTP(putProfileRec, putProfileReq)
	if putProfileRec.Code != http.StatusOK {
		t.Fatalf("expected profile update success, got %d: %s", putProfileRec.Code, putProfileRec.Body.String())
	}

	for _, target := range []string{
		store.XWorkmateSecretLocatorTargetOpenclawGatewayToken,
		store.XWorkmateSecretLocatorTargetVaultRootToken,
		store.XWorkmateSecretLocatorTargetAIGatewayAccessToken,
	} {
		secretBody, err := json.Marshal(map[string]any{"value": "super-secret-" + target})
		if err != nil {
			t.Fatalf("marshal secret payload for %s: %v", target, err)
		}

		req := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/secrets/"+target, bytes.NewReader(secretBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected secret write success for %s, got %d: %s", target, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "super-secret-"+target) {
			t.Fatalf("expected raw secret to stay out of response for %s, got %s", target, rec.Body.String())
		}
	}

	getSecretsReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/secrets", nil)
	getSecretsReq.Header.Set("Authorization", "Bearer "+token)
	getSecretsReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	getSecretsRec := httptest.NewRecorder()
	router.ServeHTTP(getSecretsRec, getSecretsReq)
	if getSecretsRec.Code != http.StatusOK {
		t.Fatalf("expected secret status fetch success, got %d: %s", getSecretsRec.Code, getSecretsRec.Body.String())
	}
	if strings.Contains(getSecretsRec.Body.String(), "super-secret-") {
		t.Fatalf("expected secret status response to hide raw values, got %s", getSecretsRec.Body.String())
	}

	getProfileReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/profile", nil)
	getProfileReq.Header.Set("Authorization", "Bearer "+token)
	getProfileReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	getProfileRec := httptest.NewRecorder()
	router.ServeHTTP(getProfileRec, getProfileReq)
	if getProfileRec.Code != http.StatusOK {
		t.Fatalf("expected profile fetch success, got %d: %s", getProfileRec.Code, getProfileRec.Body.String())
	}

	var profileResp struct {
		Profile struct {
			VaultSecretPath string `json:"vaultSecretPath"`
			VaultSecretKey  string `json:"vaultSecretKey"`
			SecretLocators  []struct {
				Target string `json:"target"`
			} `json:"secretLocators"`
		} `json:"profile"`
		TokenConfigured struct {
			Openclaw bool `json:"openclaw"`
			Vault    bool `json:"vault"`
			Apisix   bool `json:"apisix"`
		} `json:"tokenConfigured"`
	}
	if err := json.Unmarshal(getProfileRec.Body.Bytes(), &profileResp); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if !profileResp.TokenConfigured.Openclaw || !profileResp.TokenConfigured.Vault || !profileResp.TokenConfigured.Apisix {
		t.Fatalf("expected all synced tokenConfigured fields true, got %#v", profileResp.TokenConfigured)
	}
	if len(profileResp.Profile.SecretLocators) != 3 {
		t.Fatalf("expected 3 secret locators after vault writes, got %#v", profileResp.Profile.SecretLocators)
	}
	if profileResp.Profile.VaultSecretPath == "" || profileResp.Profile.VaultSecretKey == "" {
		t.Fatalf("expected openclaw legacy compatibility fields to remain readable, got %#v", profileResp.Profile)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/auth/xworkmate/secrets/"+store.XWorkmateSecretLocatorTargetOpenclawGatewayToken, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected secret delete success, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	getProfileAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/profile", nil)
	getProfileAfterDeleteReq.Header.Set("Authorization", "Bearer "+token)
	getProfileAfterDeleteReq.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	getProfileAfterDeleteRec := httptest.NewRecorder()
	router.ServeHTTP(getProfileAfterDeleteRec, getProfileAfterDeleteReq)
	if getProfileAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf("expected profile fetch after delete success, got %d: %s", getProfileAfterDeleteRec.Code, getProfileAfterDeleteRec.Body.String())
	}

	var afterDeleteResp struct {
		Profile struct {
			SecretLocators []struct {
				Target string `json:"target"`
			} `json:"secretLocators"`
		} `json:"profile"`
		TokenConfigured struct {
			Openclaw bool `json:"openclaw"`
			Vault    bool `json:"vault"`
			Apisix   bool `json:"apisix"`
		} `json:"tokenConfigured"`
	}
	if err := json.Unmarshal(getProfileAfterDeleteRec.Body.Bytes(), &afterDeleteResp); err != nil {
		t.Fatalf("decode post-delete profile response: %v", err)
	}
	if afterDeleteResp.TokenConfigured.Openclaw {
		t.Fatalf("expected deleted openclaw secret to report missing, got %#v", afterDeleteResp.TokenConfigured)
	}
	if !afterDeleteResp.TokenConfigured.Vault || !afterDeleteResp.TokenConfigured.Apisix {
		t.Fatalf("expected unrelated secret statuses to remain true, got %#v", afterDeleteResp.TokenConfigured)
	}
	if len(afterDeleteResp.Profile.SecretLocators) != 3 {
		t.Fatalf("expected locator metadata to remain after delete, got %#v", afterDeleteResp.Profile.SecretLocators)
	}
}

func TestXWorkmateSharedSecretsRequireAdminMembershipForWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, _, token := newXWorkmateTestHarnessForUser(t, &store.User{
		Name:          "Shared Demo User",
		Email:         "shared-user@example.com",
		EmailVerified: true,
		Role:          store.RoleUser,
		Level:         store.LevelUser,
		Active:        true,
	})

	body, err := json.Marshal(map[string]any{"value": "super-secret"})
	if err != nil {
		t.Fatalf("marshal secret payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/secrets/"+store.XWorkmateSecretLocatorTargetOpenclawGatewayToken, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Forwarded-Host", store.SharedXWorkmateDomain)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected shared tenant secret write to be forbidden for non-admin, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestXWorkmatePrivateSecretsAreScopedPerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	st := store.NewMemoryStore()
	vaultService := newMemoryXWorkmateVaultService()

	tenant := &store.Tenant{
		ID:      "tenant-private-1",
		Name:    "Tenant Private 1",
		Edition: store.TenantPrivateEdition,
	}
	if err := st.EnsureTenant(ctx, tenant); err != nil {
		t.Fatalf("ensure tenant: %v", err)
	}
	if err := st.EnsureTenantDomain(ctx, &store.TenantDomain{
		TenantID:  tenant.ID,
		Domain:    "tenant-private-1.svc.plus",
		Kind:      store.TenantDomainKindGenerated,
		IsPrimary: true,
		Status:    store.TenantDomainStatusVerified,
	}); err != nil {
		t.Fatalf("ensure tenant domain: %v", err)
	}

	userA := &store.User{
		Name:          "Tenant Admin A",
		Email:         "tenant-admin-a@example.com",
		EmailVerified: true,
		Role:          store.RoleAdmin,
		Level:         store.LevelAdmin,
		Active:        true,
	}
	userB := &store.User{
		Name:          "Tenant Admin B",
		Email:         "tenant-admin-b@example.com",
		EmailVerified: true,
		Role:          store.RoleAdmin,
		Level:         store.LevelAdmin,
		Active:        true,
	}
	for _, user := range []*store.User{userA, userB} {
		if err := st.CreateUser(ctx, user); err != nil {
			t.Fatalf("create user %s: %v", user.Email, err)
		}
		if err := st.UpsertTenantMembership(ctx, &store.TenantMembership{
			TenantID: tenant.ID,
			UserID:   user.ID,
			Role:     store.TenantMembershipRoleAdmin,
		}); err != nil {
			t.Fatalf("upsert tenant membership for %s: %v", user.Email, err)
		}
	}

	tokenA := "tenant-token-a"
	tokenB := "tenant-token-b"
	if err := st.CreateSession(ctx, tokenA, userA.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session A: %v", err)
	}
	if err := st.CreateSession(ctx, tokenB, userB.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session B: %v", err)
	}

	router := gin.New()
	RegisterRoutes(
		router,
		WithStore(st),
		WithEmailVerification(false),
		WithTokenService(auth.NewTokenService(auth.TokenConfig{
			PublicToken:   "public-token",
			RefreshSecret: "refresh-secret",
			AccessSecret:  "access-secret",
			AccessExpiry:  time.Hour,
			RefreshExpiry: time.Hour,
			Store:         st,
		})),
		WithXWorkmateVaultService(vaultService),
	)

	body, err := json.Marshal(map[string]any{"value": "tenant-secret-a"})
	if err != nil {
		t.Fatalf("marshal secret payload: %v", err)
	}
	writeReq := httptest.NewRequest(http.MethodPut, "/api/auth/xworkmate/secrets/"+store.XWorkmateSecretLocatorTargetOpenclawGatewayToken, bytes.NewReader(body))
	writeReq.Header.Set("Content-Type", "application/json")
	writeReq.Header.Set("Authorization", "Bearer "+tokenA)
	writeReq.Header.Set("X-Forwarded-Host", "tenant-private-1.svc.plus")
	writeRec := httptest.NewRecorder()
	router.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusOK {
		t.Fatalf("expected user A secret write success, got %d: %s", writeRec.Code, writeRec.Body.String())
	}

	getAReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/profile", nil)
	getAReq.Header.Set("Authorization", "Bearer "+tokenA)
	getAReq.Header.Set("X-Forwarded-Host", "tenant-private-1.svc.plus")
	getARec := httptest.NewRecorder()
	router.ServeHTTP(getARec, getAReq)
	if getARec.Code != http.StatusOK {
		t.Fatalf("expected user A profile fetch success, got %d: %s", getARec.Code, getARec.Body.String())
	}

	getBReq := httptest.NewRequest(http.MethodGet, "/api/auth/xworkmate/profile", nil)
	getBReq.Header.Set("Authorization", "Bearer "+tokenB)
	getBReq.Header.Set("X-Forwarded-Host", "tenant-private-1.svc.plus")
	getBRec := httptest.NewRecorder()
	router.ServeHTTP(getBRec, getBReq)
	if getBRec.Code != http.StatusOK {
		t.Fatalf("expected user B profile fetch success, got %d: %s", getBRec.Code, getBRec.Body.String())
	}

	var userAResp struct {
		TokenConfigured struct {
			Openclaw bool `json:"openclaw"`
		} `json:"tokenConfigured"`
	}
	if err := json.Unmarshal(getARec.Body.Bytes(), &userAResp); err != nil {
		t.Fatalf("decode user A profile response: %v", err)
	}
	if !userAResp.TokenConfigured.Openclaw {
		t.Fatalf("expected user A secret to be configured, got %#v", userAResp.TokenConfigured)
	}

	var userBResp struct {
		TokenConfigured struct {
			Openclaw bool `json:"openclaw"`
		} `json:"tokenConfigured"`
	}
	if err := json.Unmarshal(getBRec.Body.Bytes(), &userBResp); err != nil {
		t.Fatalf("decode user B profile response: %v", err)
	}
	if userBResp.TokenConfigured.Openclaw {
		t.Fatalf("expected user B to remain isolated from user A secret, got %#v", userBResp.TokenConfigured)
	}
}
