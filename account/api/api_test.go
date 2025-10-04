package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type apiResponse struct {
	Message   string                 `json:"message"`
	Error     string                 `json:"error"`
	Token     string                 `json:"token"`
	MFAToken  string                 `json:"mfaToken"`
	User      map[string]interface{} `json:"user"`
	MFA       map[string]interface{} `json:"mfa"`
	Secret    string                 `json:"secret"`
	URI       string                 `json:"uri"`
	ExpiresAt string                 `json:"expiresAt"`
}

type capturedEmail struct {
	To        []string
	Subject   string
	PlainBody string
	HTMLBody  string
}

type testEmailSender struct {
	mu       sync.Mutex
	messages []capturedEmail
}

func (s *testEmailSender) Send(ctx context.Context, msg EmailMessage) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	copyTo := make([]string, len(msg.To))
	copy(copyTo, msg.To)
	s.messages = append(s.messages, capturedEmail{
		To:        copyTo,
		Subject:   msg.Subject,
		PlainBody: msg.PlainBody,
		HTMLBody:  msg.HTMLBody,
	})
	return nil
}

func (s *testEmailSender) last() (capturedEmail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		return capturedEmail{}, false
	}
	return s.messages[len(s.messages)-1], true
}

func extractTokenFromMessage(t *testing.T, msg capturedEmail) string {
	t.Helper()
	re := regexp.MustCompile(`[a-f0-9]{64}`)
	if match := re.FindString(msg.PlainBody); match != "" {
		return match
	}
	if match := re.FindString(msg.HTMLBody); match != "" {
		return match
	}
	t.Fatalf("failed to extract token from email body: %q", msg.PlainBody)
	return ""
}

func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder) apiResponse {
	t.Helper()
	var resp apiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func waitForStableTOTPWindow(t *testing.T) {
	t.Helper()
	const period int64 = 30
	remainder := time.Now().Unix() % period
	const buffer int64 = 10
	if remainder > period-buffer {
		sleep := (period - remainder) + 2
		if sleep > 0 {
			time.Sleep(time.Duration(sleep) * time.Second)
		}
	}
}

func TestRegisterEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	mailer := &testEmailSender{}
	RegisterRoutes(router, WithEmailSender(mailer))

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

	resp := decodeResponse(t, rr)
	if resp.User == nil {
		t.Fatalf("expected user object in response")
	}

	if verified, ok := resp.User["emailVerified"].(bool); !ok || verified {
		t.Fatalf("expected emailVerified to be false after registration, got %#v", resp.User["emailVerified"])
	}

	if email, ok := resp.User["email"].(string); !ok || email != payload["email"] {
		t.Fatalf("expected email %q, got %#v", payload["email"], resp.User["email"])
	}

	if id, ok := resp.User["id"].(string); !ok || id == "" {
		t.Fatalf("expected user id in response")
	} else if uuid, ok := resp.User["uuid"].(string); !ok || uuid != id {
		t.Fatalf("expected uuid to match id")
	}

	if mfaEnabled, ok := resp.User["mfaEnabled"].(bool); !ok || mfaEnabled {
		t.Fatalf("expected mfaEnabled to be false, got %#v", resp.User["mfaEnabled"])
	}

	mfaData, ok := resp.User["mfa"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mfa state in user payload")
	}
	if enabled, ok := mfaData["totpEnabled"].(bool); !ok || enabled {
		t.Fatalf("expected totpEnabled to be false, got %#v", mfaData["totpEnabled"])
	}
	if pending, ok := mfaData["totpPending"].(bool); !ok || pending {
		t.Fatalf("expected totpPending to be false, got %#v", mfaData["totpPending"])
	}

	msg, ok := mailer.last()
	if !ok {
		t.Fatalf("expected verification email to be sent")
	}
	if !strings.Contains(strings.ToLower(msg.Subject), "verify") {
		t.Fatalf("expected verification subject, got %q", msg.Subject)
	}

	token := extractTokenFromMessage(t, msg)
	verifyPayload := map[string]string{"token": token}
	verifyBody, err := json.Marshal(verifyPayload)
	if err != nil {
		t.Fatalf("failed to marshal verification payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/register/verify", bytes.NewReader(verifyBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected verification success, got %d: %s", rr.Code, rr.Body.String())
	}

	resp = decodeResponse(t, rr)
	if resp.User == nil {
		t.Fatalf("expected user in verification response")
	}
	if verified, ok := resp.User["emailVerified"].(bool); !ok || !verified {
		t.Fatalf("expected emailVerified true after verification, got %#v", resp.User["emailVerified"])
	}
}

func TestRegisterEndpointWithoutEmailVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router, WithEmailVerification(false))

	payload := map[string]string{
		"name":     "Another User",
		"email":    "another@example.com",
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

	resp := decodeResponse(t, rr)
	if resp.Message != "registration successful" {
		t.Fatalf("expected success message when verification disabled, got %q", resp.Message)
	}

	if resp.User == nil {
		t.Fatalf("expected user object in response")
	}

	if verified, ok := resp.User["emailVerified"].(bool); !ok || !verified {
		t.Fatalf("expected emailVerified true when verification disabled, got %#v", resp.User["emailVerified"])
	}
}

func TestMFATOTPFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	mailer := &testEmailSender{}
	RegisterRoutes(router, WithEmailSender(mailer))

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

	msg, ok := mailer.last()
	if !ok {
		t.Fatalf("expected verification email during registration")
	}
	token := extractTokenFromMessage(t, msg)
	verifyPayload := map[string]string{"token": token}
	verifyBody, err := json.Marshal(verifyPayload)
	if err != nil {
		t.Fatalf("failed to marshal verify payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/register/verify", bytes.NewReader(verifyBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected verification success, got %d: %s", rr.Code, rr.Body.String())
	}

	loginPayload := map[string]string{
		"identifier": "Login User",
		"password":   registerPayload["password"],
	}
	loginBody, err := json.Marshal(loginPayload)
	if err != nil {
		t.Fatalf("failed to marshal login payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected login to require mfa setup, got %d", rr.Code)
	}
	resp := decodeResponse(t, rr)
	if resp.Error != "mfa_setup_required" {
		t.Fatalf("expected mfa_setup_required error, got %q", resp.Error)
	}
	if resp.MFAToken == "" {
		t.Fatalf("expected mfa token in response")
	}

	provisionPayload := map[string]string{
		"token": resp.MFAToken,
	}
	provisionBody, err := json.Marshal(provisionPayload)
	if err != nil {
		t.Fatalf("failed to marshal provision payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/mfa/totp/provision", bytes.NewReader(provisionBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected provisioning success, got %d: %s", rr.Code, rr.Body.String())
	}
	resp = decodeResponse(t, rr)
	if resp.Secret == "" {
		t.Fatalf("expected totp secret in provisioning response")
	}
	if resp.URI == "" {
		t.Fatalf("expected otpauth uri in provisioning response")
	}
	secret := resp.Secret

	generateCode := func(offset time.Duration) string {
		code, err := totp.GenerateCodeCustom(secret, time.Now().UTC().Add(offset), totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			t.Fatalf("failed to generate verification code: %v", err)
		}
		return code
	}

	waitForStableTOTPWindow(t)
	code := generateCode(-30 * time.Second)

	totpVerifyPayload := map[string]string{
		"token": resp.MFAToken,
		"code":  code,
	}
	totpVerifyBody, err := json.Marshal(totpVerifyPayload)
	if err != nil {
		t.Fatalf("failed to marshal verify payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/mfa/totp/verify", bytes.NewReader(totpVerifyBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected verification success, got %d: %s", rr.Code, rr.Body.String())
	}
	resp = decodeResponse(t, rr)
	if resp.Token == "" {
		t.Fatalf("expected session token after verification")
	}
	if resp.User == nil || resp.User["mfaEnabled"] != true {
		t.Fatalf("expected mfaEnabled true after verification")
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionReq.Header.Set("Authorization", "Bearer "+resp.Token)
	sessionRec := httptest.NewRecorder()
	router.ServeHTTP(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusOK {
		t.Fatalf("expected session lookup success, got %d", sessionRec.Code)
	}
	sessionResp := decodeResponse(t, sessionRec)
	if sessionResp.User == nil {
		t.Fatalf("expected user in session response")
	}
	if sessionResp.User["mfaEnabled"] != true {
		t.Fatalf("expected session user to have mfaEnabled true")
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/mfa/status", nil)
	statusReq.Header.Set("Authorization", "Bearer "+resp.Token)
	statusRec := httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected status success, got %d", statusRec.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/auth/session", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+resp.Token)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected session deletion success, got %d", deleteRec.Code)
	}

	sessionReq = httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionReq.Header.Set("Authorization", "Bearer "+resp.Token)
	sessionRec = httptest.NewRecorder()
	router.ServeHTTP(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected session lookup failure after deletion, got %d", sessionRec.Code)
	}

	statusReq = httptest.NewRequest(http.MethodGet, "/api/auth/mfa/status", nil)
	statusReq.Header.Set("Authorization", "Bearer "+resp.Token)
	statusRec = httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status failure after session deletion, got %d", statusRec.Code)
	}

	loginWithTotp := func(body map[string]string) *httptest.ResponseRecorder {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal login payload: %v", err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	waitForStableTOTPWindow(t)
	totpCode := generateCode(-30 * time.Second)
	if ok, _ := totp.ValidateCustom(totpCode, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}); !ok {
		t.Fatalf("locally generated totp code is invalid")
	}

	rr = loginWithTotp(map[string]string{
		"identifier": "Login User",
		"password":   registerPayload["password"],
		"totpCode":   totpCode,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected mfa login success, got %d: %s", rr.Code, rr.Body.String())
	}

	identifierStatusReq := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/mfa/status?"+url.Values{"identifier": {registerPayload["email"]}}.Encode(),
		nil,
	)
	identifierStatusRec := httptest.NewRecorder()
	router.ServeHTTP(identifierStatusRec, identifierStatusReq)
	if identifierStatusRec.Code != http.StatusOK {
		t.Fatalf("expected identifier status success, got %d: %s", identifierStatusRec.Code, identifierStatusRec.Body.String())
	}
	identifierStatusResp := decodeResponse(t, identifierStatusRec)
	if identifierStatusResp.MFA == nil {
		t.Fatalf("expected mfa payload in identifier status response")
	}
	if enabled, ok := identifierStatusResp.MFA["totpEnabled"].(bool); !ok || !enabled {
		t.Fatalf("expected identifier status to report totpEnabled true, got %#v", identifierStatusResp.MFA)
	}

	waitForStableTOTPWindow(t)
	totpCode = generateCode(0)
	if ok, _ := totp.ValidateCustom(totpCode, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}); !ok {
		t.Fatalf("locally generated totp code is invalid (email login)")
	}

	rr = loginWithTotp(map[string]string{
		"identifier": registerPayload["email"],
		"totpCode":   totpCode,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected email+totp login success, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHealthzEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected healthz endpoint to return 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode healthz response: %v", err)
	}
	if status := resp["status"]; status != "ok" {
		t.Fatalf("expected health status 'ok', got %q", status)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	mailer := &testEmailSender{}
	RegisterRoutes(router, WithEmailSender(mailer))

	registerPayload := map[string]string{
		"name":     "Reset User",
		"email":    "reset@example.com",
		"password": "originalPass1",
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
		t.Fatalf("expected registration success, got %d: %s", rr.Code, rr.Body.String())
	}

	msg, ok := mailer.last()
	if !ok {
		t.Fatalf("expected verification email during registration")
	}
	verifyToken := extractTokenFromMessage(t, msg)
	verifyPayload := map[string]string{"token": verifyToken}
	verifyBody, err := json.Marshal(verifyPayload)
	if err != nil {
		t.Fatalf("failed to marshal verification payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/register/verify", bytes.NewReader(verifyBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected verification success, got %d: %s", rr.Code, rr.Body.String())
	}

	resetPayload := map[string]string{"email": registerPayload["email"]}
	resetBody, err := json.Marshal(resetPayload)
	if err != nil {
		t.Fatalf("failed to marshal reset payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/password/reset", bytes.NewReader(resetBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected password reset request to return 202, got %d: %s", rr.Code, rr.Body.String())
	}

	msg, ok = mailer.last()
	if !ok {
		t.Fatalf("expected password reset email to be sent")
	}
	if !strings.Contains(strings.ToLower(msg.Subject), "reset") {
		t.Fatalf("expected reset subject, got %q", msg.Subject)
	}
	resetToken := extractTokenFromMessage(t, msg)

	confirmPayload := map[string]string{
		"token":    resetToken,
		"password": "newSecurePass2",
	}
	confirmBody, err := json.Marshal(confirmPayload)
	if err != nil {
		t.Fatalf("failed to marshal confirm payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/password/reset/confirm", bytes.NewReader(confirmBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected password reset confirmation success, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeResponse(t, rr)
	if resp.User == nil {
		t.Fatalf("expected user in reset confirmation response")
	}
	if verified, ok := resp.User["emailVerified"].(bool); !ok || !verified {
		t.Fatalf("expected email to remain verified after reset")
	}

	loginPayload := map[string]string{
		"identifier": registerPayload["name"],
		"password":   confirmPayload["password"],
	}
	loginBody, err := json.Marshal(loginPayload)
	if err != nil {
		t.Fatalf("failed to marshal login payload: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected login to prompt for mfa setup, got %d: %s", rr.Code, rr.Body.String())
	}
	resp = decodeResponse(t, rr)
	if resp.Error != "mfa_setup_required" {
		t.Fatalf("expected mfa_setup_required after password reset, got %q", resp.Error)
	}

	loginPayload["password"] = registerPayload["password"]
	loginBody, err = json.Marshal(loginPayload)
	if err != nil {
		t.Fatalf("failed to marshal old password payload: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected login with old password to fail, got %d", rr.Code)
	}
	resp = decodeResponse(t, rr)
	if resp.Error == "" {
		t.Fatalf("expected error when logging in with old password")
	}
}
