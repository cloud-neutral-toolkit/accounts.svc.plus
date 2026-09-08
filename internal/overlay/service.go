package overlay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var overlayDeviceIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

type Service struct {
	repo            *Repository
	keyID           string
	privateKey      ed25519.PrivateKey
	enrollmentTTL   time.Duration
	credentialTTL   time.Duration
	signedConfigTTL time.Duration
	clock           func() time.Time
}

const adminDeviceACKRecentWindow = 5 * time.Minute

func NewService(db *gorm.DB, cfg Config) (*Service, error) {
	if db == nil {
		return nil, errors.New("overlay service requires a database")
	}
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("migrate overlay schema: %w", err)
	}
	cfg = cfg.withDefaults()
	key := append(ed25519.PrivateKey(nil), cfg.SigningPrivateKey...)
	if len(key) == 0 {
		var err error
		_, key, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate overlay signing key: %w", err)
		}
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("overlay signing key must be an Ed25519 private key")
	}
	repo, err := NewRepository(db)
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, keyID: cfg.SigningKeyID, privateKey: key, enrollmentTTL: cfg.EnrollmentTTL, credentialTTL: cfg.CredentialTTL, signedConfigTTL: cfg.SignedConfigTTL, clock: cfg.Clock}, nil
}

// ConfigFromEnv is intended for the account service entrypoint. The private
// key is injected by the deployment secret manager (Vault in production),
// encoded as unpadded or padded base64. Development may omit it and receives
// an ephemeral key, which makes the limitation explicit at the boundary.
func ConfigFromEnv() (Config, error) {
	keyText := strings.TrimSpace(os.Getenv("XCONNECT_OVERLAY_SIGNING_PRIVATE_KEY"))
	if keyText == "" {
		return Config{SigningKeyID: strings.TrimSpace(os.Getenv("XCONNECT_OVERLAY_SIGNING_KEY_ID"))}, nil
	}
	key, err := base64.StdEncoding.DecodeString(keyText)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(keyText)
	}
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return Config{}, errors.New("XCONNECT_OVERLAY_SIGNING_PRIVATE_KEY must be base64 Ed25519 private key")
	}
	return Config{SigningKeyID: strings.TrimSpace(os.Getenv("XCONNECT_OVERLAY_SIGNING_KEY_ID")), SigningPrivateKey: ed25519.PrivateKey(key)}, nil
}

func (s *Service) Repository() *Repository { return s.repo }

func (s *Service) AdminOverview(ctx context.Context, ownerUserID string) (AdminOverview, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return AdminOverview{}, ErrInvalidInput
	}
	var networks int64
	if err := s.repo.DB.WithContext(ctx).Model(&NetworkRecord{}).Where("owner_user_id = ?", ownerUserID).Count(&networks).Error; err != nil {
		return AdminOverview{}, err
	}
	adminDevices, err := s.AdminDevices(ctx, ownerUserID)
	if err != nil {
		return AdminOverview{}, err
	}
	var devices, gateways, ones, connectedGateways, connectedOnes int64
	for _, device := range adminDevices {
		if device.Status != "active" {
			continue
		}
		devices++
		connected := device.ConnectionStatus == "recent_ack"
		switch device.Role {
		case RoleGateway:
			gateways++
			if connected {
				connectedGateways++
			}
		case RoleOne:
			ones++
			if connected {
				connectedOnes++
			}
		}
	}
	return AdminOverview{
		Status:       "available",
		NetworkCount: networks,
		DeviceCount:  devices,
		GatewayCount: gateways,
		OneCount:     ones,
		// These legacy overview fields mean recent current-generation ACK,
		// not a live tunnel or WireGuard handshake.
		GatewayStatus: overlayResourceStatus(gateways, connectedGateways, networks),
		OneStatus:     overlayResourceStatus(ones, connectedOnes, networks),
		SigningKeyID:  s.keyID,
	}, nil
}

// The admin read model reports lifecycle state and recent configuration ACKs,
// not a fabricated network handshake. A recent_ack only means that Accounts
// received a current-generation configuration ACK; it does not claim that a
// live tunnel or WireGuard handshake exists.

func overlayResourceStatus(activeCount, connectedCount, networkCount int64) string {
	if connectedCount > 0 {
		return "connected"
	}
	if activeCount > 0 {
		return "active"
	}
	if networkCount > 0 {
		return "pending"
	}
	return "not_configured"
}

func (s *Service) AdminNetworks(ctx context.Context, ownerUserID string) ([]Network, error) {
	var records []NetworkRecord
	if err := s.repo.DB.WithContext(ctx).Where("owner_user_id = ?", strings.TrimSpace(ownerUserID)).Order("id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]Network, 0, len(records))
	for _, record := range records {
		result = append(result, toNetwork(record))
	}
	return result, nil
}

func (s *Service) AdminDevices(ctx context.Context, ownerUserID string) ([]AdminDevice, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	var networks []NetworkRecord
	if err := s.repo.DB.WithContext(ctx).Where("owner_user_id = ?", ownerUserID).Order("id ASC").Find(&networks).Error; err != nil {
		return nil, err
	}
	if len(networks) == 0 {
		return []AdminDevice{}, nil
	}

	networkByID := make(map[string]NetworkRecord, len(networks))
	networkIDs := make([]string, 0, len(networks))
	for _, network := range networks {
		networkByID[network.ID] = network
		networkIDs = append(networkIDs, network.ID)
	}

	var records []DeviceRecord
	if err := s.repo.DB.WithContext(ctx).Where("network_id IN ?", networkIDs).Order("id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []AdminDevice{}, nil
	}

	var acks []AckRecord
	if err := s.repo.DB.WithContext(ctx).
		Model(&AckRecord{}).
		Joins("JOIN overlay_networks ON overlay_networks.id = overlay_signed_config_acks.network_id").
		Where("overlay_networks.owner_user_id = ? AND overlay_signed_config_acks.generation = overlay_networks.config_generation", ownerUserID).
		Find(&acks).Error; err != nil {
		return nil, err
	}
	currentACK := make(map[string]time.Time, len(acks))
	for _, ack := range acks {
		network, ok := networkByID[ack.NetworkID]
		if !ok || ack.Generation != network.ConfigGeneration || ack.ConfigID != configID(network.ID, ack.DeviceID, network.ConfigGeneration) {
			continue
		}
		currentACK[adminDeviceACKKey(ack.NetworkID, ack.DeviceID)] = ack.ReceivedAt
	}

	now := s.now()
	result := make([]AdminDevice, 0, len(records))
	for _, record := range records {
		network, ok := networkByID[record.NetworkID]
		if !ok {
			continue
		}
		ackReceivedAt, acknowledged := currentACK[adminDeviceACKKey(network.ID, record.ID)]
		result = append(result, toAdminDevice(record, ackReceivedAt, acknowledged, now))
	}
	return result, nil
}

func adminDeviceACKKey(networkID, deviceID string) string { return networkID + "\x00" + deviceID }

func (s *Service) AdminInvites(ctx context.Context, ownerUserID string) ([]InviteSummary, error) {
	var records []InviteRecord
	ownedNetworks := s.repo.DB.WithContext(ctx).Model(&NetworkRecord{}).Select("id").Where("owner_user_id = ?", strings.TrimSpace(ownerUserID))
	if err := s.repo.DB.WithContext(ctx).Where("network_id IN (?)", ownedNetworks).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]InviteSummary, 0, len(records))
	for _, record := range records {
		result = append(result, InviteSummary{ID: record.ID, NetworkID: record.NetworkID, DeviceID: record.DeviceID, Platform: record.Platform, Role: record.Role, ExpiresAt: record.ExpiresAt.UTC(), RemainingUses: record.RemainingUses, ConsumedAt: record.ConsumedAt, CreatedAt: record.CreatedAt.UTC()})
	}
	return result, nil
}

// AdminCreateInvite creates a single device-bound enrollment invitation for an
// existing network owned by the authenticated account. The raw invitation is
// intentionally returned only in this response; persistence keeps its digest.
func (s *Service) AdminCreateInvite(ctx context.Context, ownerUserID, controllerURL string, request AdminInviteRequest) (AdminInviteResult, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	controllerURL = strings.TrimRight(strings.TrimSpace(controllerURL), "/")
	request.NetworkID = strings.TrimSpace(request.NetworkID)
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	request.Platform = strings.TrimSpace(request.Platform)
	request.Role = strings.TrimSpace(request.Role)
	if ownerUserID == "" || controllerURL == "" || request.NetworkID == "" || !overlayDeviceIDPattern.MatchString(request.DeviceID) || request.ExpiresAt.IsZero() || !request.ExpiresAt.After(s.now()) {
		return AdminInviteResult{}, ErrInvalidInput
	}
	if request.Role == "" {
		request.Role = RoleOne
	}
	if request.Role != RoleOne && request.Role != RoleGateway {
		return AdminInviteResult{}, ErrInvalidInput
	}
	switch request.Platform {
	case "linux", "darwin", "windows", "ios", "android":
	default:
		return AdminInviteResult{}, ErrInvalidInput
	}

	var network NetworkRecord
	if err := s.repo.DB.WithContext(ctx).Where("id = ? AND owner_user_id = ?", request.NetworkID, ownerUserID).First(&network).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AdminInviteResult{}, ErrNotFound
		}
		return AdminInviteResult{}, err
	}

	joinToken := newOpaqueToken("xjt_")
	invite := InviteRecord{
		ID:            uuid.NewString(),
		NetworkID:     network.ID,
		TokenHash:     HashSecret(joinToken),
		DeviceID:      request.DeviceID,
		Platform:      request.Platform,
		Role:          request.Role,
		ExpiresAt:     canonicalTime(request.ExpiresAt),
		RemainingUses: 1,
	}
	if err := s.repo.DB.WithContext(ctx).Create(&invite).Error; err != nil {
		return AdminInviteResult{}, err
	}
	return AdminInviteResult{
		Invite:  InviteSummary{ID: invite.ID, NetworkID: invite.NetworkID, DeviceID: invite.DeviceID, Platform: invite.Platform, Role: invite.Role, ExpiresAt: invite.ExpiresAt, RemainingUses: invite.RemainingUses, CreatedAt: invite.CreatedAt.UTC()},
		JoinURI: "xconnect://join/" + joinToken + "?controller=" + url.QueryEscape(controllerURL),
	}, nil
}

func (s *Service) AdminBootstrap(ctx context.Context, cfg BootstrapConfig, controllerURL, joinToken string) (AdminBootstrapResult, error) {
	joinToken, err := s.Seed(ctx, cfg, joinToken)
	if err != nil {
		return AdminBootstrapResult{}, err
	}
	var network NetworkRecord
	if err := s.repo.DB.WithContext(ctx).Where("id = ?", cfg.Network.ID).First(&network).Error; err != nil {
		return AdminBootstrapResult{}, err
	}
	var invite InviteRecord
	if err := s.repo.DB.WithContext(ctx).Where("token_hash = ?", HashSecret(joinToken)).First(&invite).Error; err != nil {
		return AdminBootstrapResult{}, err
	}
	controllerURL = strings.TrimRight(strings.TrimSpace(controllerURL), "/")
	if controllerURL == "" {
		return AdminBootstrapResult{}, ErrInvalidInput
	}
	return AdminBootstrapResult{Network: toNetwork(network), Invite: InviteSummary{ID: invite.ID, NetworkID: invite.NetworkID, DeviceID: invite.DeviceID, Platform: invite.Platform, Role: invite.Role, ExpiresAt: invite.ExpiresAt.UTC(), RemainingUses: invite.RemainingUses, CreatedAt: invite.CreatedAt.UTC()}, JoinURI: "xconnect://join/" + joinToken + "?controller=" + url.QueryEscape(controllerURL)}, nil
}

func (s *Service) AdminRevokeDevice(ctx context.Context, ownerUserID, deviceID string) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	deviceID = strings.TrimSpace(deviceID)
	if ownerUserID == "" || deviceID == "" {
		return ErrInvalidInput
	}
	now := s.now()
	return s.repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ownedNetworks := tx.Model(&NetworkRecord{}).Select("id").Where("owner_user_id = ?", ownerUserID)
		result := tx.Model(&DeviceRecord{}).Where("id = ? AND network_id IN (?) AND status = ?", deviceID, ownedNetworks, "active").Updates(map[string]any{"status": "revoked", "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Model(&CredentialRecord{}).Where("device_id = ? AND revoked_at IS NULL", deviceID).Update("revoked_at", now).Error
	})
}

func (s *Service) AdminPolicy(ctx context.Context, ownerUserID, networkID string) (PolicyArtifact, error) {
	var network NetworkRecord
	if err := s.repo.DB.WithContext(ctx).Where("id = ? AND owner_user_id = ?", strings.TrimSpace(networkID), strings.TrimSpace(ownerUserID)).First(&network).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PolicyArtifact{}, ErrNotFound
		}
		return PolicyArtifact{}, err
	}
	return s.policyArtifact(network)
}

func (s *Service) AdminUpdatePolicy(ctx context.Context, ownerUserID, networkID string, policy PolicyArtifact) (PolicyArtifact, error) {
	var result PolicyArtifact
	err := s.repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var network NetworkRecord
		if err := tx.Where("id = ? AND owner_user_id = ?", strings.TrimSpace(networkID), strings.TrimSpace(ownerUserID)).First(&network).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if policy.NetworkID != "" && policy.NetworkID != network.ID || policy.DefaultAction != "" && policy.DefaultAction != "deny" && policy.DefaultAction != "allow" {
			return ErrInvalidInput
		}
		generation := network.ConfigGeneration + 1
		policy.SchemaVersion = 1
		policy.CompilerVersion = PolicyCompilerVersion
		policy.NetworkID = network.ID
		policy.Revision = generation
		if policy.DefaultAction == "" {
			policy.DefaultAction = "deny"
		}
		raw, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		if err := tx.Model(&NetworkRecord{}).Where("id = ?", network.ID).Updates(map[string]any{"policy_json": string(raw), "config_generation": generation}).Error; err != nil {
			return err
		}
		result = policy
		return nil
	})
	return result, err
}

func (s *Service) Seed(ctx context.Context, cfg BootstrapConfig, joinToken string) (string, error) {
	return s.repo.Seed(ctx, cfg, joinToken)
}

func (s *Service) SigningKeys(now time.Time) []SigningKey {
	now = canonicalTime(now)
	return []SigningKey{{KeyID: s.keyID, Algorithm: "Ed25519", PublicKey: base64.StdEncoding.EncodeToString(s.privateKey.Public().(ed25519.PublicKey)), Status: "current", NotBefore: now.Add(-time.Minute)}}
}

func (s *Service) Exchange(ctx context.Context, request ExchangeRequest) (ExchangeResponse, error) {
	if request.Role == "" {
		request.Role = RoleOne
	}
	if err := validateExchange(request); err != nil {
		return ExchangeResponse{}, err
	}
	now := s.now()
	tokenHash := HashSecret(request.JoinToken)
	invite, err := s.repo.peekInvite(ctx, tokenHash)
	if err != nil {
		return ExchangeResponse{}, err
	}
	if invite.DeviceID != "" && invite.DeviceID != request.DeviceID || invite.Platform != "" && invite.Platform != request.Platform || invite.Role != "" && invite.Role != request.Role {
		return ExchangeResponse{}, ErrInviteConstraint
	}
	credentialRawID := randomHex(16)
	credentialID := "xdcid_" + credentialRawID
	credentialSecret := "xdc_" + credentialRawID + "." + newOpaqueToken("")
	enrollmentSecret := newOpaqueToken("xenr_")
	credentialIssued := canonicalTime(now)
	credentialExpiry := canonicalTime(now.Add(s.credentialTTL))
	enrollmentExpiry := canonicalTime(now.Add(s.enrollmentTTL))
	var device DeviceRecord
	var network NetworkRecord
	var createErr error
	invite, device, network, createErr = s.repo.createDevice(ctx, tokenHash, request, now, CredentialRecord{ID: newID("cred"), CredentialID: credentialID, TokenHash: HashSecret(credentialSecret), IssuedAt: credentialIssued, ExpiresAt: credentialExpiry}, EnrollmentRecord{ID: newID("enr"), TokenHash: HashSecret(enrollmentSecret), IssuedAt: credentialIssued, ExpiresAt: enrollmentExpiry})
	err = createErr
	if err != nil {
		return ExchangeResponse{}, err
	}
	return ExchangeResponse{EnrollmentToken: enrollmentSecret, TokenType: TokenTypeBearer, ExpiresAt: enrollmentExpiry, Scope: []string{ScopeConfigRead, ScopeConfigAck, ScopeDeviceRevoke}, DeviceCredential: DeviceCredential{CredentialID: credentialID, Credential: credentialSecret, TokenType: TokenTypeDevice, IssuedAt: credentialIssued, ExpiresAt: credentialExpiry, Scope: []string{ScopeSessionMint, ScopeRotate, ScopeDeviceRevoke}}, Device: toDevice(device), Network: toNetwork(network), SigningKeys: s.SigningKeys(now)}, nil
}

func (s *Service) MintSession(ctx context.Context, credential string, request DeviceSessionRequest) (DeviceSessionResponse, error) {
	if strings.TrimSpace(request.ClientNonce) == "" || len(request.ClientNonce) > 128 {
		return DeviceSessionResponse{}, ErrInvalidInput
	}
	credentialRecord, device, network, err := s.repo.credential(ctx, HashSecret(credential))
	if err != nil || credentialRecord.RevokedAt != nil || !credentialRecord.ExpiresAt.After(s.now()) || device.Status != "active" {
		return DeviceSessionResponse{}, ErrInvalidToken
	}
	now := s.now()
	enrollmentSecret := newOpaqueToken("xenr_")
	expires := canonicalTime(now.Add(s.enrollmentTTL))
	if err := s.repo.DB.WithContext(ctx).Create(&EnrollmentRecord{ID: newID("enr"), DeviceID: device.ID, TokenHash: HashSecret(enrollmentSecret), IssuedAt: canonicalTime(now), ExpiresAt: expires}).Error; err != nil {
		return DeviceSessionResponse{}, err
	}
	return DeviceSessionResponse{ClientNonce: request.ClientNonce, EnrollmentToken: enrollmentSecret, TokenType: TokenTypeBearer, IssuedAt: canonicalTime(now), ExpiresAt: expires, Scope: []string{ScopeConfigRead, ScopeConfigAck}, DeviceID: device.ID, NetworkID: network.ID, SigningKeys: s.SigningKeys(now)}, nil
}

func (s *Service) EnrollmentConfig(ctx context.Context, enrollmentToken, deviceID, networkID string, v2 bool) (SignedConfig, string, error) {
	enrollment, device, network, err := s.repo.enrollment(ctx, HashSecret(enrollmentToken))
	if err != nil || enrollment.ConsumedAt != nil || !enrollment.ExpiresAt.After(s.now()) {
		return SignedConfig{}, "", ErrInvalidToken
	}
	if strings.TrimSpace(deviceID) != device.ID || networkID != "" && networkID != network.ID {
		return SignedConfig{}, "", ErrForbidden
	}
	return s.buildSignedConfig(device, network, v2)
}

func (s *Service) UserConfig(ctx context.Context, userID, deviceID, networkID string, v2 bool) (SignedConfig, string, error) {
	device, network, err := s.repo.userDevice(ctx, userID, deviceID)
	if err != nil {
		return SignedConfig{}, "", err
	}
	if networkID != "" && networkID != network.ID {
		return SignedConfig{}, "", ErrForbidden
	}
	return s.buildSignedConfig(device, network, v2)
}

func (s *Service) GatewayConfig(ctx context.Context, enrollmentToken string) (GatewaySignedConfig, string, error) {
	enrollment, device, network, err := s.repo.enrollment(ctx, HashSecret(enrollmentToken))
	if err != nil || enrollment.ConsumedAt != nil || !enrollment.ExpiresAt.After(s.now()) || device.Status != "active" {
		return GatewaySignedConfig{}, "", ErrInvalidToken
	}
	if device.Role != RoleGateway || device.ID != network.GatewayID {
		return GatewaySignedConfig{}, "", ErrForbidden
	}
	devices, err := s.repo.devices(ctx, network.ID)
	if err != nil {
		return GatewaySignedConfig{}, "", err
	}
	peers := make([]GatewayPeer, 0, len(devices))
	for _, candidate := range devices {
		if candidate.Role != RoleOne {
			continue
		}
		peers = append(peers, GatewayPeer{DeviceID: candidate.ID, WireGuardPublicKey: candidate.WireGuardPublicKey, WireGuardAddress: candidate.WireGuardAddress, AllowedIPs: candidate.WireGuardAddress})
	}
	now := canonicalTime(s.now())
	address := network.GatewayWireGuardAddress
	if address == "" {
		prefix, parseErr := netip.ParsePrefix(network.CIDR)
		if parseErr != nil {
			return GatewaySignedConfig{}, "", ErrInvalidInput
		}
		address = prefix.Addr().Next().String() + "/32"
	}
	config := GatewaySignedConfig{SchemaVersion: 1, Role: RoleGateway, ConfigID: configID(network.ID, network.GatewayID, network.ConfigGeneration), NetworkID: network.ID, GatewayID: network.GatewayID, Generation: network.ConfigGeneration, IssuedAt: now, ExpiresAt: canonicalTime(now.Add(s.signedConfigTTL)), InterfaceName: "xconzero0", Address: address, ListenPort: network.GatewayEndpointPort, MTU: 1420, Peers: peers, Transport: GatewayTransport{ServerName: network.TransportServerName, Port: network.TransportPort, AuthID: network.TransportAuthID}}
	payload, err := gatewaySigningBytes(config)
	if err != nil {
		return GatewaySignedConfig{}, "", err
	}
	config.Signature = Signature{Algorithm: "Ed25519", KeyID: s.keyID, Value: base64.StdEncoding.EncodeToString(ed25519.Sign(s.privateKey, payload))}
	raw, err := json.Marshal(config)
	if err != nil {
		return GatewaySignedConfig{}, "", err
	}
	sum := sha256.Sum256(raw)
	return config, `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func (s *Service) AckEnrollment(ctx context.Context, enrollmentToken string, request SignedConfigAckRequest) (SignedConfigAckResponse, error) {
	enrollment, device, network, err := s.repo.enrollment(ctx, HashSecret(enrollmentToken))
	if err != nil || enrollment.ConsumedAt != nil || !enrollment.ExpiresAt.After(s.now()) {
		return SignedConfigAckResponse{}, ErrInvalidToken
	}
	if request.DeviceID != device.ID || request.ConfigID != configID(network.ID, device.ID, request.Generation) || request.AppliedAt.IsZero() {
		return SignedConfigAckResponse{}, ErrForbidden
	}
	return s.repo.ack(ctx, request, device, network, s.now())
}

func (s *Service) AckUser(ctx context.Context, userID string, request SignedConfigAckRequest) (SignedConfigAckResponse, error) {
	device, network, err := s.repo.userDevice(ctx, userID, request.DeviceID)
	if err != nil {
		return SignedConfigAckResponse{}, err
	}
	if request.ConfigID != configID(network.ID, device.ID, request.Generation) || request.AppliedAt.IsZero() {
		return SignedConfigAckResponse{}, ErrForbidden
	}
	return s.repo.ack(ctx, request, device, network, s.now())
}

func (s *Service) PolicyArtifact(ctx context.Context, enrollmentToken string, generation uint64, digest string) ([]byte, error) {
	enrollment, device, network, err := s.repo.enrollment(ctx, HashSecret(enrollmentToken))
	if err != nil || enrollment.ConsumedAt != nil || !enrollment.ExpiresAt.After(s.now()) {
		return nil, ErrInvalidToken
	}
	if generation != network.ConfigGeneration || device.Status != "active" {
		return nil, ErrForbidden
	}
	policy, err := s.policyArtifact(network)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, ErrNotFound
	}
	return raw, nil
}

func (s *Service) buildSignedConfig(device DeviceRecord, network NetworkRecord, v2 bool) (SignedConfig, string, error) {
	now := canonicalTime(s.now())
	expires := canonicalTime(now.Add(s.signedConfigTTL))
	generation := network.ConfigGeneration
	if generation == 0 {
		generation = 1
	}
	config := SignedConfig{
		SchemaVersion: 1,
		ConfigID:      configID(network.ID, device.ID, generation),
		NetworkID:     network.ID,
		DeviceID:      device.ID,
		Generation:    generation,
		IssuedAt:      now,
		ExpiresAt:     expires,
		ProxyCore:     "xray",
		Transport: Transport{
			Kind: "vless-tls-xudp", Loopback: Endpoint{Host: "127.0.0.1", Port: 18080},
			Remote: RemoteEndpoint{Host: network.GatewayEndpointHost, Port: network.TransportPort, ServerName: network.TransportServerName}, AuthID: network.TransportAuthID,
		},
		WireGuard: WireGuard{
			InterfaceName: "xconone0", Addresses: []string{device.WireGuardAddress}, MTU: 1420,
			Peers: []WireGuardPeer{{GatewayID: network.GatewayID, PublicKey: network.GatewayWireGuardKey, AllowedIPs: []string{network.CIDR}, Endpoint: Endpoint{Host: "127.0.0.1", Port: 18080}, PersistentKeepaliveSeconds: 25}},
		},
	}
	if v2 {
		policy, err := s.policyArtifact(network)
		if err != nil {
			return SignedConfig{}, "", err
		}
		raw, err := json.Marshal(policy)
		if err != nil {
			return SignedConfig{}, "", err
		}
		sum := sha256.Sum256(raw)
		digest := hex.EncodeToString(sum[:])
		config.SchemaVersion = 2
		config.Policy = &PolicyReference{Generation: generation, Digest: digest, Path: fmt.Sprintf("/api/overlay/v1/enrollment/policy-artifacts/%d/%s", generation, digest), MediaType: PolicyMediaType}
	}
	payload, err := signingBytes(config)
	if err != nil {
		return SignedConfig{}, "", err
	}
	signature := ed25519.Sign(s.privateKey, payload)
	config.Signature = Signature{Algorithm: "Ed25519", KeyID: s.keyID, Value: base64.StdEncoding.EncodeToString(signature)}
	final, err := json.Marshal(config)
	if err != nil {
		return SignedConfig{}, "", err
	}
	etagSum := sha256.Sum256(final)
	return config, `"` + hex.EncodeToString(etagSum[:]) + `"`, nil
}

func defaultPolicyArtifact(networkID string, generation uint64) PolicyArtifact {
	return PolicyArtifact{SchemaVersion: 1, CompilerVersion: PolicyCompilerVersion, NetworkID: networkID, Revision: generation, DefaultAction: "deny", ProtectedFlows: []string{"control:controller-session", "control:gateway-apply-result", "control:gateway-heartbeat", "control:gateway-policy-artifact", "control:gateway-snapshot"}, Rules: []PolicyRule{}}
}

func (s *Service) policyArtifact(network NetworkRecord) (PolicyArtifact, error) {
	if strings.TrimSpace(network.PolicyJSON) == "" {
		return defaultPolicyArtifact(network.ID, network.ConfigGeneration), nil
	}
	var policy PolicyArtifact
	if err := json.Unmarshal([]byte(network.PolicyJSON), &policy); err != nil {
		return PolicyArtifact{}, fmt.Errorf("decode stored overlay policy: %w", err)
	}
	if policy.NetworkID != network.ID || policy.Revision != network.ConfigGeneration {
		return PolicyArtifact{}, ErrGenerationConflict
	}
	return policy, nil
}

func signingBytes(config SignedConfig) ([]byte, error) {
	unsigned := struct {
		SchemaVersion int              `json:"schema_version"`
		ConfigID      string           `json:"config_id"`
		NetworkID     string           `json:"network_id"`
		DeviceID      string           `json:"device_id"`
		Generation    uint64           `json:"generation"`
		IssuedAt      time.Time        `json:"issued_at"`
		ExpiresAt     time.Time        `json:"expires_at"`
		ProxyCore     string           `json:"proxy_core"`
		Transport     Transport        `json:"transport"`
		WireGuard     WireGuard        `json:"wireguard"`
		Policy        *PolicyReference `json:"policy"`
	}{config.SchemaVersion, config.ConfigID, config.NetworkID, config.DeviceID, config.Generation, config.IssuedAt, config.ExpiresAt, config.ProxyCore, config.Transport, config.WireGuard, config.Policy}
	if config.SchemaVersion == 1 {
		return json.Marshal(struct {
			SchemaVersion int       `json:"schema_version"`
			ConfigID      string    `json:"config_id"`
			NetworkID     string    `json:"network_id"`
			DeviceID      string    `json:"device_id"`
			Generation    uint64    `json:"generation"`
			IssuedAt      time.Time `json:"issued_at"`
			ExpiresAt     time.Time `json:"expires_at"`
			ProxyCore     string    `json:"proxy_core"`
			Transport     Transport `json:"transport"`
			WireGuard     WireGuard `json:"wireguard"`
		}{config.SchemaVersion, config.ConfigID, config.NetworkID, config.DeviceID, config.Generation, config.IssuedAt, config.ExpiresAt, config.ProxyCore, config.Transport, config.WireGuard})
	}
	return json.Marshal(unsigned)
}

func gatewaySigningBytes(config GatewaySignedConfig) ([]byte, error) {
	return json.Marshal(struct {
		SchemaVersion int              `json:"schema_version"`
		Role          string           `json:"role"`
		ConfigID      string           `json:"config_id"`
		NetworkID     string           `json:"network_id"`
		GatewayID     string           `json:"gateway_id"`
		Generation    uint64           `json:"generation"`
		IssuedAt      time.Time        `json:"issued_at"`
		ExpiresAt     time.Time        `json:"expires_at"`
		InterfaceName string           `json:"interface_name"`
		Address       string           `json:"address"`
		ListenPort    int              `json:"listen_port"`
		MTU           int              `json:"mtu"`
		Peers         []GatewayPeer    `json:"peers"`
		Transport     GatewayTransport `json:"transport"`
	}{config.SchemaVersion, config.Role, config.ConfigID, config.NetworkID, config.GatewayID, config.Generation, config.IssuedAt, config.ExpiresAt, config.InterfaceName, config.Address, config.ListenPort, config.MTU, config.Peers, config.Transport})
}

func validateExchange(request ExchangeRequest) error {
	role := request.Role
	if role == "" {
		role = RoleOne
	}
	publicKey, keyErr := base64.StdEncoding.DecodeString(request.WireGuardPublicKey)
	if !validOpaque(request.JoinToken, "xjt_") || !overlayDeviceIDPattern.MatchString(request.DeviceID) || strings.TrimSpace(request.Platform) == "" || keyErr != nil || len(publicKey) != 32 || len(request.Name) > 255 || len(request.Hostname) > 255 {
		return ErrInvalidInput
	}
	switch request.Platform {
	case "linux", "darwin", "windows", "ios", "android":
	default:
		return ErrInvalidInput
	}
	if role != RoleOne && role != RoleGateway {
		return ErrInvalidInput
	}
	return nil
}

func validOpaque(value, prefix string) bool {
	if value != strings.TrimSpace(value) || !strings.HasPrefix(value, prefix) {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil && len(raw) == 32
}

func canonicalTime(value time.Time) time.Time { return value.UTC().Truncate(time.Second) }
func (s *Service) now() time.Time             { return canonicalTime(s.clock()) }

func newOpaqueToken(prefix string) string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf)
}

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func newID(prefix string) string { return prefix + "_" + strings.ReplaceAll(uuidNewString(), "-", "") }
func uuidNewString() string      { return fmt.Sprintf("%s", randomUUID()) }
func randomUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", buf[:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}

func configID(networkID, deviceID string, generation uint64) string {
	sum := sha256.Sum256([]byte(networkID + "\x00" + deviceID + "\x00" + fmt.Sprint(generation)))
	return "cfg_" + hex.EncodeToString(sum[:])[:32]
}

func toDevice(record DeviceRecord) Device {
	role := ""
	if record.Role == RoleGateway {
		role = RoleGateway
	}
	return Device{ID: record.ID, UserID: record.UserID, NetworkID: record.NetworkID, Role: role, Name: record.Name, Platform: record.Platform, Hostname: record.Hostname, WireGuardPublicKey: record.WireGuardPublicKey, WireGuardAddress: record.WireGuardAddress, CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC(), LastSeenAt: record.LastSeenAt}
}

func toAdminDevice(record DeviceRecord, ackReceivedAt time.Time, acknowledged bool, now time.Time) AdminDevice {
	connectionStatus := "never_seen"
	if record.Status == "revoked" {
		connectionStatus = "revoked"
	} else if acknowledged {
		connectionStatus = "stale"
		lastACKAt := ackReceivedAt
		if record.LastSeenAt != nil && record.LastSeenAt.After(lastACKAt) {
			lastACKAt = *record.LastSeenAt
		}
		if !lastACKAt.After(now) && !lastACKAt.Before(now.Add(-adminDeviceACKRecentWindow)) {
			connectionStatus = "recent_ack"
		}
	}
	return AdminDevice{ID: record.ID, UserID: record.UserID, NetworkID: record.NetworkID, Role: record.Role, Name: record.Name, Platform: record.Platform, Hostname: record.Hostname, WireGuardPublicKey: record.WireGuardPublicKey, WireGuardAddress: record.WireGuardAddress, Status: record.Status, LastSeenAt: record.LastSeenAt, ConnectionStatus: connectionStatus, CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
}

func toNetwork(record NetworkRecord) Network {
	return Network{ID: record.ID, DisplayName: record.DisplayName, CIDR: record.CIDR, GatewayID: record.GatewayID, GatewayWireGuardKey: record.GatewayWireGuardKey, GatewayEndpointHost: record.GatewayEndpointHost, GatewayEndpointPort: record.GatewayEndpointPort, TransportServerName: record.TransportServerName, TransportPort: record.TransportPort, TransportAuthID: record.TransportAuthID, ConfigGeneration: record.ConfigGeneration, CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
}
