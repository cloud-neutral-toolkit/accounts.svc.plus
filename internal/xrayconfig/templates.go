package xrayconfig

import _ "embed"

var (
	//go:embed template_server.json
	serverTemplateJSON []byte
)

// DefaultDefinition returns the built-in Xray configuration definition used when
// no explicit definition is provided. The template is embedded in the binary so
// that configuration rendering no longer depends on filesystem state at
// runtime.
func DefaultDefinition() Definition {
	return JSONDefinition{Raw: append([]byte(nil), serverTemplateJSON...)}
}
