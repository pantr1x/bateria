//go:build windows

package winui

import (
	"fmt"
	"image"
	"math"
	"os"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/pantr1x/bateria/internal/battery"
	"github.com/pantr1x/bateria/internal/config"
	"github.com/pantr1x/bateria/internal/icon"
	"github.com/pantr1x/bateria/internal/win"
)

const (
	trayClassName = "BateriaTrayWindow"

	trayIconID      = 1
	msgTrayCallback = win.WMApp + 1
	msgReadingDone  = win.WMApp + 2
	timerRefresh    = 1
	timerPromote    = 2
)

// Položky kontextovej ponuky.
const (
	cmdDetails = iota + 100
	cmdModeSystem
	cmdModeBattery
	cmdModePercent
	cmdAutostart
	cmdPromote
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
	promoteTries   int

	// Stav batérie sa číta v samostatnej úlohe: volania ovládača idú cez
	// systém a pri chybnom ovládači vedia trvať. Keby bežali vo vlákne
	// okna, aplikácia by na ten čas prestala reagovať na kliknutia –
	// vrátane príkazu Ukončiť.
	mu      sync.Mutex
	pending battery.Status
	hasNew  bool
	reading bool
}

var (
	app      *App
	trayProc = syscall.NewCallback(trayWndProc)

	// msgShowWindow si posielajú inštancie aplikácie medzi sebou. Keď
	// používateľ spustí program znova, bežiaca inštancia otvorí okno
	// s podrobnosťami – inak by sa zdalo, že kliknutie nič neurobilo.
	msgShowWindow = win.RegisterWindowMessage("BateriaShowWindow")
)

// Run spustí aplikáciu a vráti sa, až keď sa ukončí.
func Run() error {
	win.EnableDPIAwareness()
	// Jedna inštancia stačí. Poznáme ju podľa okna s našou triedou – je to
	// spoľahlivejšie než zámok, ktorý by pri omyle aplikáciu ticho ukončil.
	if other := win.FindWindow(trayClassName); other != 0 {
		if !win.AskYesNo("Batéria",
			"Batéria už beží.\n\nChceš bežiacu verziu ukončiť a spustiť túto?\n\n"+
				"Nie = nechať bežať a len otvoriť okno s podrobnosťami.") {
			win.PostMessage(other, msgShowWindow, 0, 0)
			return nil
		}
		if !stopInstance(other) {
			return fmt.Errorf("bežiacu verziu sa nepodarilo ukončiť; " +
				"skús ju zavrieť v Správcovi úloh (položka bateria)")
		}
	}

	a := &App{est: battery.NewEstimator(), cfg: config.Default()}
	firstRun := false
	if p, err := config.Path(); err == nil {
		a.cfgPath = p
		if _, err := os.Stat(p); err != nil {
			firstRun = true
		}
		a.cfg = config.Load(p)
	}
	app = a

	inst := win.ModuleHandle()
	class := win.WndClassEx{
		WndProc:   trayProc,
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
	// Prvá ikona sa nakreslí hneď, nech v paneli nie je prázdne miesto,
	// kým dobehne prvé meranie.
	a.updateIcon()
	a.refresh()
	if !a.tray.OK() {
		return fmt.Errorf("ikonu sa nepodarilo pridať do oznamovacej oblasti")
	}
	if firstRun {
		// Windows 11 nové ikony schováva pod šípku – nech používateľ vie,
		// že aplikácia beží a kde ju má hľadať.
		a.tray.Balloon("Batéria beží",
			"Ikona je pri hodinách. Ak ju nevidíš, je skrytá pod šípkou ^ – "+
				"stačí ju odtiaľ potiahnuť myšou na panel úloh.")
		a.saveConfig()
	}
	if !a.cfg.TrayPromoted {
		// Prieskumník si ikonu zapíše do registra až chvíľu po jej pridaní,
		// preto sa o vytiahnutie z prepadovej ponuky pokúsime až o chvíľu.
		win.SetTimer(a.hwnd, timerPromote, 2000)
	}
	win.SetTimer(a.hwnd, timerRefresh, uint32(a.cfg.RefreshSeconds)*1000)

	win.RunMessageLoop()
	return nil
}

// Quit ukončí bežiacu inštanciu aplikácie. Vráti false, keď žiadna nebeží
// alebo sa ju nepodarilo zavrieť.
func Quit() bool {
	other := win.FindWindow(trayClassName)
	if other == 0 {
		return false
	}
	return stopInstance(other)
}

// stopInstance požiada okno o zavretie a počká, kým naozaj zmizne. Funguje
// aj na staršie verzie aplikácie – WM_CLOSE spracúva predvolená obsluha
// okna, ktorá okno zruší a tým ikonu z panela odstráni.
func stopInstance(hwnd win.HWND) bool {
	win.PostMessage(hwnd, win.WMClose, 0, 0)
	for i := 0; i < 60; i++ {
		if win.FindWindow(trayClassName) == 0 {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func trayWndProc(hwnd win.HWND, msg uint32, wparam, lparam uintptr) uintptr {
	defer guard("obsluha správ okna")
	a := app
	if a == nil {
		return win.DefWindowProc(hwnd, msg, wparam, lparam)
	}
	switch msg {
	case msgTrayCallback:
		// Pri verzii ikony 4 je v dolnej polovici lParam správa myši.
		// Ľavé kliknutie príde ako NIN_SELECT, nie ako WM_LBUTTONUP;
		// staršia verzia posiela WM_LBUTTONUP, preto sú tu obe.
		switch uint32(win.LoWord(lparam)) {
		case win.NINSelect, win.NINKeySelect, win.WMLButtonUp, win.NINBalloonUserClick:
			a.togglePopup()
		case win.WMContextMenu, win.WMRButtonUp:
			a.showMenu()
		}
		return 0

	case msgReadingDone:
		a.applyReading()
		return 0

	case win.WMTimer:
		switch wparam {
		case timerRefresh:
			a.refresh()
		case timerPromote:
			win.KillTimer(a.hwnd, timerPromote)
			a.promoteIcon(false)
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
	// Druhé spustenie programu: ukážeme, že aplikácia beží.
	// Porovnanie musí byť mimo switchu a s kontrolou na nulu – keby sa
	// správa nezaregistrovala, splynula by s prázdnou správou WM_NULL.
	if msgShowWindow != 0 && msg == msgShowWindow {
		if !a.cfg.TrayPromoted {
			a.promoteIcon(false)
		}
		if a.popup == nil || !a.popup.visible {
			a.togglePopup()
		}
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

// refresh spustí meranie. Výsledok si vyzdvihne obsluha okna, keď meranie
// dobehne – vlákno okna sa tak nikdy nečaká na ovládač.
func (a *App) refresh() {
	a.mu.Lock()
	if a.reading {
		a.mu.Unlock()
		return // predchádzajúce meranie ešte beží
	}
	a.reading = true
	a.mu.Unlock()

	hwnd := a.hwnd
	go func() {
		// Pád v tejto úlohe by inak zhodil celý program bez slova.
		defer func() {
			if r := recover(); r != nil {
				a.mu.Lock()
				a.reading = false
				a.mu.Unlock()
				Report(fmt.Errorf("meranie batérie: %v", r), debug.Stack())
			}
		}()
		st, err := battery.Read()
		if err != nil {
			// Bez údajov ukážeme ikonu „bez batérie“, aplikácia beží ďalej.
			st = battery.Status{}
		}
		a.mu.Lock()
		a.pending, a.hasNew, a.reading = st, true, false
		a.mu.Unlock()
		win.PostMessage(hwnd, msgReadingDone, 0, 0)
	}()
}

// applyReading premietne nameraný stav do ikony aj do okna. Beží vo vlákne
// okna, takže odhadovač aj ikona zostávajú v jedných rukách.
func (a *App) applyReading() {
	a.mu.Lock()
	st, ok := a.pending, a.hasNew
	a.hasNew = false
	a.mu.Unlock()
	if !ok {
		return
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
	light := win.TaskbarUsesLightTheme()
	key := fmt.Sprintf("%d|%v|%s|%d|%v|%v", size, light, a.cfg.IconMode,
		int(math.Round(a.status.Percent)), a.status.State == battery.StateCharging,
		a.status.Present)

	if key != a.iconKey || a.iconHandle == 0 {
		img := iconImage(a.status, size, a.cfg.IconMode, light)
		if h := win.CreateIconFromResource(icon.EncodeResource(img), size, size); h != 0 {
			a.iconHandle = h
			a.iconKey = key
		}
	}
	a.tray.Update(a.iconHandle, a.status.Tooltip())
}

// iconImage pripraví obrázok ikony pre daný stav, veľkosť a režim.
func iconImage(st battery.Status, size int32, mode string, light bool) *image.NRGBA {
	theme := icon.DarkTaskbar()
	if light {
		theme = icon.LightTaskbar()
	}
	spec := icon.Spec{
		Size: int(size), Theme: theme, Percent: st.Percent,
		Charging: st.State == battery.StateCharging, Present: st.Present,
	}
	switch mode {
	case config.IconPercent:
		spec.Mode = icon.ModePercent
	case config.IconSystem:
		if img, ok := systemIcon(spec); ok {
			return img
		}
		// Písmo symbolov v systéme nie je (staršie Windows) – nakreslíme vlastnú.
	}
	return icon.Render(spec)
}

// systemIcon vykreslí ten istý znak, akým kreslí ikonu batérie samotný
// panel úloh Windowsu. Keď písmo alebo znak chýba, vráti false.
func systemIcon(s icon.Spec) (*image.NRGBA, bool) {
	primary, fallback := icon.SystemGlyph(s.Percent, s.Charging, s.Present)
	col := s.LevelColor()
	for _, face := range []string{icon.FontFluent, icon.FontMDL2} {
		for _, r := range []rune{primary, fallback} {
			if r == 0 {
				continue
			}
			if mask, ok := glyphMask(face, r, int32(s.Size)); ok {
				return icon.MaskImage(mask, s.Size, col), true
			}
		}
	}
	return nil, false
}

// glyphMask vykreslí znak a umiestni ho do stredu ikony. Keď je znak väčší
// než ikona (pri niektorých veľkostiach písma sa to stáva), prekreslí ho
// menším písmom, aby sa zmestil celý.
func glyphMask(face string, r rune, size int32) ([]byte, bool) {
	em := size
	for attempt := 0; attempt < 3; attempt++ {
		raw, ok := win.GlyphAlpha(face, r, em, size*3)
		if !ok {
			return nil, false
		}
		mask, inkW, inkH, ok := icon.FitMask(raw, int(size*3), int(size))
		if !ok {
			return nil, false
		}
		larger := inkW
		if inkH > larger {
			larger = inkH
		}
		if larger <= int(size) {
			return mask, true
		}
		next := int32(int(em) * int(size) / larger)
		if next < 6 || next >= em {
			return mask, true // menšie už nemá zmysel, radšej mierny orez
		}
		em = next
	}
	return nil, false
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
	m.Item(cmdModeSystem, "Ikona: ako vo Windowse", a.cfg.IconMode == config.IconSystem, false)
	m.Item(cmdModeBattery, "Ikona: vlastná", a.cfg.IconMode == config.IconBattery, false)
	m.Item(cmdModePercent, "Ikona: percentá", a.cfg.IconMode == config.IconPercent, false)
	m.Separator()
	m.Item(cmdAutostart, "Spúšťať s Windowsom", win.AutostartEnabled(), false)
	m.Item(cmdPromote, "Zobraziť ikonu vždy v paneli", a.iconPromoted(), false)
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
	case cmdModeSystem:
		a.setIconMode(config.IconSystem)
	case cmdModeBattery:
		a.setIconMode(config.IconBattery)
	case cmdModePercent:
		a.setIconMode(config.IconPercent)
	case cmdAutostart:
		a.toggleAutostart()
	case cmdPromote:
		a.promoteIcon(true)
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
	a.saveConfig()
	a.refresh()
}

// iconPromoted povie, či Windows ikonu ukazuje priamo v paneli úloh.
func (a *App) iconPromoted() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	on, err := win.TrayIconPromoted(exe)
	return err == nil && on
}

// promoteIcon vytiahne ikonu z prepadovej ponuky pod šípkou priamo do panela
// úloh. Pri vyvolaní z ponuky funguje ako prepínač a výsledok ohlási.
func (a *App) promoteIcon(interactive bool) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	want := true
	if interactive {
		want = !a.iconPromoted()
	}

	if err := win.SetTrayIconPromoted(exe, want); err != nil {
		if !interactive {
			// Prieskumník o ikone ešte nevie – skúsime to o chvíľu znova.
			if a.promoteTries < 3 {
				a.promoteTries++
				win.SetTimer(a.hwnd, timerPromote, 5000)
			}
			return
		}
		win.MessageBox("Batéria",
			"Ikonu sa nepodarilo natrvalo zobraziť v paneli úloh:\n"+err.Error()+
				"\n\nDá sa to zapnúť aj ručne: Nastavenia → Prispôsobenie → "+
				"Panel úloh → Iné ikony na systémovej lište.", win.MBIconError)
		return
	}

	a.cfg.TrayPromoted = true
	a.saveConfig()
	// Prieskumník nastavenie načíta, keď ikonu pridáme nanovo.
	a.tray.Remove()
	a.tray.Add()
	a.iconKey = ""
	a.refresh()

	if interactive {
		msg := "Ikona sa bude zobrazovať priamo v paneli úloh."
		if !want {
			msg = "Ikona sa presunie späť do prepadovej ponuky pod šípkou."
		}
		win.MessageBox("Batéria",
			msg+"\n\nAk sa zmena neprejaví hneď, stačí sa odhlásiť a znova "+
				"prihlásiť do Windowsu.", win.MBIconInfo)
	}
}

func (a *App) saveConfig() {
	if a.cfgPath != "" {
		_ = config.Save(a.cfgPath, a.cfg)
	}
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
