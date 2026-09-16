package battery

import (
	"math"
	"time"
)

// Estimator dopočítava čas do plného nabitia a do vybitia.
//
// Prečo to vôbec treba: Windows cez GetSystemPowerStatus hlási len
// BatteryLifeTime (čas do vybitia) a aj to nie vždy; čas do plného nabitia
// (BatteryFullLifeTime) v praxi vracia -1, teda „neznáme“. Presné čísla sa
// dajú získať z ovládača batérie – okamžitý tok energie v mW spolu s
// aktuálnou a plnou kapacitou v mWh. Keď ovládač tok nehlási, odhad urobíme
// z vlastnej histórie meraní (smernica kapacity v čase).
type Estimator struct {
	// Window je, ako ďaleko do minulosti siaha história pri výpočte smernice.
	Window time.Duration
	// Tau je časová konštanta vyhladzovania okamžitého toku energie.
	// Bez nej by odhad poskakoval podľa okamžitého zaťaženia procesora.
	Tau time.Duration

	samples   []sample
	lastState State
	smooth    float64
	hasSmooth bool
	lastAt    time.Time

	// Zapamätaná rýchlosť posledného vybíjania (jednotiek za hodinu).
	// Vďaka nej vieme aj v sieti povedať, ako dlho by počítač vydržal.
	lastDrain    float64
	drainUsesCap bool
}

type sample struct {
	at       time.Time
	capacity float64
	percent  float64
}

// Rozumné hranice pre výsledok. Čokoľvek mimo nich je šum, nie odhad.
const (
	minEstimate = 30 * time.Second
	maxEstimate = 72 * time.Hour

	minSpan    = 90 * time.Second
	minSamples = 3
)

// NewEstimator vytvorí odhadovač s predvolenými parametrami.
func NewEstimator() *Estimator {
	return &Estimator{Window: 12 * time.Minute, Tau: 45 * time.Second}
}

// Reset zahodí históriu. Volá sa pri zmene stavu (napr. odpojenie nabíjačky).
func (e *Estimator) Reset() {
	e.samples = e.samples[:0]
	e.hasSmooth = false
	e.smooth = 0
}

// RuntimeOnBattery odhadne, ako dlho by počítač vydržal, keby sa teraz
// odpojil zo siete. Vráti false, kým aplikácia nevidela vybíjanie.
func (e *Estimator) RuntimeOnBattery(s Status) (time.Duration, bool) {
	if e.lastDrain <= 0 {
		return 0, false
	}
	current := s.Percent
	if e.drainUsesCap {
		if s.Capacity <= 0 {
			return 0, false
		}
		current = s.Capacity
	}
	var d, unused time.Duration
	if !setEstimate(&d, &unused, current/e.lastDrain) {
		return 0, false
	}
	return d, true
}

// Drain vráti zapamätanú rýchlosť vybíjania (jednotiek za hodinu) a to, či
// ide o jednotky kapacity alebo percentá. Slúži na uloženie medzi behmi:
// bez nej by aplikácia po každom reštarte čakala, kým sa to znova naučí.
func (e *Estimator) Drain() (float64, bool, bool) {
	if e.lastDrain <= 0 {
		return 0, false, false
	}
	return e.lastDrain, e.drainUsesCap, true
}

// RestoreDrain vráti do odhadovača skôr zapamätanú rýchlosť vybíjania.
func (e *Estimator) RestoreDrain(perHour float64, usesCapacity bool) {
	if perHour > 0 {
		e.lastDrain, e.drainUsesCap = perHour, usesCapacity
	}
}

// rememberDrain si uloží, ako rýchlo batéria ubúdala, aby sa dal odhad
// výdrže ukázať aj počas nabíjania.
func (e *Estimator) rememberDrain(s *Status) {
	if s.State != StateDischarging || s.TimeToEmpty <= 0 {
		return
	}
	hours := s.TimeToEmpty.Hours()
	if hours <= 0 {
		return
	}
	if s.Capacity > 0 {
		e.lastDrain, e.drainUsesCap = s.Capacity/hours, true
		return
	}
	if s.Percent > 0 {
		e.lastDrain, e.drainUsesCap = s.Percent/hours, false
	}
}

// Update doplní do stavu časové odhady. Status sa mení na mieste.
func (e *Estimator) Update(s *Status) {
	if s.SampledAt.IsZero() {
		s.SampledAt = time.Now()
	}
	if !s.Present || s.State == StateNoBattery || s.State == StateUnknown {
		e.Reset()
		e.lastState = s.State
		s.TimeToFull, s.TimeToEmpty = 0, 0
		s.Estimate = SrcNone
		return
	}

	if s.State != e.lastState {
		e.Reset()
		e.lastState = s.State
	}
	e.push(s.SampledAt, s.Capacity, s.Percent)

	rate, rateOK := e.smoothRate(s)
	s.RateSmoothed = rate
	s.RateSmoothedKnown = rateOK

	// 1. Najlepší zdroj: tok energie z ovládača batérie.
	switch {
	case rateOK && e.fromRate(s, rate):
		s.Estimate = SrcDriver
	// 2. Odhad z histórie vlastných meraní.
	case e.fromHistory(s):
		s.Estimate = SrcHistory
	// 3. Pri vybíjaní ešte zostáva odhad samotného Windowsu, ak ho reader
	//    stihol naplniť (TimeToEmpty + SrcWindows).
	case s.State == StateDischarging && s.TimeToEmpty > 0 && s.Estimate == SrcWindows:
	default:
		s.TimeToFull, s.TimeToEmpty = 0, 0
		s.Estimate = SrcNone
	}

	e.rememberDrain(s)
	if s.State != StateDischarging {
		if d, ok := e.RuntimeOnBattery(*s); ok {
			s.RuntimeOnBattery = d
		}
	}
}

func (e *Estimator) push(at time.Time, capacity, percent float64) {
	// Ochrana proti preskočeniu času dozadu (zmena času, prebudenie zo spánku).
	if !e.lastAt.IsZero() && at.Before(e.lastAt) {
		e.Reset()
	}
	e.lastAt = at
	e.samples = append(e.samples, sample{at: at, capacity: capacity, percent: percent})
	cut := at.Add(-e.window())
	i := 0
	for i < len(e.samples) && e.samples[i].at.Before(cut) {
		i++
	}
	// Aspoň dve merania si necháme vždy, nech sa okno nevyprázdni.
	if i > 0 && len(e.samples)-i >= 2 {
		e.samples = append(e.samples[:0], e.samples[i:]...)
	}
}

func (e *Estimator) window() time.Duration {
	if e.Window <= 0 {
		return 12 * time.Minute
	}
	return e.Window
}

// smoothRate vyhladí okamžitý tok energie exponenciálnym priemerom.
func (e *Estimator) smoothRate(s *Status) (float64, bool) {
	if !s.RateKnown {
		e.hasSmooth = false
		return 0, false
	}
	raw := s.Rate
	if !e.hasSmooth {
		e.smooth, e.hasSmooth = raw, true
		return raw, true
	}
	dt := time.Duration(0)
	if n := len(e.samples); n >= 2 {
		dt = e.samples[n-1].at.Sub(e.samples[n-2].at)
	}
	tau := e.Tau
	if tau <= 0 {
		tau = 45 * time.Second
	}
	alpha := 1.0
	if dt > 0 {
		alpha = 1 - math.Exp(-dt.Seconds()/tau.Seconds())
	}
	// Väčší skok (napr. iný režim nabíjania) nemá zmysel doťahovať pomaly.
	if math.Abs(raw-e.smooth) > math.Abs(e.smooth)*0.75+2000 {
		e.smooth = raw
	} else {
		e.smooth += alpha * (raw - e.smooth)
	}
	return e.smooth, true
}

// fromRate: čas = zostávajúca kapacita / tok. Jednotky sa vykrátia, takže
// to platí aj pre ovládače, ktoré hlásia relatívne jednotky namiesto mWh.
func (e *Estimator) fromRate(s *Status, rate float64) bool {
	if s.FullCapacity <= 0 || s.Capacity < 0 {
		return false
	}
	switch s.State {
	case StateCharging:
		if rate <= 0 {
			return false
		}
		missing := s.FullCapacity - s.Capacity
		if missing <= 0 {
			return false
		}
		return setEstimate(&s.TimeToFull, &s.TimeToEmpty, missing/rate)
	case StateDischarging:
		drain := -rate
		if drain <= 0 || s.Capacity <= 0 {
			return false
		}
		return setEstimate(&s.TimeToEmpty, &s.TimeToFull, s.Capacity/drain)
	}
	return false
}

// fromHistory preloží meraniami priamku (metóda najmenších štvorcov)
// a zo smernice dopočíta zostávajúci čas.
func (e *Estimator) fromHistory(s *Status) bool {
	if len(e.samples) < minSamples {
		return false
	}
	span := e.samples[len(e.samples)-1].at.Sub(e.samples[0].at)
	if span < minSpan {
		return false
	}

	// Ak sú známe kapacity, počítame v nich; inak v percentách.
	useCapacity := s.FullCapacity > 0 && s.Capacity > 0
	value := func(sm sample) float64 {
		if useCapacity {
			return sm.capacity
		}
		return sm.percent
	}
	current, target := s.Percent, 100.0
	if useCapacity {
		current, target = s.Capacity, s.FullCapacity
	}

	slope, ok := slopePerHour(e.samples, value)
	if !ok {
		return false
	}
	switch s.State {
	case StateCharging:
		if slope <= 0 {
			return false
		}
		missing := target - current
		if missing <= 0 {
			return false
		}
		return setEstimate(&s.TimeToFull, &s.TimeToEmpty, missing/slope)
	case StateDischarging:
		if slope >= 0 || current <= 0 {
			return false
		}
		return setEstimate(&s.TimeToEmpty, &s.TimeToFull, current/-slope)
	}
	return false
}

// slopePerHour vráti smernicu preloženej priamky v jednotkách za hodinu.
func slopePerHour(samples []sample, value func(sample) float64) (float64, bool) {
	n := float64(len(samples))
	t0 := samples[0].at
	var sumT, sumY float64
	for _, sm := range samples {
		sumT += sm.at.Sub(t0).Hours()
		sumY += value(sm)
	}
	meanT, meanY := sumT/n, sumY/n
	var num, den float64
	for _, sm := range samples {
		dt := sm.at.Sub(t0).Hours() - meanT
		num += dt * (value(sm) - meanY)
		den += dt * dt
	}
	if den == 0 || math.IsNaN(num) || math.IsInf(num, 0) {
		return 0, false
	}
	return num / den, true
}

// setEstimate zapíše hodiny ako trvanie, ak je výsledok v rozumnom rozsahu.
func setEstimate(dst, other *time.Duration, hours float64) bool {
	if math.IsNaN(hours) || math.IsInf(hours, 0) || hours <= 0 {
		return false
	}
	d := time.Duration(hours * float64(time.Hour))
	if d < minEstimate || d > maxEstimate {
		return false
	}
	*dst, *other = d, 0
	return true
}
