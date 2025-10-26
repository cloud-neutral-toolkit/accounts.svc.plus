package xrayconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testConfig struct {
	Outbounds []struct {
		Settings struct {
			VNext []struct {
				Users []struct {
					ID         string `json:"id"`
					Email      string `json:"email,omitempty"`
					Flow       string `json:"flow,omitempty"`
					Encryption string `json:"encryption"`
				} `json:"users"`
				Address string `json:"address"`
			} `json:"vnext"`
		} `json:"settings"`
	} `json:"outbounds"`
}

func TestGeneratorGenerate(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	outputPath := filepath.Join(dir, "config.json")

	template := `{
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "legacy.example",
            "users": [
              {"id": "old", "email": "old@example"}
            ]
          }
        ]
      }
    },
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
		{ID: "uuid-a", Email: "a@demo", Flow: "xtls-rprx-vision", Encryption: "custom"},
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

	if len(cfg.Outbounds) == 0 {
		t.Fatalf("expected at least 1 outbound, got %d", len(cfg.Outbounds))
	}

	vnext := cfg.Outbounds[0].Settings.VNext
	if len(vnext) == 0 {
		t.Fatalf("expected vnext entries, got %d", len(vnext))
	}

	gotUsers := vnext[0].Users
	if len(gotUsers) != len(clients) {
		t.Fatalf("expected %d users, got %d", len(clients), len(gotUsers))
	}
	if gotUsers[0].ID != "uuid-a" || gotUsers[0].Email != "a@demo" {
		t.Fatalf("unexpected first user: %+v", gotUsers[0])
	}
	if gotUsers[0].Flow != "xtls-rprx-vision" {
		t.Fatalf("unexpected first user flow: %+v", gotUsers[0])
	}
	if gotUsers[0].Encryption != "custom" {
		t.Fatalf("unexpected first user encryption: %+v", gotUsers[0])
	}
	if gotUsers[1].ID != "uuid-b" || gotUsers[1].Email != "" || gotUsers[1].Flow != DefaultFlow || gotUsers[1].Encryption != DefaultEncryption {
		t.Fatalf("unexpected second user: %+v", gotUsers[1])
	}
}

func TestGeneratorGenerateMissingID(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	outputPath := filepath.Join(dir, "config.json")

	template := `{"outbounds":[{"settings":{"vnext":[{"users":[]}]}]}]}`
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
