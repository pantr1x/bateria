package battery

import (
	"math"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "—"},
		{-time.Minute, "—"},
		{20 * time.Second, "< 1 min"},
		{45 * time.Minute, "45 min"},
		{2 * time.Hour, "2 h"},
		{2*time.Hour + 15*time.Minute, "2 h 15 min"},
		{89 * time.Second, "1 min"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.in); got != c.want {
			t.Errorf("FormatDuration(%v) = %q, chcem %q", c.in, got, c.want)
		}
	}
}

func approxHours(t *testing.T, d time.Duration, want float64, tol float64) {
	t.Helper()
	got := d.Hours()
	if math.Abs(got-want) > tol {
		t.Errorf("odhad %v (%.3f h), chcem %.3f h (±%.3f)", d, got, want, tol)
	}
}

func TestRateChargingGivesTimeToFull(t *testing.T) {
	e := NewEstimator()
	s := &Status{
		Present: true, OnAC: true, State: StateCharging,
		Capacity: 30000, FullCapacity: 50000, Percent: 60,
		Rate: 20000, RateKnown: true, SampledAt: time.Now(),
	}
	e.Update(s)
	if s.Estimate != SrcDriver {
		t.Fatalf("zdroj = %v, chcem SrcDriver", s.Estimate)
	}
	approxHours(t, s.TimeToFull, 1, 0.01)
	if s.TimeToEmpty != 0 {
		t.Errorf("pri nabíjaní nemá byť čas do vybitia, je %v", s.TimeToEmpty)
	}
	if w, ok := s.Watts(); !ok || math.Abs(w-20) > 0.01 {
		t.Errorf("Watts() = %.2f, %v; chcem 20 W", w, ok)
	}
}

func TestRateDischargingGivesTimeToEmpty(t *testing.T) {
	e := NewEstimator()
	s := &Status{
		Present: true, State: StateDischarging,
		Capacity: 25000, FullCapacity: 50000, Percent: 50,
		Rate: -10000, RateKnown: true, SampledAt: time.Now(),
	}
	e.Update(s)
	if s.Estimate != SrcDriver {
		t.Fatalf("zdroj = %v, chcem SrcDriver", s.Estimate)
	}
	approxHours(t, s.TimeToEmpty, 2.5, 0.01)
	if s.TimeToFull != 0 {
		t.Errorf("pri vybíjaní nemá byť čas do nabitia, je %v", s.TimeToFull)
	}
}

// Ovládače s relatívnymi jednotkami: watty nedávajú zmysel, ale pomer áno.
func TestRelativeUnitsStillGiveTime(t *testing.T) {
	e := NewEstimator()
	s := &Status{
		Present: true, State: StateDischarging, Relative: true,
		Capacity: 500, FullCapacity: 1000, Percent: 50,
		Rate: -250, RateKnown: true, SampledAt: time.Now(),
	}
	e.Update(s)
	approxHours(t, s.TimeToEmpty, 2, 0.01)
	if _, ok := s.Watts(); ok {
		t.Error("pri relatívnych jednotkách sa watty hlásiť nemajú")
	}
}

// Keď ovládač tok nehlási, odhad musí vzniknúť z histórie meraní.
func TestHistoryEstimateWhenRateUnknown(t *testing.T) {
	e := NewEstimator()
	base := time.Now().Add(-10 * time.Minute)
	var last *Status
	// 10 minút nabíjania rýchlosťou 10 000 mWh/h, z 20 000 na 50 000 chýba 30 000.
	for i := 0; i <= 20; i++ {
		at := base.Add(time.Duration(i) * 30 * time.Second)
		cap := 20000 + 10000*at.Sub(base).Hours()
		last = &Status{
			Present: true, OnAC: true, State: StateCharging,
			Capacity: cap, FullCapacity: 50000, Percent: cap / 500,
			SampledAt: at,
		}
		e.Update(last)
	}
	if last.Estimate != SrcHistory {
		t.Fatalf("zdroj = %v, chcem SrcHistory", last.Estimate)
	}
	// Po 10 minútach je kapacita ~21 667, chýba ~28 333 → ~2,83 h.
	approxHours(t, last.TimeToFull, (50000-21666.7)/10000, 0.05)
}

func TestHistoryNeedsEnoughSpan(t *testing.T) {
	e := NewEstimator()
	base := time.Now()
	var last *Status
	for i := 0; i < 3; i++ {
		last = &Status{
			Present: true, State: StateDischarging,
			Capacity: 40000 - float64(i)*10, FullCapacity: 50000, Percent: 80,
			SampledAt: base.Add(time.Duration(i) * 10 * time.Second),
		}
		e.Update(last)
	}
	if last.Estimate != SrcNone || last.TimeToEmpty != 0 {
		t.Errorf("z 20 sekúnd meraní sa odhad robiť nemá: %v / %v", last.Estimate, last.TimeToEmpty)
	}
}

// Po odpojení nabíjačky sa história nesmie miešať so starými meraniami.
func TestStateChangeResetsHistory(t *testing.T) {
	e := NewEstimator()
	base := time.Now().Add(-10 * time.Minute)
	for i := 0; i <= 20; i++ {
		at := base.Add(time.Duration(i) * 30 * time.Second)
		cap := 20000 + 10000*at.Sub(base).Hours()
		e.Update(&Status{
			Present: true, OnAC: true, State: StateCharging,
			Capacity: cap, FullCapacity: 50000, Percent: cap / 500, SampledAt: at,
		})
	}
	s := &Status{
		Present: true, State: StateDischarging,
		Capacity: 21666, FullCapacity: 50000, Percent: 43,
		SampledAt: base.Add(10*time.Minute + 30*time.Second),
	}
	e.Update(s)
	if len(e.samples) != 1 {
		t.Errorf("história po zmene stavu má mať 1 meranie, má %d", len(e.samples))
	}
	if s.Estimate != SrcNone {
		t.Errorf("hneď po zmene stavu nemá byť odhad, je %v", s.Estimate)
	}
}

// Windows vie povedať len čas do vybitia; ak nemáme nič lepšie, necháme ho.
func TestWindowsFallbackKept(t *testing.T) {
	e := NewEstimator()
	s := &Status{
		Present: true, State: StateDischarging, Percent: 55,
		TimeToEmpty: 90 * time.Minute, Estimate: SrcWindows,
		SampledAt: time.Now(),
	}
	e.Update(s)
	if s.Estimate != SrcWindows || s.TimeToEmpty != 90*time.Minute {
		t.Errorf("odhad Windowsu sa mal zachovať, je %v / %v", s.Estimate, s.TimeToEmpty)
	}
}

func TestAbsurdEstimatesRejected(t *testing.T) {
	e := NewEstimator()
	s := &Status{
		Present: true, State: StateCharging,
		Capacity: 1, FullCapacity: 50000, Percent: 0,
		Rate: 1, RateKnown: true, SampledAt: time.Now(), // 50 000 hodín
	}
	e.Update(s)
	if s.TimeToFull != 0 || s.Estimate == SrcDriver {
		t.Errorf("nezmyselný odhad sa mal zahodiť: %v / %v", s.TimeToFull, s.Estimate)
	}
}

func TestSmoothingDampensSpikes(t *testing.T) {
	e := NewEstimator()
	base := time.Now()
	for i := 0; i < 10; i++ {
		e.Update(&Status{
			Present: true, State: StateDischarging,
			Capacity: 25000, FullCapacity: 50000, Percent: 50,
			Rate: -10000, RateKnown: true,
			SampledAt: base.Add(time.Duration(i) * 2 * time.Second),
		})
	}
	s := &Status{
		Present: true, State: StateDischarging,
		Capacity: 25000, FullCapacity: 50000, Percent: 50,
		Rate: -14000, RateKnown: true, // krátky výkyv
		SampledAt: base.Add(20 * time.Second),
	}
	e.Update(s)
	if s.RateSmoothed < -12000 {
		t.Errorf("vyhladená hodnota %.0f mW príliš sleduje výkyv", s.RateSmoothed)
	}
	if s.RateSmoothed > -10000 {
		t.Errorf("vyhladená hodnota %.0f mW sa výkyvom vôbec nepohla", s.RateSmoothed)
	}
}

func TestTooltip(t *testing.T) {
	s := Status{
		Present: true, OnAC: true, State: StateCharging, Percent: 73,
		TimeToFull:   time.Hour + 12*time.Minute,
		RateSmoothed: 24500, RateSmoothedKnown: true,
	}
	want := "73 % – nabíja sa\nDo plného nabitia: 1 h 12 min\nNabíjanie: 24,5 W"
	got := s.Tooltip()
	if got != want {
		t.Errorf("Tooltip() =\n%q\nchcem\n%q", got, want)
	}
	if n := len([]rune(got)); n > 127 {
		t.Errorf("tooltip má %d znakov, Windows berie 127", n)
	}
}

// V sieti chceme vedieť aj to, ako dlho by počítač vydržal po odpojení.
// Rýchlosť vybíjania si odhadovač pamätá z predchádzajúceho behu na batérii.
func TestRuntimeOnBatteryRememberedWhileCharging(t *testing.T) {
	e := NewEstimator()
	now := time.Now()
	e.Update(&Status{
		Present: true, State: StateDischarging,
		Capacity: 25000, FullCapacity: 50000, Percent: 50,
		Rate: -10000, RateKnown: true, SampledAt: now,
	})

	charging := &Status{
		Present: true, OnAC: true, State: StateCharging,
		Capacity: 25000, FullCapacity: 50000, Percent: 50,
		Rate: 20000, RateKnown: true, SampledAt: now.Add(time.Minute),
	}
	e.Update(charging)
	if charging.RuntimeOnBattery == 0 {
		t.Fatal("odhad výdrže po odpojení chýba")
	}
	approxHours(t, charging.RuntimeOnBattery, 2.5, 0.05)
	// Nabíjanie samotné musí zostať nedotknuté.
	approxHours(t, charging.TimeToFull, 1.25, 0.05)
}

func TestRuntimeOnBatteryUnknownAtStart(t *testing.T) {
	e := NewEstimator()
	s := &Status{
		Present: true, OnAC: true, State: StateCharging,
		Capacity: 25000, FullCapacity: 50000, Percent: 50,
		Rate: 20000, RateKnown: true, SampledAt: time.Now(),
	}
	e.Update(s)
	if s.RuntimeOnBattery != 0 {
		t.Errorf("bez skúsenosti s vybíjaním nemá byť odhad výdrže, je %v", s.RuntimeOnBattery)
	}
}

// Pri vybíjaní by dvojitý údaj („zostáva“ aj „výdrž po odpojení“) mýlil.
func TestRuntimeOnBatteryHiddenWhileDischarging(t *testing.T) {
	e := NewEstimator()
	now := time.Now()
	for i := 0; i < 2; i++ {
		s := &Status{
			Present: true, State: StateDischarging,
			Capacity: 25000, FullCapacity: 50000, Percent: 50,
			Rate: -10000, RateKnown: true, SampledAt: now.Add(time.Duration(i) * time.Minute),
		}
		e.Update(s)
		if s.RuntimeOnBattery != 0 {
			t.Errorf("pri vybíjaní sa výdrž po odpojení nemá vypĺňať, je %v", s.RuntimeOnBattery)
		}
	}
}

// Pri plnej batérii v sieti neexistuje ani čas do nabitia, ani do vybitia.
// Vtedy má cenu ukázať, ako dlho by počítač vydržal po odpojení.
func TestTooltipShowsRuntimeWhenFull(t *testing.T) {
	s := Status{
		Present: true, OnAC: true, State: StateFull, Percent: 100,
		RuntimeOnBattery: 5*time.Hour + 20*time.Minute,
	}
	want := "100 % – v sieti, plná\nVýdrž po odpojení: ~ 5 h 20 min"
	if got := s.Tooltip(); got != want {
		t.Errorf("Tooltip() =\n%q\nchcem\n%q", got, want)
	}
}

func TestTooltipFullWithoutHistory(t *testing.T) {
	s := Status{Present: true, OnAC: true, State: StateFull, Percent: 100}
	want := "100 % – v sieti, plná\nBatéria je nabitá"
	if got := s.Tooltip(); got != want {
		t.Errorf("Tooltip() = %q, chcem %q", got, want)
	}
}

// Zapamätaná rýchlosť vybíjania musí prežiť reštart aplikácie.
func TestDrainSurvivesRestore(t *testing.T) {
	e := NewEstimator()
	e.Update(&Status{
		Present: true, State: StateDischarging,
		Capacity: 25000, FullCapacity: 50000, Percent: 50,
		Rate: -10000, RateKnown: true, SampledAt: time.Now(),
	})
	perHour, usesCap, ok := e.Drain()
	if !ok || perHour <= 0 {
		t.Fatalf("Drain() = %.0f, %v, %v", perHour, usesCap, ok)
	}

	fresh := NewEstimator()
	if _, _, ok := fresh.Drain(); ok {
		t.Error("nový odhadovač nemá čo pamätať")
	}
	fresh.RestoreDrain(perHour, usesCap)
	s := Status{Present: true, OnAC: true, State: StateFull,
		Capacity: 50000, FullCapacity: 50000, Percent: 100}
	d, ok := fresh.RuntimeOnBattery(s)
	if !ok {
		t.Fatal("po obnovení sa výdrž mala dať spočítať")
	}
	approxHours(t, d, 5, 0.05)
}
