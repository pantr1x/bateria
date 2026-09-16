//go:build windows

package winui

import (
	"fmt"
	"math"
	"os"
	"syscall"

	"github.com/pantr1x/bateria/internal/battery"
	"github.com/pantr1x/bateria/internal/config"
	"github.com/pantr1x/bateria/internal/icon"
	"github.com/pantr1x/bateria/internal/win"
)

const (
	trayClassName = "BateriaTrayWindow"
	mutexName     = `Local\BateriaTrayApp`

	trayIconID      = 1
	msgTrayCallback = win.WMApp + 1
	timerRefresh    = 1
)

// Položky kontextovej ponuky.
const (
	cmdDetails = iota + 100
	cmdModeBattery
	cmdModePercent
	cmdAutostart
	cmdExit
)

// App drží stav bežiacej aplikácie. Beží len jedna inštancia, preto si ju
// obsluha správ okna nájde cez premennú balíka.
type App struct {
	hwnd    win.HWND
	tray    *win.TrayIcon
	popup   *popup
	est     *battery.Estimator
	status  battery.Status
	cfg     config.Config
	cfgPath string

	taskbarCreated uint32
	iconHandle     uintptr
	iconKey        string
	menuOpen       bool
}

var (
	app          *App
	trayProcOnce = syscall.NewCallback(trayWndProc)
)

// Run spustí aplikáciu a vráti sa, až keď sa ukončí.
func Run() error {
	win.EnableDPIAwareness()
	if !win.SingleInstance(mutexName) {
		return nil // aplikácia už beží, druhá ikona v paneli netreba
	}

	a := &App{est: battery.NewEstimator()}
	if p, err := config.Path(); err == nil {
		a.cfgPath = p
		a.cfg = config.Load(p)
	} else {
		a.cfg = config.Default()
	}
	app = a

	inst := win.ModuleHandle()
	class := win.WndClassEx{
		WndProc:   trayProcOnce,
		Instance:  inst,
		Cursor:    win.LoadArrowCursor(),
		ClassName: win.Str(trayClassName),
	}
	if win.RegisterClass(&class) == 0 {
		return fmt.Errorf("triedu okna sa nepodarilo zaregistrovať")
	}
	a.hwnd = win.CreateWindow(0, trayClassName, "Batéria", win.WSOverlapped,
		0, 0, 0, 0, 0, 0, inst)
	if a.hwnd == 0 {
		return fmt.Errorf("okno aplikácie sa nepodarilo vytvoriť")
	}

	// Po reštarte Prieskumníka sa panel úloh vytvorí nanovo a ikonu treba
	// pridať znova – inak by aplikácia bežala neviditeľne.
	a.taskbarCreated = win.RegisterWindowMessage("TaskbarCreated")

	a.tray = win.NewTrayIcon(a.hwnd, trayIconID, msgTrayCallback)
	a.refresh()
	win.SetTimer(a.hwnd, timerRefresh, uint32(a.cfg.RefreshSeconds)*1000)

	win.RunMessageLoop()
	return nil
}

func trayWndProc(hwnd win.HWND, msg uint32, wparam, lparam uintptr) uintptr {
	a := app
	if a == nil {
		return win.DefWindowProc(hwnd, msg, wparam, lparam)
	}
	switch msg {
	case msgTrayCallback:
		// Pri verzii ikony 4 je v dolnej polovici lParam správa myši.
		switch uint32(win.LoWord(lparam)) {
		case win.WMLButtonUp:
			a.togglePopup()
		case win.WMContextMenu, win.WMRButtonUp:
			a.showMenu()
		}
		return 0

	case win.WMTimer:
		if wparam == timerRefresh {
			a.refresh()
		}
		return 0

	case win.WMSettingChange:
		// Zmena svetlého/tmavého režimu: ikonu treba prekresliť inou farbou.
		a.iconKey = ""
		a.refresh()
		return 0

	case win.WMPowerBroadcst:
		// Pripojenie nabíjačky nečakáme na ďalší tik časovača.
		a.refresh()
		return 1

	case win.WMDestroy:
		a.tray.Remove()
		win.PostQuit(0)
		return 0
	}
	if a.taskbarCreated != 0 && msg == a.taskbarCreated {
		a.tray.Add()
		a.iconKey = ""
		a.refresh()
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wparam, lparam)
}

// refresh odmeria stav batérie a premietne ho do ikony aj do okna.
func (a *App) refresh() {
	st, err := battery.Read()
	if err != nil {
		// Bez údajov ukážeme ikonu „bez batérie“, aplikácia beží ďalej.
		st = battery.Status{SampledAt: st.SampledAt}
	}
	a.est.Update(&st)
	a.status = st
	a.updateIcon()
	if a.popup != nil && a.popup.visible {
		a.popup.refresh()
	}
}

// updateIcon prekreslí ikonu, len keď sa naozaj zmenil jej obsah.
// Bublinu s časom treba nastaviť vždy – mení sa aj pri rovnakom obrázku.
func (a *App) updateIcon() {
	size := trayIconSize()
	theme := icon.DarkTaskbar()
	if win.TaskbarUsesLightTheme() {
		theme = icon.LightTaskbar()
	}
	mode := icon.ModeBattery
	if a.cfg.IconMode == config.IconPercent {
		mode = icon.ModePercent
	}
	charging := a.status.State == battery.StateCharging
	key := fmt.Sprintf("%d|%v|%d|%d|%v|%v", size, win.TaskbarUsesLightTheme(), mode,
		int(math.Round(a.status.Percent)), charging, a.status.Present)

	if key != a.iconKey || a.iconHandle == 0 {
		img := icon.Render(icon.Spec{
			Size: int(size), Mode: mode, Theme: theme,
			Percent: a.status.Percent, Charging: charging, Present: a.status.Present,
		})
		if h := win.CreateIconFromResource(icon.EncodeResource(img), size, size); h != 0 {
			a.iconHandle = h
			a.iconKey = key
		}
	}
	a.tray.Update(a.iconHandle, a.status.Tooltip())
}

// trayIconSize vráti veľkosť malej ikony pri aktuálnom rozlíšení.
func trayIconSize() int32 {
	s := win.GetSystemMetrics(win.SMCXSmIcon)
	if s < 16 {
		s = 16
	}
	if s > 64 {
		s = 64
	}
	return s
}

func (a *App) showMenu() {
	if a.menuOpen {
		return // pravé tlačidlo pošle dve správy, ponuku otvárame raz
	}
	a.menuOpen = true
	defer func() { a.menuOpen = false }()

	m := win.NewMenu()
	defer m.Destroy()

	// Prvé dva riadky sú len informácia, preto sú neaktívne.
	percent, subtitle := Headline(a.status)
	m.Item(0, percent+" · "+subtitle, false, true)
	if label, d, ok := a.status.Remaining(); ok {
		m.Item(0, label+": "+battery.FormatDuration(d), false, true)
	}
	m.Separator()
	m.Item(cmdDetails, "Podrobnosti…", false, false)
	m.Separator()
	m.Item(cmdModeBattery, "Ikona: batéria", a.cfg.IconMode == config.IconBattery, false)
	m.Item(cmdModePercent, "Ikona: percentá", a.cfg.IconMode == config.IconPercent, false)
	m.Separator()
	m.Item(cmdAutostart, "Spúšťať s Windowsom", win.AutostartEnabled(), false)
	m.Separator()
	m.Item(cmdExit, "Ukončiť", false, false)

	if id := m.Track(a.hwnd); id != 0 {
		a.command(id)
	}
}

func (a *App) command(id uint32) {
	switch id {
	case cmdDetails:
		a.togglePopup()
	case cmdModeBattery:
		a.setIconMode(config.IconBattery)
	case cmdModePercent:
		a.setIconMode(config.IconPercent)
	case cmdAutostart:
		a.toggleAutostart()
	case cmdExit:
		win.DestroyWindow(a.hwnd)
	}
}

func (a *App) setIconMode(mode string) {
	if a.cfg.IconMode == mode {
		return
	}
	a.cfg.IconMode = mode
	a.iconKey = ""
	if a.cfgPath != "" {
		_ = config.Save(a.cfgPath, a.cfg)
	}
	a.refresh()
}

func (a *App) toggleAutostart() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = win.SetAutostart(!win.AutostartEnabled(), exe)
}

func (a *App) togglePopup() {
	if a.popup == nil {
		a.popup = newPopup(a)
	}
	if a.popup == nil {
		return
	}
	if a.popup.visible {
		a.popup.hide()
		return
	}
	a.popup.show()
}
