package winui

import (
	"testing"
	"time"

	"github.com/pantr1x/bateria/internal/battery"
)

func TestThousands(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 999: "999", 1000: "1 000",
		51000: "51 000", 1234567: "1 234 567", -4200: "-4 200"}
	for in, want := range cases {
		if got := Thousands(in); got != want {
			t.Errorf("Thousands(%d) = %q, chcem %q", in, got, want)
		}
	}
}

func TestHeadlineCharging(t *testing.T) {
	s := battery.Status{Present: true, OnAC: true, State: battery.StateCharging,
		Percent: 73.4, RateSmoothed: 24500, RateSmoothedKnown: true}
	p, sub := Headline(s)
	if p != "73 %" {
		t.Errorf("percentá = %q, chcem \"73 %%\"", p)
	}
	if sub != "Nabíja sa · príkon 24,5 W" {
		t.Errorf("popis = %q", sub)
	}
}

func TestHeadlineNoBattery(t *testing.T) {
	p, sub := Headline(battery.Status{})
	if p != "—" || sub != "Batéria sa nenašla" {
		t.Errorf("bez batérie: %q / %q", p, sub)
	}
}

func TestDetailsCharging(t *testing.T) {
	s := battery.Status{
		Present: true, OnAC: true, State: battery.StateCharging, Percent: 73,
		Capacity: 37200, FullCapacity: 51000, DesignCapacity: 56000,
		CycleCount: 128, Voltage: 11460,
		TimeToFull: time.Hour + 12*time.Minute, Estimate: battery.SrcDriver,
	}
	rows := Details(s)
	want := map[string]string{
		"Do plného nabitia": "1 h 12 min",
		"Kapacita":          "37 200 / 51 000 mWh",
		"Zdravie batérie":   "91 %",
		"Počet cyklov":      "128",
		"Napätie":           "11,46 V",
		"Zdroj odhadu":      "ovládač batérie",
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.Label] = r.Value
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("riadok %q = %q, chcem %q", k, got[k], v)
		}
	}
}

// Údaje, ktoré systém nehlási, sa v okne nemajú objaviť ako prázdne riadky.
func TestDetailsOmitsUnknownValues(t *testing.T) {
	s := battery.Status{Present: true, State: battery.StateDischarging, Percent: 50}
	for _, r := range Details(s) {
		switch r.Label {
		case "Kapacita", "Zdravie batérie", "Počet cyklov", "Napätie":
			t.Errorf("riadok %q sa nemal zobraziť (údaj nie je známy)", r.Label)
		}
	}
}

func TestDetailsRelativeUnitsHideCapacity(t *testing.T) {
	s := battery.Status{Present: true, State: battery.StateDischarging, Percent: 50,
		Capacity: 500, FullCapacity: 1000, Relative: true}
	for _, r := range Details(s) {
		if r.Label == "Kapacita" {
			t.Error("pri relatívnych jednotkách nemá zmysel ukazovať mWh")
		}
	}
}
