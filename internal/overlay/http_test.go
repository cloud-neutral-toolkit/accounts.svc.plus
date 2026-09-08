package overlay

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOverlayHTTPTest(t *testing.T) (*Service, *gin.Engine, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	service, err := NewService(db, Config{SigningKeyID: "zero-key-1", SigningPrivateKey: signer, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	joinToken, err := service.Seed(t.Context(), BootstrapConfig{Network: BootstrapNetwork{ID: "sit-private", DisplayName: "SIT private", CIDR: "10.77.0.0/29", GatewayID: "gw-sit-1", GatewayWireGuardKey: gatewayKey, GatewayWireGuardAddress: "10.77.0.1/32", GatewayEndpointHost: "gw.example.test", GatewayEndpointPort: 51820, TransportServerName: "gw.example.test", TransportPort: 443, TransportAuthID: "11111111-1111-1111-1111-111111111111", OwnerUserID: "11111111-2222-4333-8444-555555555555"}, Invite: BootstrapInvite{Platform: "linux", Role: RoleOne, ExpiresAt: now.Add(time.Hour)}}, "")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler := NewHTTPHandler(service)
	router.POST("/api/overlay/v1/join-tokens/exchange", handler.Exchange)
	router.POST("/api/overlay/v1/device/session", handler.MintSession)
	router.GET("/api/overlay/v1/enrollment/signed-config", handler.EnrollmentConfig)
	router.POST("/api/overlay/v1/enrollment/signed-config/:generation/ack", handler.AckEnrollment)
	router.GET("/api/overlay/v1/enrollment/policy-artifacts/:generation/:digest", handler.PolicyArtifact)
	router.GET("/api/overlay/v1/gateway/signed-config", handler.GatewayConfig)
	return service, router, joinToken
}

func TestOverlayLifecyclePersistsHashesAndSignsConfig(t *testing.T) {
	service, router, joinToken := newOverlayHTTPTest(t)
	publicKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	payload := `{"join_token":"` + joinToken + `","device_id":"one-laptop","name":"Laptop","platform":"linux","hostname":"laptop","wireguard_public_key":"` + publicKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/overlay/v1/join-tokens/exchange", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || resp.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("exchange status=%d headers=%v body=%s", resp.Code, resp.Header(), resp.Body.String())
	}
	var exchange ExchangeResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &exchange); err != nil {
		t.Fatal(err)
	}
	if exchange.Device.Role != "" || exchange.Device.WireGuardAddress != "10.77.0.2/32" {
		t.Fatalf("unexpected One device response: %#v", exchange.Device)
	}
	var exchangePayload map[string]json.RawMessage
	if err := json.Unmarshal(resp.Body.Bytes(), &exchangePayload); err != nil {
		t.Fatal(err)
	}
	var exchangeDevicePayload map[string]json.RawMessage
	if err := json.Unmarshal(exchangePayload["device"], &exchangeDevicePayload); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"role", "status", "connection_status"} {
		if _, ok := exchangeDevicePayload[field]; ok {
			t.Fatalf("strict exchange Device contract unexpectedly exposed admin field %q: %s", field, exchangeDevicePayload[field])
		}
	}
	var storedDevice DeviceRecord
	if err := service.repo.DB.First(&storedDevice, "id = ?", exchange.Device.ID).Error; err != nil || storedDevice.UserUUID != storedDevice.UserID || storedDevice.UserID != "11111111-2222-4333-8444-555555555555" {
		t.Fatalf("device owner compatibility columns diverged: %#v err=%v", storedDevice, err)
	}
	var credentials []CredentialRecord
	if err := service.repo.DB.Find(&credentials).Error; err != nil || len(credentials) != 1 || credentials[0].TokenHash == exchange.DeviceCredential.Credential {
		t.Fatalf("credential storage is unsafe: err=%v records=%#v", err, credentials)
	}
	credentialRawID := strings.TrimPrefix(exchange.DeviceCredential.CredentialID, "xdcid_")
	if !strings.HasPrefix(exchange.DeviceCredential.Credential, "xdc_"+credentialRawID+".") {
		t.Fatalf("device credential is not bound to credential_id")
	}
	var invites []InviteRecord
	if err := service.repo.DB.Find(&invites).Error; err != nil || len(invites) != 1 || invites[0].TokenHash == joinToken || invites[0].RemainingUses != 0 {
		t.Fatalf("invite storage is unsafe: err=%v records=%#v", err, invites)
	}

	sessionBody := `{"client_nonce":"00000000-0000-4000-8000-000000000001"}`
	sessionReq := httptest.NewRequest(http.MethodPost, "/api/overlay/v1/device/session", bytes.NewBufferString(sessionBody))
	sessionReq.Header.Set("Authorization", "Device "+exchange.DeviceCredential.Credential)
	sessionResp := httptest.NewRecorder()
	router.ServeHTTP(sessionResp, sessionReq)
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("session status=%d body=%s", sessionResp.Code, sessionResp.Body.String())
	}
	var session DeviceSessionResponse
	if err := json.Unmarshal(sessionResp.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}

	configReq := httptest.NewRequest(http.MethodGet, "/api/overlay/v1/enrollment/signed-config?device_id=one-laptop&network_id=sit-private", nil)
	configReq.Header.Set("Authorization", "Bearer "+session.EnrollmentToken)
	configResp := httptest.NewRecorder()
	router.ServeHTTP(configResp, configReq)
	if configResp.Code != http.StatusOK || configResp.Header().Get("Cache-Control") != "private, no-store" || configResp.Header().Get("ETag") == "" {
		t.Fatalf("config status=%d headers=%v body=%s", configResp.Code, configResp.Header(), configResp.Body.String())
	}
	var config SignedConfig
	if err := json.Unmarshal(configResp.Body.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	unsigned, err := signingBytes(config)
	if err != nil || !ed25519.Verify(service.privateKey.Public().(ed25519.PublicKey), unsigned, mustDecodeBase64(t, config.Signature.Value)) {
		t.Fatalf("signed config verification failed: err=%v config=%#v", err, config)
	}
	v2Req := httptest.NewRequest(http.MethodGet, "/api/overlay/v1/enrollment/signed-config?device_id=one-laptop&network_id=sit-private", nil)
	v2Req.Header.Set("Authorization", "Bearer "+session.EnrollmentToken)
	v2Req.Header.Set("Accept", SignedConfigV2MediaType)
	v2Resp := httptest.NewRecorder()
	router.ServeHTTP(v2Resp, v2Req)
	if v2Resp.Code != http.StatusOK || v2Resp.Header().Get("Content-Type") != SignedConfigV2MediaType {
		t.Fatalf("v2 config status=%d content-type=%q body=%s", v2Resp.Code, v2Resp.Header().Get("Content-Type"), v2Resp.Body.String())
	}
	var v2 SignedConfig
	if err := json.Unmarshal(v2Resp.Body.Bytes(), &v2); err != nil || v2.SchemaVersion != 2 || v2.Policy == nil {
		t.Fatalf("invalid v2 config: err=%v config=%#v", err, v2)
	}
	policyReq := httptest.NewRequest(http.MethodGet, v2.Policy.Path, nil)
	policyReq.Header.Set("Authorization", "Bearer "+session.EnrollmentToken)
	policyResp := httptest.NewRecorder()
	router.ServeHTTP(policyResp, policyReq)
	if policyResp.Code != http.StatusOK || policyResp.Header().Get("Content-Type") != PolicyMediaType {
		t.Fatalf("policy status=%d content-type=%q body=%s", policyResp.Code, policyResp.Header().Get("Content-Type"), policyResp.Body.String())
	}

	ackBody := `{"config_id":"` + config.ConfigID + `","device_id":"one-laptop","applied_at":"2026-09-07T12:00:00Z"}`
	ackReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/overlay/v1/enrollment/signed-config/%d/ack", config.Generation), bytes.NewBufferString(ackBody))
	ackReq.Header.Set("Authorization", "Bearer "+session.EnrollmentToken)
	ackResp := httptest.NewRecorder()
	router.ServeHTTP(ackResp, ackReq)
	if ackResp.Code != http.StatusOK || !bytes.Contains(ackResp.Body.Bytes(), []byte(`"acked":true`)) {
		t.Fatalf("ack status=%d body=%s", ackResp.Code, ackResp.Body.String())
	}
	var acknowledgedDevice DeviceRecord
	if err := service.repo.DB.Where("id = ?", "one-laptop").First(&acknowledgedDevice).Error; err != nil {
		t.Fatal(err)
	}
	if acknowledgedDevice.LastSeenAt == nil {
		t.Fatal("successful signed-config ACK did not update device activity")
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/api/overlay/v1/join-tokens/exchange", bytes.NewBufferString(payload))
	replayResp := httptest.NewRecorder()
	router.ServeHTTP(replayResp, replayReq)
	if replayResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected one-use invite rejection, got %d: %s", replayResp.Code, replayResp.Body.String())
	}
}

func TestGatewayRoleUsesCentralSignedSnapshot(t *testing.T) {
	service, router, _ := newOverlayHTTPTest(t)
	joinToken, err := service.Seed(t.Context(), BootstrapConfig{
		Network: BootstrapNetwork{ID: "uat-private", DisplayName: "UAT private", CIDR: "10.88.0.0/29", GatewayID: "gw-uat-1", GatewayWireGuardKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32)), GatewayWireGuardAddress: "10.88.0.1/32", GatewayEndpointHost: "gw.uat.test", GatewayEndpointPort: 51820, TransportServerName: "gw.uat.test", TransportPort: 443, TransportAuthID: "22222222-2222-2222-2222-222222222222"},
		Invite:  BootstrapInvite{Role: RoleGateway, Platform: "linux", ExpiresAt: time.Now().UTC().Add(time.Hour)},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"join_token":"` + joinToken + `","device_id":"gw-uat-1","platform":"linux","role":"gateway","wireguard_public_key":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32)) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/overlay/v1/join-tokens/exchange", bytes.NewBufferString(payload))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("gateway exchange status=%d body=%s", resp.Code, resp.Body.String())
	}
	var exchange ExchangeResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &exchange); err != nil {
		t.Fatal(err)
	}
	if exchange.Device.WireGuardAddress != "10.88.0.1/32" {
		t.Fatalf("gateway received non-gateway address: %q", exchange.Device.WireGuardAddress)
	}
	gatewayReq := httptest.NewRequest(http.MethodGet, "/api/overlay/v1/gateway/signed-config", nil)
	gatewayReq.Header.Set("Authorization", "Bearer "+exchange.EnrollmentToken)
	gatewayResp := httptest.NewRecorder()
	router.ServeHTTP(gatewayResp, gatewayReq)
	if gatewayResp.Code != http.StatusOK {
		t.Fatalf("gateway config status=%d body=%s", gatewayResp.Code, gatewayResp.Body.String())
	}
	var config GatewaySignedConfig
	if err := json.Unmarshal(gatewayResp.Body.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	if config.Role != RoleGateway || config.GatewayID != "gw-uat-1" || config.Generation != 2 || config.Signature.KeyID != "zero-key-1" {
		t.Fatalf("unexpected gateway config: %#v", config)
	}
	unsigned, err := gatewaySigningBytes(config)
	if err != nil || !ed25519.Verify(service.privateKey.Public().(ed25519.PublicKey), unsigned, mustDecodeBase64(t, config.Signature.Value)) {
		t.Fatalf("gateway signed config verification failed: err=%v", err)
	}
}

func mustDecodeBase64(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
