//go:build windows

package winui

import (
	"fmt"
	"os"
	"strings"

	"github.com/pantr1x/bateria/internal/battery"
	"github.com/pantr1x/bateria/internal/config"
	"github.com/pantr1x/bateria/internal/icon"
	"github.com/pantr1x/bateria/internal/win"
)

// Diagnose zistí, čo aplikácia v systéme vidí. Spúšťa sa prepínačom -diag
// a výsledok sa ukáže v okne so správou – aplikácia beží bez konzoly, takže
// bežný výpis by nemal kam ísť.
func Diagnose() string {
	var b strings.Builder
	line := func(format string, args ...any) {
		fmt.Fprintf(&b, format+"\n", args...)
	}

	exe, _ := os.Executable()
	line("Program: %s", exe)
	if win.FindWindow(trayClassName) != 0 {
		line("Beh: aplikácia už beží (druhé spustenie sa ticho ukončí)")
	} else {
		line("Beh: aplikácia zatiaľ nebeží")
	}

	if exe != "" {
		switch on, err := win.TrayIconPromoted(exe); {
		case err != nil:
			line("Ikona priamo v paneli: %v", err)
		case on:
			line("Ikona priamo v paneli: áno")
		default:
			line("Ikona priamo v paneli: nie (je skrytá pod šípkou ^)")
		}
	}

	if p, err := config.Path(); err == nil {
		stav := "zatiaľ neexistuje"
		if _, err := os.Stat(p); err == nil {
			stav = "existuje"
		}
		line("Nastavenia: %s (%s)", p, stav)
	}
	line("")

	st, err := battery.Read()
	if err != nil {
		line("Batéria: nepodarilo sa prečítať (%v)", err)
	} else {
		est := battery.NewEstimator()
		est.Update(&st)
		line("Batéria: %s", strings.ReplaceAll(st.Tooltip(), "\n", " | "))
		line("Prítomná: %v, v sieti: %v, stav: %s", st.Present, st.OnAC, st.State)
		if st.FullCapacity > 0 {
			line("Kapacita: %.0f / %.0f (návrh %.0f)", st.Capacity, st.FullCapacity, st.DesignCapacity)
		}
		line("Tok energie z ovládača: známy=%v, %.0f mW", st.RateKnown, st.Rate)
		line("Zdroj odhadu: %s", st.Estimate)
	}
	line("")

	size := trayIconSize()
	line("Veľkosť ikony v paneli: %d px", size)
	line("Svetlý panel úloh: %v, svetlý vzhľad okien: %v",
		win.TaskbarUsesLightTheme(), win.AppsUseLightTheme())

	primary, _ := icon.SystemGlyph(st.Percent, st.State == battery.StateCharging, true)
	for _, face := range []string{icon.FontFluent, icon.FontMDL2} {
		if _, ok := glyphMask(face, primary, size); ok {
			line("Systémové písmo ikon: %s – znak %#x k dispozícii", face, primary)
		} else {
			line("Systémové písmo ikon: %s – nedostupné", face)
		}
	}
	return b.String()
}
