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

	gen := Generator{
		Definition: JSONDefinition{Raw: []byte(template)},
		OutputPath: outputPath,
	}

	clients := []Client{
		{ID: "uuid-a", Email: "a@demo", Flow: "xtls-rprx-vision"},
		{ID: "uuid-b"},
	}

	if err := gen.Generate(clients); err != nil {
		t.Fatalf("generate: %v", err)
	}

	var cfg testConfig
	raw, err := gen.Render(clients)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
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

	onDisk, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(onDisk) != string(raw) {
		t.Fatalf("written config does not match rendered output")
	}
}

func TestGeneratorGenerateMissingID(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "config.json")
	template := `{"inbounds":[{"settings":{"clients":[]}}]}`

	gen := Generator{
		Definition: JSONDefinition{Raw: []byte(template)},
		OutputPath: outputPath,
	}

	err := gen.Generate([]Client{{Email: "missing@id"}})
	if err == nil || !strings.Contains(err.Error(), "missing id") {
		t.Fatalf("expected missing id error, got %v", err)
	}
}

func TestGeneratorRenderUsesDefaultDefinition(t *testing.T) {
	gen := Generator{}

	data, err := gen.Render(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected data from default definition")
	}
	if data[len(data)-1] != '\n' {
		t.Fatal("expected render output to end with newline")
	}
}
