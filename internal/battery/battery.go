// Package battery poskytuje stav batérie a odhad času do nabitia/vybitia.
//
// Logika, ktorá nezávisí od Windows (odhady, formátovanie), je v tomto
// a v estimator.go, aby sa dala testovať na ľubovoľnej platforme.
package battery

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// State je stav napájania batérie.
type State int

const (
	StateUnknown State = iota
	// StateCharging – beží nabíjanie.
	StateCharging
	// StateDischarging – beží na batérii.
	StateDischarging
	// StateFull – je v sieti a batéria je plná.
	StateFull
	// StateIdleOnAC – je v sieti, ale nenabíja sa (napr. zapnutý limit
	// nabíjania na 80 %, ktorý majú mnohé notebooky).
	StateIdleOnAC
	// StateNoBattery – v počítači nie je batéria (stolný počítač).
	StateNoBattery
)

func (s State) String() string {
	switch s {
	case StateCharging:
		return "nabíja sa"
	case StateDischarging:
		return "na batérii"
	case StateFull:
		return "v sieti, plná"
	case StateIdleOnAC:
		return "v sieti, nenabíja sa"
	case StateNoBattery:
		return "bez batérie"
	default:
		return "neznámy stav"
	}
}

// Source hovorí, odkiaľ pochádza časový odhad.
type Source int

const (
	// SrcNone – odhad nie je k dispozícii.
	SrcNone Source = iota
	// SrcDriver – z ovládača batérie (IOCTL_BATTERY_QUERY_STATUS), najpresnejšie.
	SrcDriver
	// SrcWindows – z GetSystemPowerStatus (len čas do vybitia).
	SrcWindows
	// SrcHistory – z vlastnej histórie meraní (keď ovládač rýchlosť nehlási).
	SrcHistory
)

func (s Source) String() string {
	switch s {
	case SrcDriver:
		return "ovládač batérie"
	case SrcWindows:
		return "Windows"
	case SrcHistory:
		return "meranie v čase"
	default:
		return "nedostupné"
	}
}

// Status je jedno odčítanie stavu batérie.
type Status struct {
	Present bool
	OnAC    bool
	State   State

	// Percent je nabitie 0–100. Ak sú známe kapacity, počíta sa z nich,
	// inak sa berie hodnota, ktorú hlási Windows.
	Percent float64

	// Kapacity sú v mWh. Ak je Relative == true, ovládač hlási bezrozmerné
	// jednotky – pomery (a teda aj časy) platia, ale watty nie.
	Capacity       float64
	FullCapacity   float64
	DesignCapacity float64
	Relative       bool

	// Rate je okamžitý tok energie v mW: kladný pri nabíjaní,
	// záporný pri vybíjaní. Platné len ak RateKnown.
	Rate      float64
	RateKnown bool

	// RateSmoothed je ten istý tok po vyhladení (Estimator). Okamžitá
	// hodnota skáče podľa zaťaženia, na zobrazenie je nepoužiteľná.
	RateSmoothed      float64
	RateSmoothedKnown bool

	Voltage    float64 // mV
	CycleCount uint32

	// TimeToFull / TimeToEmpty sú nenulové, len ak sa dal urobiť odhad.
	TimeToFull  time.Duration
	TimeToEmpty time.Duration
	Estimate    Source

	// RuntimeOnBattery je odhad, ako dlho by počítač vydržal, keby sa teraz
	// odpojil zo siete. Počíta sa z rýchlosti posledného vybíjania, takže
	// je k dispozícii, až keď aplikácia nejaký čas bežala na batérii.
	RuntimeOnBattery time.Duration

	SampledAt time.Time
}

// Health je zdravie batérie v percentách (plná kapacita / návrhová kapacita).
// Vracia 0, ak sa nedá zistiť.
func (s Status) Health() float64 {
	if s.DesignCapacity <= 0 || s.FullCapacity <= 0 {
		return 0
	}
	return math.Min(100, s.FullCapacity/s.DesignCapacity*100)
}

// Watts je tok energie v wattoch (kladný = nabíjanie). Druhá návratová
// hodnota je false, ak údaj nie je k dispozícii alebo je v relatívnych jednotkách.
func (s Status) Watts() (float64, bool) {
	if s.Relative {
		return 0, false
	}
	if s.RateSmoothedKnown {
		return s.RateSmoothed / 1000, true
	}
	if !s.RateKnown {
		return 0, false
	}
	return s.Rate / 1000, true
}

// Remaining vráti relevantný časový odhad pre aktuálny stav a jeho popis.
// Presne toto chceme vidieť v tooltipe: v sieti čas do plného nabitia,
// na batérii čas do vybitia.
func (s Status) Remaining() (label string, d time.Duration, ok bool) {
	switch s.State {
	case StateCharging:
		return "Do plného nabitia", s.TimeToFull, s.TimeToFull > 0
	case StateDischarging:
		return "Do vybitia", s.TimeToEmpty, s.TimeToEmpty > 0
	case StateFull, StateIdleOnAC:
		return "Do plného nabitia", 0, false
	default:
		return "", 0, false
	}
}

// FormatDuration vypíše trvanie v tvare „2 h 15 min“ / „45 min“ / „< 1 min“.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	// Zaokrúhlime na minúty, aby odhad neposkakoval po sekundách.
	total := int(math.Round(d.Minutes()))
	if total <= 0 {
		return "< 1 min"
	}
	h, m := total/60, total%60
	switch {
	case h == 0:
		return fmt.Sprintf("%d min", m)
	case m == 0:
		return fmt.Sprintf("%d h", h)
	default:
		return fmt.Sprintf("%d h %d min", h, m)
	}
}

// Tooltip je text pre bublinu pri ikone v oznamovacej oblasti.
// Windows orezáva tooltip na 127 znakov, preto ho držíme krátky.
func (s Status) Tooltip() string {
	if !s.Present {
		return "Batéria – nezistená"
	}
	out := fmt.Sprintf("%.0f %% – %s", s.Percent, s.State)
	if label, d, ok := s.Remaining(); ok {
		out += "\n" + label + ": " + FormatDuration(d)
	} else if s.State == StateFull && s.RuntimeOnBattery <= 0 {
		out += "\nBatéria je nabitá"
	} else if s.RuntimeOnBattery > 0 {
		out += "\nVýdrž po odpojení: ~ " + FormatDuration(s.RuntimeOnBattery)
	} else if s.State == StateIdleOnAC {
		out += "\nNenabíja sa (limit nabíjania)"
	} else {
		out += "\nČas sa ešte počíta…"
	}
	if w, ok := s.Watts(); ok && math.Abs(w) >= 0.1 {
		out += "\n" + verbForRate(w) + ": " + Decimal(math.Abs(w), 1) + " W"
	}
	return truncateRunes(out, 127)
}

// Decimal vypíše číslo s desatinnou čiarkou, ako sa píše po slovensky.
func Decimal(v float64, digits int) string {
	s := strconv.FormatFloat(v, 'f', digits, 64)
	return strings.Replace(s, ".", ",", 1)
}

func verbForRate(w float64) string {
	if w > 0 {
		return "Nabíjanie"
	}
	return "Spotreba"
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
