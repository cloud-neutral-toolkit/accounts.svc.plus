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
      "settings": {
        "clients": [
          {"id": "old", "email": "old@example"}
        ]
      }
    }
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

	if len(cfg.Inbounds) == 0 {
		t.Fatalf("expected at least 1 inbound, got %d", len(cfg.Inbounds))
	}

	clientsSection := cfg.Inbounds[0].Settings.Clients
	if len(clientsSection) == 0 {
		t.Fatalf("expected client entries, got %d", len(clientsSection))
	}

	if len(clientsSection) != len(clients) {
		t.Fatalf("expected %d clients, got %d", len(clients), len(clientsSection))
	}
	if clientsSection[0].ID != "uuid-a" || clientsSection[0].Email != "a@demo" {
		t.Fatalf("unexpected first client: %+v", clientsSection[0])
	}
	if clientsSection[0].Flow != "xtls-rprx-vision" {
		t.Fatalf("unexpected first client flow: %+v", clientsSection[0])
	}
	if clientsSection[1].ID != "uuid-b" || clientsSection[1].Email != "" || clientsSection[1].Flow != DefaultFlow {
		t.Fatalf("unexpected second client: %+v", clientsSection[1])
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
