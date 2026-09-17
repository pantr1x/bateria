// Package config načítava a ukladá nastavenia aplikácie.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Režimy zobrazenia ikony.
const (
	// IconSystem je ikona, akú kreslí sám Windows (znak zo systémového písma).
	IconSystem = "system"
	// IconBattery je vlastná kreslená ikona batérie.
	IconBattery = "battery"
	// IconPercent je číslo v percentách.
	IconPercent = "percent"
	// IconTime je zostávajúci čas ako text priamo v paneli („2:13“).
	IconTime = "time"
)

// Config sú nastavenia uložené vo formáte JSON.
type Config struct {
	// IconMode je "battery" alebo "percent".
	IconMode string `json:"icon_mode"`
	// RefreshSeconds je perióda merania v sekundách.
	RefreshSeconds int `json:"refresh_seconds"`
	// DrainPerHour je zapamätaná rýchlosť vybíjania z posledného behu na
	// batérii. Vďaka nej vie aplikácia hneď po štarte povedať, ako dlho by
	// počítač vydržal po odpojení – inak by sa to učila odznova.
	DrainPerHour      float64 `json:"drain_per_hour"`
	DrainUsesCapacity bool    `json:"drain_uses_capacity"`
	// ClockTextDisabled vypína vloženie času priamo do hodín v paneli úloh
	// (cez bateria-hook.dll). Je to záporná voľba, aby staršie súbory s
	// nastaveniami mali text zapnutý.
	ClockTextDisabled bool `json:"clock_text_disabled"`
	// TrayPromoted si pamätá, že sme ikonu už raz vytiahli z prepadovej
	// ponuky do panela úloh. Druhý raz to aplikácia nerobí – keby si ju
	// používateľ medzitým schoval, nemá mu to prepisovať späť.
	TrayPromoted bool `json:"tray_promoted"`
}

// Default vráti predvolené nastavenia.
func Default() Config {
	return Config{IconMode: IconTime, RefreshSeconds: 2}
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
	switch loaded.IconMode {
	case IconSystem, IconBattery, IconPercent, IconTime:
		c.IconMode = loaded.IconMode
	}
	if loaded.RefreshSeconds >= 1 && loaded.RefreshSeconds <= 60 {
		c.RefreshSeconds = loaded.RefreshSeconds
	}
	c.TrayPromoted = loaded.TrayPromoted
	c.ClockTextDisabled = loaded.ClockTextDisabled
	if loaded.DrainPerHour > 0 {
		c.DrainPerHour = loaded.DrainPerHour
		c.DrainUsesCapacity = loaded.DrainUsesCapacity
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
