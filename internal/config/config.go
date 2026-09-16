// Package config načítava a ukladá nastavenia aplikácie.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Režimy zobrazenia ikony.
const (
	IconBattery = "battery"
	IconPercent = "percent"
)

// Config sú nastavenia uložené vo formáte JSON.
type Config struct {
	// IconMode je "battery" alebo "percent".
	IconMode string `json:"icon_mode"`
	// RefreshSeconds je perióda merania v sekundách.
	RefreshSeconds int `json:"refresh_seconds"`
}

// Default vráti predvolené nastavenia.
func Default() Config {
	return Config{IconMode: IconBattery, RefreshSeconds: 2}
}

// Path vráti cestu k súboru s nastaveniami (%APPDATA%\Bateria\config.json).
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Bateria", "config.json"), nil
}

// Load načíta nastavenia. Ak súbor neexistuje alebo je poškodený, vráti
// predvolené hodnoty – aplikácia sa kvôli nastaveniam nikdy nesmie zastaviť.
func Load(path string) Config {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return c
	}
	if loaded.IconMode == IconBattery || loaded.IconMode == IconPercent {
		c.IconMode = loaded.IconMode
	}
	if loaded.RefreshSeconds >= 1 && loaded.RefreshSeconds <= 60 {
		c.RefreshSeconds = loaded.RefreshSeconds
	}
	return c
}

// Save uloží nastavenia a podľa potreby vytvorí priečinok.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
