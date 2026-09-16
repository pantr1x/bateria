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
	want := Config{IconMode: IconPercent, RefreshSeconds: 5, PanelOffset: 240}
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); got != want {
		t.Errorf("Load() = %+v, chcem %+v", got, want)
	}
}

// Nezmyselné hodnoty zo súboru sa nesmú dostať do behu aplikácie.
// Text v paneli je zapnutý, kým ho niekto výslovne nevypne – aj v starých
// súboroch s nastaveniami, ktoré o ňom nič nevedia.
func TestPanelEnabledByDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"icon_mode":"time"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(p)
	if got.PanelDisabled {
		t.Error("text v paneli mal zostať zapnutý")
	}
	if got.PanelOffset != Default().PanelOffset {
		t.Errorf("odsadenie = %d, chcem predvolené %d", got.PanelOffset, Default().PanelOffset)
	}
}

func TestLoadRejectsOutOfRange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"icon_mode":"hviezda","refresh_seconds":9999,"panel_offset":99999}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load(p)
	if got.IconMode != Default().IconMode {
		t.Errorf("neznámy režim ikony sa mal zahodiť, je %q", got.IconMode)
	}
	if got.RefreshSeconds != Default().RefreshSeconds {
		t.Errorf("neplatná perióda sa mala zahodiť, je %d", got.RefreshSeconds)
	}
	if got.PanelOffset != Default().PanelOffset {
		t.Errorf("neplatné odsadenie sa malo zahodiť, je %d", got.PanelOffset)
	}
}
