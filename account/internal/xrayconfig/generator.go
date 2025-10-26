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

// Client represents an entry under outbounds[].settings.vnext[].users[] in the Xray
// config.
type Client struct {
	ID    string
	Email string
	Flow  string
}

// Generator updates the Xray configuration file based on a template and a set of
// active clients.
type Generator struct {
	// TemplatePath is the path to the base configuration template. The
	// template must contain an "outbounds" array with objects that expose a
	// "settings" object.
	TemplatePath string

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
	if strings.TrimSpace(g.TemplatePath) == "" {
		return errors.New("template path is required")
	}
	if strings.TrimSpace(g.OutputPath) == "" {
		return errors.New("output path is required")
	}

	rawTemplate, err := os.ReadFile(g.TemplatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(rawTemplate, &root); err != nil {
		return fmt.Errorf("decode template json: %w", err)
	}

	if err := replaceClients(root, clients); err != nil {
		return err
	}

	buf, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	buf = append(buf, '\n')

	mode := g.FileMode
	if mode == 0 {
		mode = 0o644
	}
	if err := atomicWriteFile(g.OutputPath, buf, mode); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func replaceClients(root map[string]interface{}, clients []Client) error {
	outboundsValue, ok := root["outbounds"]
	if !ok {
		return errors.New("template missing outbounds array")
	}

	outboundsSlice, ok := outboundsValue.([]interface{})
	if !ok {
		return fmt.Errorf("template outbounds has unexpected type %T", outboundsValue)
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

	var updated bool
	for idx, outbound := range outboundsSlice {
		outboundMap, ok := outbound.(map[string]interface{})
		if !ok {
			return fmt.Errorf("template outbound %d has unexpected type %T", idx, outbound)
		}

		settingsValue, ok := outboundMap["settings"]
		if !ok {
			continue
		}

		settingsMap, ok := settingsValue.(map[string]interface{})
		if !ok {
			return fmt.Errorf("template outbound %d settings has unexpected type %T", idx, settingsValue)
		}

		vnextValue, ok := settingsMap["vnext"]
		if !ok {
			continue
		}

		vnextSlice, ok := vnextValue.([]interface{})
		if !ok {
			return fmt.Errorf("template outbound %d vnext has unexpected type %T", idx, vnextValue)
		}

		for vnextIdx, vnext := range vnextSlice {
			vnextMap, ok := vnext.(map[string]interface{})
			if !ok {
				return fmt.Errorf("template outbound %d vnext %d has unexpected type %T", idx, vnextIdx, vnext)
			}

			// Always replace the users array so the config reflects the exact
			// state from the database.
			vnextMap["users"] = clientObjects
			vnextSlice[vnextIdx] = vnextMap
			updated = true
		}

		settingsMap["vnext"] = vnextSlice
		outboundMap["settings"] = settingsMap
		outboundsSlice[idx] = outboundMap
	}

	if !updated {
		return errors.New("template missing vnext users array")
	}

	root["outbounds"] = outboundsSlice
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
