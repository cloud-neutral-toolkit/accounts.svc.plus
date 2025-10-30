package xrayconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultFlow is applied to VLESS clients when no explicit flow is
	// provided. It matches the tlsSettings baked into the template.
	DefaultFlow = "xtls-rprx-vision"
)

// Client represents an entry under inbounds[0].settings.clients[] in the Xray
// config.
type Client struct {
	ID    string
	Email string
	Flow  string
}

// Generator updates the Xray configuration file based on a template and a set of
// active clients.
type Generator struct {
	// Definition provides the base Xray configuration that will be mutated to
	// include the provided clients. When nil, DefaultDefinition is used.
	Definition Definition

	// OutputPath is the destination path for the generated configuration
	// (typically /usr/local/etc/xray/config.json).
	OutputPath string

	// FileMode controls the permissions for the generated file. When zero it
	// defaults to 0644.
	FileMode fs.FileMode
}

// Generate writes a new Xray configuration with the provided clients. The base
// template is loaded on every invocation to ensure updates remain additive and
// idempotent even when multiple callers trigger regeneration.
func (g Generator) Generate(clients []Client) error {
	if strings.TrimSpace(g.OutputPath) == "" {
		return errors.New("output path is required")
	}

	buf, err := g.Render(clients)
	if err != nil {
		return err
	}

	mode := g.FileMode
	if mode == 0 {
		mode = 0o644
	}
	if err := atomicWriteFile(g.OutputPath, buf, mode); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// Render returns the rendered configuration JSON without writing it to disk.
// The returned buffer always ends with a newline.
func (g Generator) Render(clients []Client) ([]byte, error) {
	definition := g.Definition
	if definition == nil {
		definition = DefaultDefinition()
	}

	root, err := definition.Base()
	if err != nil {
		return nil, fmt.Errorf("load template: %w", err)
	}

	if err := replaceClients(root, clients); err != nil {
		return nil, err
	}

	buf, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	buf = append(buf, '\n')
	return buf, nil
}

func replaceClients(root map[string]interface{}, clients []Client) error {
	inboundsValue, ok := root["inbounds"]
	if !ok {
		return errors.New("template missing inbounds array")
	}

	inboundsSlice, ok := inboundsValue.([]interface{})
	if !ok {
		return fmt.Errorf("template inbounds has unexpected type %T", inboundsValue)
	}
	if len(inboundsSlice) == 0 {
		return errors.New("template missing inbound entry")
	}

	clientObjects := make([]interface{}, 0, len(clients))
	for idx, client := range clients {
		id := strings.TrimSpace(client.ID)
		if id == "" {
			return fmt.Errorf("client %d missing id", idx)
		}
		entry := map[string]interface{}{
			"id": id,
		}
		if email := strings.TrimSpace(client.Email); email != "" {
			entry["email"] = email
		}
		flow := strings.TrimSpace(client.Flow)
		if flow == "" {
			flow = DefaultFlow
		}
		entry["flow"] = flow
		clientObjects = append(clientObjects, entry)
	}

	inbound, ok := inboundsSlice[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("template inbound 0 has unexpected type %T", inboundsSlice[0])
	}

	settingsValue, ok := inbound["settings"]
	if !ok {
		return errors.New("template inbound missing settings object")
	}

	settingsMap, ok := settingsValue.(map[string]interface{})
	if !ok {
		return fmt.Errorf("template inbound settings has unexpected type %T", settingsValue)
	}

	clientsValue, ok := settingsMap["clients"]
	if !ok {
		return errors.New("template inbound missing clients array")
	}

	if _, ok := clientsValue.([]interface{}); !ok {
		return fmt.Errorf("template inbound clients has unexpected type %T", clientsValue)
	}

	// Always replace the clients array so the config reflects the exact
	// state from the database.
	settingsMap["clients"] = clientObjects
	inbound["settings"] = settingsMap
	inboundsSlice[0] = inbound
	root["inbounds"] = inboundsSlice
	return nil
}

func atomicWriteFile(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".xray-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
