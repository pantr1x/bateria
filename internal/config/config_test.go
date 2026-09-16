package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "niet.json"))
	if got != Default() {
		t.Errorf("Load() = %+v, chcem predvolené %+v", got, Default())
	}
}

func TestLoadBrokenFileGivesDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte("{toto nie je json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); got != Default() {
		t.Errorf("Load() = %+v, chcem predvolené", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "podpriecinok", "config.json")
	want := Config{IconMode: IconPercent, RefreshSeconds: 5}
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); got != want {
		t.Errorf("Load() = %+v, chcem %+v", got, want)
	}
}

// Nezmyselné hodnoty zo súboru sa nesmú dostať do behu aplikácie.
func TestLoadRejectsOutOfRange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"icon_mode":"hviezda","refresh_seconds":9999}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(p)
	if got.IconMode != IconBattery {
		t.Errorf("neznámy režim ikony sa mal zahodiť, je %q", got.IconMode)
	}
	if got.RefreshSeconds != Default().RefreshSeconds {
		t.Errorf("neplatná perióda sa mala zahodiť, je %d", got.RefreshSeconds)
	}
}
