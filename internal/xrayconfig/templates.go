package xrayconfig

import _ "embed"

var (
	//go:embed template_server.json
	serverTemplateJSON []byte

	//go:embed template_tcp.json
	tcpTemplateJSON []byte

	//go:embed template_xhttp.json
	xhttpTemplateJSON []byte

	//go:embed VLESS-TCP-URI.Scheme
	vlessTCPScheme []byte

	//go:embed VLESS-XHTTP-URI.Scheme
	vlessXHTTPScheme []byte
)

// DefaultDefinition returns the built-in Xray configuration definition used when
// no explicit definition is provided. The template is embedded in the binary so
// that configuration rendering no longer depends on filesystem state at
// runtime.
func DefaultDefinition() Definition {
	return TCPDefinition()
}

// TCPDefinition returns the Xray configuration for TCP transport.
func TCPDefinition() Definition {
	return JSONDefinition{Raw: append([]byte(nil), tcpTemplateJSON...)}
}

// XHTTPDefinition returns the Xray configuration for XHTTP transport.
func XHTTPDefinition() Definition {
	return JSONDefinition{Raw: append([]byte(nil), xhttpTemplateJSON...)}
}

// VLESSTCPScheme returns the embedded VLESS URI template for TCP transport.
func VLESSTCPScheme() string {
	return string(append([]byte(nil), vlessTCPScheme...))
}

// VLESSXHTTPScheme returns the embedded VLESS URI template for XHTTP transport.
func VLESSXHTTPScheme() string {
	return string(append([]byte(nil), vlessXHTTPScheme...))
}
