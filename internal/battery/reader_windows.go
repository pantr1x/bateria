//go:build windows

package battery

import (
	"time"

	"github.com/pantr1x/bateria/internal/win"
)

// Read prečíta aktuálny stav batérie. Spája dva zdroje:
//
//   - GetSystemPowerStatus: percentá, či sme v sieti, čas do vybitia;
//   - ovládač batérie (IOCTL): kapacity a okamžitý tok energie v mW,
//     z ktorého sa jediného dá spočítať čas do plného nabitia.
//
// Keď druhý zdroj nie je k dispozícii (niektoré ovládače ho nehlásia),
// stav sa poskladá z toho prvého a zvyšok doplní Estimator.
func Read() (Status, error) {
	s := Status{SampledAt: time.Now()}

	sps, err := win.GetSystemPowerStatus()
	if err == nil {
		s.OnAC = sps.ACLineStatus == 1
		s.Present = sps.BatteryFlag&win.BatteryFlagNoBattery == 0 && sps.BatteryFlag != win.BatteryFlagUnknown
		if sps.BatteryLifePercent <= 100 {
			s.Percent = float64(sps.BatteryLifePercent)
		}
		if sps.BatteryFlag&win.BatteryFlagCharging != 0 {
			s.State = StateCharging
		}
		if sps.BatteryLifeTime != win.Unknown && sps.BatteryLifeTime > 0 {
			s.TimeToEmpty = time.Duration(sps.BatteryLifeTime) * time.Second
			s.Estimate = SrcWindows
		}
	}

	readings, rerr := win.ReadBatteries()
	if rerr == nil && len(readings) > 0 {
		applyReadings(&s, readings)
	} else if err != nil {
		return s, err
	}

	if !s.Present {
		s.State = StateNoBattery
		return s, nil
	}
	if s.State != StateCharging {
		s.State = restState(s)
	} else if s.Percent >= 100 || (s.FullCapacity > 0 && s.Capacity >= s.FullCapacity) {
		// Niektoré ovládače hlásia nabíjanie aj pri plnej batérii. Vtedy by
		// sa čas do nabitia počítal z nuly, čo nedáva zmysel.
		s.State = StateFull
	}
	return s, nil
}

// applyReadings doplní do stavu údaje z ovládača. Viac batérií sa spočíta
// dokopy – systém ich aj tak vybíja ako jeden zdroj.
func applyReadings(s *Status, rs []win.BatteryReading) {
	var capacity, full, design, rate float64
	var rateKnown, charging, discharging, present bool
	var driverSeconds uint32
	for _, r := range rs {
		present = true
		capacity += float64(r.Capacity)
		full += float64(r.FullCapacity)
		design += float64(r.DesignedCap)
		if r.RateKnown {
			rate += float64(r.Rate)
			rateKnown = true
		}
		charging = charging || r.Charging
		discharging = discharging || r.Discharging
		if r.Relative {
			s.Relative = true
		}
		if r.CycleCount > s.CycleCount {
			s.CycleCount = r.CycleCount
		}
		if r.Voltage > 0 && s.Voltage == 0 {
			s.Voltage = float64(r.Voltage)
		}
		if r.EstimatedSeconds > 0 && (driverSeconds == 0 || r.EstimatedSeconds < driverSeconds) {
			driverSeconds = r.EstimatedSeconds
		}
	}
	s.Present = s.Present || present
	s.Capacity, s.FullCapacity, s.DesignCapacity = capacity, full, design
	if rateKnown {
		s.Rate, s.RateKnown = rate, true
	}
	if full > 0 && capacity > 0 {
		s.Percent = capacity / full * 100
		if s.Percent > 100 {
			s.Percent = 100
		}
	}
	if charging {
		s.State = StateCharging
	} else if discharging && s.State == StateUnknown {
		s.State = StateDischarging
	}
	// Odhad ovládača berieme len ako zálohu; Estimator ho prebije, keď má
	// k dispozícii tok energie.
	if driverSeconds > 0 && !charging && s.TimeToEmpty == 0 {
		s.TimeToEmpty = time.Duration(driverSeconds) * time.Second
		s.Estimate = SrcWindows
	}
}

// restState rozlíši stav, keď sa nenabíja: v sieti plná, v sieti s limitom
// nabíjania, alebo vybíjanie.
func restState(s Status) State {
	if !s.OnAC {
		return StateDischarging
	}
	if s.Percent >= 99 {
		return StateFull
	}
	return StateIdleOnAC
}
