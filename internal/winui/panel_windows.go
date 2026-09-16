//go:build windows

package winui

import (
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pantr1x/bateria/internal/battery"
	"github.com/pantr1x/bateria/internal/icon"
	"github.com/pantr1x/bateria/internal/panelbin"
	"github.com/pantr1x/bateria/internal/win"
)

// Text v paneli úloh kreslí samostatný program bateria-panel.exe. Jeho okno
// je potomkom okna panela úloh, takže sa s panelom posúva, oreže aj skryje.
// Je to samostatný proces zámerne: okno potomka cudzieho procesu zdieľa
// s panelom vstupnú frontu, takže čokoľvek pomalé v tomto programe by
// spomalilo aj panel úloh. Tento tu len kreslí text, ktorý dostane.

const panelRetryDelay = 5 * time.Second

// startPanel rozbalí a spustí program s textom.
func (a *App) startPanel() {
	if a.cfg.PanelDisabled || a.cfgPath == "" || win.PanelRunning() {
		return
	}
	if !a.panelRetry.IsZero() && time.Since(a.panelRetry) < panelRetryDelay {
		return
	}
	a.panelRetry = time.Now()

	path, err := panelbin.Extract(filepath.Dir(a.cfgPath))
	if err != nil {
		Report(fmt.Errorf("program pre text v paneli sa nepodarilo rozbaliť: %w", err), nil)
		return
	}
	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		Report(fmt.Errorf("program pre text v paneli sa nepodarilo spustiť: %w", err), nil)
		return
	}
	// Uvoľníme popisovač procesu, keď skončí; inak by tu zostal visieť.
	go func() { _ = cmd.Wait() }()
}

// stopPanel požiada program s textom, aby sa ukončil.
func (a *App) stopPanel() {
	win.ClosePanel()
	a.panelActive = false
}

// updatePanel pošle panelu aktuálny text. Keď nebeží, skúsi ho spustiť.
func (a *App) updatePanel() {
	if a.cfg.PanelDisabled {
		a.panelActive = false
		return
	}
	gap := int32(-1)
	if a.cfg.PanelGap > 0 {
		gap = int32(a.cfg.PanelGap)
	}
	a.panelActive = win.UpdatePanel(panelText(a.status),
		panelColor(a.status, win.TaskbarUsesLightTheme()), gap)
	if !a.panelActive {
		a.startPanel()
	}
}

// panelText je text, ktorý sa vypíše do panela úloh.
func panelText(st battery.Status) string {
	if !st.Present {
		return "bez batérie"
	}
	if _, d, ok := st.Remaining(); ok {
		return battery.FormatDuration(d)
	}
	if st.RuntimeOnBattery > 0 {
		// Plná batéria v sieti: ukážeme, ako dlho by vydržala po odpojení.
		return "~ " + battery.FormatDuration(st.RuntimeOnBattery)
	}
	return fmt.Sprintf("%d %%", int(math.Round(st.Percent)))
}

// panelColor je farba textu: zelená pri nabíjaní, žltá a červená pri
// nízkom nabití, inak farba popredia panela úloh.
func panelColor(st battery.Status, lightTaskbar bool) win.Color {
	theme := icon.DarkTaskbar()
	if lightTaskbar {
		theme = icon.LightTaskbar()
	}
	c := icon.Spec{
		Percent:  st.Percent,
		Charging: st.State == battery.StateCharging,
		Present:  st.Present,
		Theme:    theme,
	}.LevelColor()
	return win.RGB(c.R, c.G, c.B)
}
