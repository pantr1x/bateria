// Package winui je používateľské rozhranie aplikácie: ikona v oznamovacej
// oblasti, kontextová ponuka a okno s podrobnosťami.
package winui

import (
	"math"
	"strconv"
	"strings"

	"github.com/pantr1x/bateria/internal/battery"
)

// Row je jeden riadok v okne s podrobnosťami.
type Row struct{ Label, Value string }

// Headline vráti veľké číslo a vetu pod ním.
func Headline(s battery.Status) (percent, subtitle string) {
	if !s.Present {
		return "—", "Batéria sa nenašla"
	}
	percent = strconv.Itoa(int(math.Round(s.Percent))) + " %"
	subtitle = strings.ToUpper(s.State.String()[:1]) + s.State.String()[1:]
	if w, ok := s.Watts(); ok && math.Abs(w) >= 0.1 {
		verb := "spotreba"
		if w > 0 {
			verb = "príkon"
		}
		subtitle += " · " + verb + " " + battery.Decimal(math.Abs(w), 1) + " W"
	}
	return percent, subtitle
}

// Details poskladá riadky s podrobnosťami. Riadky, ktorých údaj systém
// nehlási, sa vynechajú – prázdne políčka nikomu nepomôžu.
func Details(s battery.Status) []Row {
	if !s.Present {
		return []Row{{"Stav", "v počítači nie je batéria"}}
	}
	var rows []Row

	label, d, ok := s.Remaining()
	switch {
	case ok:
		rows = append(rows, Row{label, battery.FormatDuration(d)})
	case s.State == battery.StateFull:
		rows = append(rows, Row{"Do plného nabitia", "batéria je nabitá"})
	case s.State == battery.StateIdleOnAC:
		rows = append(rows, Row{"Do plného nabitia", "nenabíja sa"})
	default:
		rows = append(rows, Row{label, "počíta sa…"})
	}

	// V sieti zaujíma aj to, ako dlho by počítač vydržal po odpojení.
	if s.State != battery.StateDischarging && s.RuntimeOnBattery > 0 {
		rows = append(rows, Row{"Výdrž po odpojení", "~ " + battery.FormatDuration(s.RuntimeOnBattery)})
	}
	if s.FullCapacity > 0 && !s.Relative {
		rows = append(rows, Row{"Kapacita", Thousands(int(math.Round(s.Capacity))) + " / " +
			Thousands(int(math.Round(s.FullCapacity))) + " mWh"})
	}
	if h := s.Health(); h > 0 {
		rows = append(rows, Row{"Zdravie batérie", battery.Decimal(h, 0) + " %"})
	}
	if s.CycleCount > 0 {
		rows = append(rows, Row{"Počet cyklov", Thousands(int(s.CycleCount))})
	}
	if s.Voltage > 0 {
		rows = append(rows, Row{"Napätie", battery.Decimal(s.Voltage/1000, 2) + " V"})
	}
	rows = append(rows, Row{"Zdroj odhadu", s.Estimate.String()})
	return rows
}

// Thousands oddelí tisícky medzerou, ako sa píše po slovensky (51 000).
func Thousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
