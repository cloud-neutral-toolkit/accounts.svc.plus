package api

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"account/internal/auth"
	"account/internal/store"
	"account/internal/xrayconfig"
)

const (
	defaultXHTTPPath = "/split"
	defaultXHTTPMode = "auto"
	defaultXHTTPPort = 443
	defaultTCPPort   = 1443
	defaultTLSFP     = "chrome"
	defaultTCPFlow   = "xtls-rprx-vision"
)

type vlessNode struct {
	Name           string   `json:"name"`
	Address        string   `json:"address"`
	Port           int      `json:"port,omitempty"`
	Users          []string `json:"users,omitempty"`
	Transport      string   `json:"transport,omitempty"`
	Path           string   `json:"path,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	Security       string   `json:"security,omitempty"`
	Flow           string   `json:"flow,omitempty"`
	ServerName     string   `json:"server_name,omitempty"`
	XHTTPPort      int      `json:"xhttp_port,omitempty"`
	TCPPort        int      `json:"tcp_port,omitempty"`
	URISchemeXHTTP string   `json:"uri_scheme_xhttp,omitempty"`
	URISchemeTCP   string   `json:"uri_scheme_tcp,omitempty"`
}

func (h *handler) listAgentNodes(c *gin.Context) {
	// Get current user ID to use as VLESS UUID
	userID := auth.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "account_paused",
			"message": "account is paused",
		})
		return
	}

	proxyUUID := strings.TrimSpace(user.ProxyUUID)
	if proxyUUID == "" {
		proxyUUID = user.ID
	}

	if user.ProxyUUIDExpiresAt != nil && time.Now().UTC().After(*user.ProxyUUIDExpiresAt) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "proxy_uuid_expired",
			"message": "proxy access has expired, please renew",
		})
		return
	}

	hosts := parseProxyNodeHosts(h.publicURL)
	if len(hosts) == 0 {
		c.JSON(http.StatusOK, []vlessNode{})
		return
	}

	xhttpPath := envOrDefault("XRAY_XHTTP_PATH", defaultXHTTPPath)
	xhttpMode := envOrDefault("XRAY_XHTTP_MODE", defaultXHTTPMode)
	xhttpPort := envIntOrDefault("XRAY_XHTTP_PORT", defaultXHTTPPort)
	tcpPort := envIntOrDefault("XRAY_TCP_PORT", defaultTCPPort)

	xhttpScheme := xrayconfig.VLESSXHTTPScheme()
	tcpScheme := xrayconfig.VLESSTCPScheme()

	users := []string{proxyUUID}
	nodes := make([]vlessNode, 0, len(hosts))
	for _, host := range hosts {
		nodeName := nodeNameForHost(host)
		nodes = append(nodes, vlessNode{
			Name:       nodeName,
			Address:    host,
			Port:       xhttpPort,
			Users:      users,
			Transport:  "xhttp",
			Path:       xhttpPath,
			Mode:       xhttpMode,
			Security:   "tls",
			Flow:       defaultTCPFlow,
			ServerName: host,
			XHTTPPort:  xhttpPort,
			TCPPort:    tcpPort,
			URISchemeXHTTP: renderVLESSURIScheme(xhttpScheme, map[string]string{
				"UUID":   proxyUUID,
				"DOMAIN": host,
				"NODE":   host,
				"PATH":   url.QueryEscape(xhttpPath),
				"MODE":   url.QueryEscape(xhttpMode),
				"SNI":    host,
				"FP":     defaultTLSFP,
				"TAG":    url.QueryEscape(nodeName),
			}),
			URISchemeTCP: renderVLESSURIScheme(tcpScheme, map[string]string{
				"UUID":   proxyUUID,
				"DOMAIN": host,
				"NODE":   host,
				"SNI":    host,
				"FP":     defaultTLSFP,
				"FLOW":   defaultTCPFlow,
				"TAG":    url.QueryEscape(nodeName),
			}),
		})
	}

	c.JSON(http.StatusOK, nodes)
}

func parseProxyNodeHosts(publicURL string) []string {
	seen := make(map[string]struct{})
	hosts := make([]string, 0)

	appendHost := func(raw string) {
		host := normalizeHost(raw)
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}

	if raw := strings.TrimSpace(os.Getenv("XRAY_PROXY_NODES")); raw != "" {
		fields := strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
		})
		for _, field := range fields {
			appendHost(field)
		}
	}

	if len(hosts) == 0 {
		appendHost(publicURL)
	}

	return hosts
}

func normalizeHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if strings.Contains(value, "://") {
		u, err := url.Parse(value)
		if err == nil {
			value = u.Hostname()
		}
	}

	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	if strings.Contains(value, "/") {
		parts := strings.SplitN(value, "/", 2)
		value = parts[0]
	}
	if strings.Contains(value, ":") {
		host, _, found := strings.Cut(value, ":")
		if found {
			value = host
		}
	}

	return strings.TrimSpace(value)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func nodeNameForHost(host string) string {
	prefix := host
	if idx := strings.Index(prefix, "."); idx > 0 {
		prefix = prefix[:idx]
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = host
	}
	prefix = strings.ReplaceAll(prefix, "_", "-")
	prefix = strings.ToUpper(prefix)
	return prefix + "-NODE"
}

func renderVLESSURIScheme(tpl string, values map[string]string) string {
	rendered := strings.TrimSpace(tpl)
	if rendered == "" {
		return ""
	}

	for key, value := range values {
		rendered = strings.ReplaceAll(rendered, "${"+key+"}", value)
	}
	return rendered
}
