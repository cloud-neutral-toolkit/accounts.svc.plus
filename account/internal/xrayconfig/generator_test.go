package xrayconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testConfig struct {
	Inbounds []struct {
		Settings struct {
			Clients []struct {
				ID    string `json:"id"`
				Email string `json:"email,omitempty"`
				Flow  string `json:"flow,omitempty"`
			} `json:"clients"`
			Decryption string `json:"decryption"`
		} `json:"settings"`
	} `json:"inbounds"`
}

func TestGeneratorGenerate(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	outputPath := filepath.Join(dir, "config.json")

	template := `{
  "inbounds": [
    {
      "tag": "vless",
      "settings": {
        "clients": [
          {"id": "old", "email": "old@example"}
        ],
        "decryption": "none"
      }
    }
  ],
  "outbounds": [
    {"protocol": "freedom"}
  ]
}`

	if err := os.WriteFile(templatePath, []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	gen := Generator{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
	}

	clients := []Client{
		{ID: "uuid-a", Email: "a@demo", Flow: "xtls-rprx-vision"},
		{ID: "uuid-b"},
	}

	if err := gen.Generate(clients); err != nil {
		t.Fatalf("generate: %v", err)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var cfg testConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	if len(cfg.Inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %d", len(cfg.Inbounds))
	}

	gotClients := cfg.Inbounds[0].Settings.Clients
	if len(gotClients) != len(clients) {
		t.Fatalf("expected %d clients, got %d", len(clients), len(gotClients))
	}

	if gotClients[0].ID != "uuid-a" || gotClients[0].Email != "a@demo" {
		t.Fatalf("unexpected first client: %+v", gotClients[0])
	}
	if gotClients[0].Flow != "xtls-rprx-vision" {
		t.Fatalf("unexpected first client flow: %+v", gotClients[0])
	}
	if gotClients[1].ID != "uuid-b" || gotClients[1].Email != "" || gotClients[1].Flow != DefaultFlow {
		t.Fatalf("unexpected second client: %+v", gotClients[1])
	}

	if cfg.Inbounds[0].Settings.Decryption != "none" {
		t.Fatalf("decryption field was modified: %q", cfg.Inbounds[0].Settings.Decryption)
	}
}

func TestGeneratorGenerateMissingID(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	outputPath := filepath.Join(dir, "config.json")

	template := `{"inbounds":[{"settings":{"clients":[]}}]}`
	if err := os.WriteFile(templatePath, []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	gen := Generator{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
	}

	err := gen.Generate([]Client{{Email: "missing@id"}})
	if err == nil || !strings.Contains(err.Error(), "missing id") {
		t.Fatalf("expected missing id error, got %v", err)
	}
}
