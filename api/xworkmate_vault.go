package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"account/internal/store"
)

const defaultXWorkmateVaultTimeout = 5 * time.Second

type xworkmateVaultService interface {
	WriteSecret(ctx context.Context, locator store.XWorkmateSecretLocator, value string) error
	DeleteSecret(ctx context.Context, locator store.XWorkmateSecretLocator) error
	HasSecret(ctx context.Context, locator store.XWorkmateSecretLocator) (bool, error)
	ReadSecret(ctx context.Context, locator store.XWorkmateSecretLocator) (string, error)
}

type XWorkmateVaultConfig struct {
	Address    string
	Token      string
	Namespace  string
	Mount      string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type httpXWorkmateVaultService struct {
	baseURL   string
	token     string
	namespace string
	mount     string
	client    *http.Client
}

type memoryXWorkmateVaultService struct {
	mu    sync.RWMutex
	store map[string]map[string]string
}

type xworkmateManagedSecretTarget struct {
	Target            string
	Required          bool
	TokenConfiguredID string
}

var xworkmateManagedSecretTargets = []xworkmateManagedSecretTarget{
	{
		Target:            store.XWorkmateSecretLocatorTargetOpenclawGatewayToken,
		Required:          true,
		TokenConfiguredID: "openclaw",
	},
	{
		Target:            store.XWorkmateSecretLocatorTargetVaultRootToken,
		Required:          false,
		TokenConfiguredID: "vault",
	},
	{
		Target:            store.XWorkmateSecretLocatorTargetAIGatewayAccessToken,
		Required:          false,
		TokenConfiguredID: "apisix",
	},
	{
		Target:            store.XWorkmateSecretLocatorTargetOllamaCloudAPIKey,
		Required:          false,
		TokenConfiguredID: "",
	},
}

func newMemoryXWorkmateVaultService() *memoryXWorkmateVaultService {
	return &memoryXWorkmateVaultService{
		store: make(map[string]map[string]string),
	}
}

func NewXWorkmateVaultService(cfg XWorkmateVaultConfig) (xworkmateVaultService, error) {
	address := strings.TrimSpace(cfg.Address)
	token := strings.TrimSpace(cfg.Token)
	if address == "" || token == "" {
		return nil, nil
	}

	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid xworkmate vault address: %q", address)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultXWorkmateVaultTimeout
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	mount := strings.Trim(strings.TrimSpace(cfg.Mount), "/")
	if mount == "" {
		mount = "secret"
	}

	return &httpXWorkmateVaultService{
		baseURL:   strings.TrimRight(parsed.String(), "/"),
		token:     token,
		namespace: strings.TrimSpace(cfg.Namespace),
		mount:     mount,
		client:    client,
	}, nil
}

func (s *memoryXWorkmateVaultService) WriteSecret(ctx context.Context, locator store.XWorkmateSecretLocator, value string) error {
	_ = ctx
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return fmt.Errorf("vault locator is incomplete")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("secret value is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store[locator.SecretPath] == nil {
		s.store[locator.SecretPath] = make(map[string]string)
	}
	s.store[locator.SecretPath][locator.SecretKey] = value
	return nil
}

func (s *memoryXWorkmateVaultService) DeleteSecret(ctx context.Context, locator store.XWorkmateSecretLocator) error {
	_ = ctx
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	secretMap := s.store[locator.SecretPath]
	if secretMap == nil {
		return nil
	}
	delete(secretMap, locator.SecretKey)
	if len(secretMap) == 0 {
		delete(s.store, locator.SecretPath)
	}
	return nil
}

func (s *memoryXWorkmateVaultService) HasSecret(ctx context.Context, locator store.XWorkmateSecretLocator) (bool, error) {
	_ = ctx
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return false, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	secretMap := s.store[locator.SecretPath]
	if secretMap == nil {
		return false, nil
	}
	_, ok := secretMap[locator.SecretKey]
	return ok, nil
}

func (s *memoryXWorkmateVaultService) ReadSecret(ctx context.Context, locator store.XWorkmateSecretLocator) (string, error) {
	_ = ctx
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return "", fmt.Errorf("vault locator is incomplete")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	secretMap := s.store[locator.SecretPath]
	if secretMap == nil {
		return "", fmt.Errorf("secret not found")
	}
	value, ok := secretMap[locator.SecretKey]
	if !ok {
		return "", fmt.Errorf("secret not found")
	}
	return value, nil
}

func (s *httpXWorkmateVaultService) WriteSecret(ctx context.Context, locator store.XWorkmateSecretLocator, value string) error {
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return fmt.Errorf("vault locator is incomplete")
	}

	data, err := s.readSecretMap(ctx, locator.SecretPath)
	if err != nil {
		return err
	}
	if data == nil {
		data = make(map[string]string)
	}
	data[locator.SecretKey] = value

	body, err := json.Marshal(map[string]any{
		"data": data,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.dataURL(locator.SecretPath), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	return s.do(req, nil)
}

func (s *httpXWorkmateVaultService) DeleteSecret(ctx context.Context, locator store.XWorkmateSecretLocator) error {
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return nil
	}

	data, err := s.readSecretMap(ctx, locator.SecretPath)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	delete(data, locator.SecretKey)

	if len(data) == 0 {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.metadataURL(locator.SecretPath), nil)
		if err != nil {
			return err
		}
		return s.do(req, nil)
	}

	body, err := json.Marshal(map[string]any{
		"data": data,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.dataURL(locator.SecretPath), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.do(req, nil)
}

func (s *httpXWorkmateVaultService) HasSecret(ctx context.Context, locator store.XWorkmateSecretLocator) (bool, error) {
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return false, nil
	}

	data, err := s.readSecretMap(ctx, locator.SecretPath)
	if err != nil {
		return false, err
	}
	_, ok := data[locator.SecretKey]
	return ok, nil
}

func (s *httpXWorkmateVaultService) ReadSecret(ctx context.Context, locator store.XWorkmateSecretLocator) (string, error) {
	store.NormalizeXWorkmateSecretLocator(&locator)
	if locator.SecretPath == "" || locator.SecretKey == "" {
		return "", fmt.Errorf("vault locator is incomplete")
	}
	data, err := s.readSecretMap(ctx, locator.SecretPath)
	if err != nil {
		return "", err
	}
	value, ok := data[locator.SecretKey]
	if !ok {
		return "", fmt.Errorf("secret not found")
	}
	return value, nil
}

func (s *httpXWorkmateVaultService) readSecretMap(ctx context.Context, secretPath string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.dataURL(secretPath), nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := s.do(req, &payload); err != nil {
		if strings.Contains(err.Error(), "vault status 404") {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if payload.Data.Data == nil {
		return map[string]string{}, nil
	}
	return payload.Data.Data, nil
}

func (s *httpXWorkmateVaultService) do(req *http.Request, out any) error {
	req.Header.Set("X-Vault-Token", s.token)
	if s.namespace != "" {
		req.Header.Set("X-Vault-Namespace", s.namespace)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("vault status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *httpXWorkmateVaultService) dataURL(secretPath string) string {
	return fmt.Sprintf("%s/v1/%s/data/%s", s.baseURL, s.mount, strings.Trim(strings.TrimSpace(secretPath), "/"))
}

func (s *httpXWorkmateVaultService) metadataURL(secretPath string) string {
	return fmt.Sprintf("%s/v1/%s/metadata/%s", s.baseURL, s.mount, strings.Trim(strings.TrimSpace(secretPath), "/"))
}

func findXWorkmateManagedTarget(target string) (xworkmateManagedSecretTarget, bool) {
	normalized := strings.ToLower(strings.TrimSpace(target))
	for _, candidate := range xworkmateManagedSecretTargets {
		if candidate.Target == normalized {
			return candidate, true
		}
	}
	return xworkmateManagedSecretTarget{}, false
}

func buildManagedXWorkmateSecretLocator(access *xworkmateAccessContext, userID, target string) (store.XWorkmateSecretLocator, error) {
	managedTarget, ok := findXWorkmateManagedTarget(target)
	if !ok {
		return store.XWorkmateSecretLocator{}, fmt.Errorf("unknown xworkmate secret target: %s", target)
	}
	if access == nil || access.Tenant == nil {
		return store.XWorkmateSecretLocator{}, fmt.Errorf("xworkmate access context is required")
	}

	path := fmt.Sprintf("xworkmate/tenants/%s/shared", access.Tenant.ID)
	if access.ProfileScope == store.XWorkmateProfileScopeUserPrivate {
		trimmedUserID := strings.TrimSpace(userID)
		if trimmedUserID == "" {
			return store.XWorkmateSecretLocator{}, fmt.Errorf("xworkmate private scope user id is required")
		}
		path = fmt.Sprintf("xworkmate/tenants/%s/users/%s", access.Tenant.ID, trimmedUserID)
	}

	locator := store.XWorkmateSecretLocator{
		ID:         strings.Join([]string{"managed", access.Tenant.ID, strings.TrimSpace(userID), access.ProfileScope, managedTarget.Target}, "|"),
		Provider:   store.XWorkmateSecretLocatorProviderVault,
		SecretPath: path,
		SecretKey:  managedTarget.Target,
		Target:     managedTarget.Target,
		Required:   managedTarget.Required,
	}
	store.NormalizeXWorkmateSecretLocator(&locator)
	return locator, nil
}
