//go:build windows

package win

import "unsafe"

// Správy okna, ktoré aplikácia spracúva.
const (
	WMDestroy       = 0x0002
	WMClose         = 0x0010
	WMPaint         = 0x000F
	WMEraseBkgnd    = 0x0014
	WMTimer         = 0x0113
	WMCommand       = 0x0111
	WMActivate      = 0x0006
	WMSettingChange = 0x001A
	WMDPIChanged    = 0x02E0
	WMPowerBroadcst = 0x0218
	WMApp           = 0x8000
	WMLButtonUp     = 0x0202
	WMRButtonUp     = 0x0205
	WMContextMenu   = 0x007B
	WMLButtonDown   = 0x0201
	WMKeyDown       = 0x0100
	WMSetFocus      = 0x0007

	VKEscape = 0x1B

	WAInactive = 0
)

// Správy, ktorými ikona v oznamovacej oblasti hlási kliknutie, keď je
// zaregistrovaná ako verzia 4. Ľavé kliknutie vtedy NEPRÍDE ako
// WM_LBUTTONUP, ale ako NIN_SELECT – bez toho by ikona na kliknutie
// nereagovala.
const (
	NINSelect           = 0x0400 // WM_USER + 0
	NINKeySelect        = 0x0401
	NINBalloonUserClick = 0x0405
)

// Štýly okna.
const (
	WSPopup        = 0x80000000
	WSVisible      = 0x10000000
	WSOverlapped   = 0x00000000
	WSEXToolWindow = 0x00000080
	WSEXTopmost    = 0x00000008
	WSEXNoActivate = 0x08000000

	SWPShowWindow = 0x0040
	SWPNoActivate = 0x0010

	SWHide     = 0
	SWShow     = 5
	SWShowNoAc = 4

	CSDropShadow = 0x00020000
	CSHRedraw    = 0x0002
	CSVRedraw    = 0x0001

	IDCArrow = 32512

	SMCXSmIcon = 49
	SMCYSmIcon = 50
)

// WndClassEx popisuje triedu okna.
type WndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

// PaintStruct nesie údaje o prekresľovaní okna.
type PaintStruct struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

// MonitorInfo popisuje monitor vrátane pracovnej plochy (bez panela úloh).
type MonitorInfo struct {
	Size    uint32
	Monitor RECT
	Work    RECT
	Flags   uint32
}

var (
	procRegisterClassEx    = user32.proc("RegisterClassExW")
	procCreateWindowEx     = user32.proc("CreateWindowExW")
	procDefWindowProc      = user32.proc("DefWindowProcW")
	procDestroyWindow      = user32.proc("DestroyWindow")
	procGetMessage         = user32.proc("GetMessageW")
	procTranslateMessage   = user32.proc("TranslateMessage")
	procDispatchMessage    = user32.proc("DispatchMessageW")
	procPostQuitMessage    = user32.proc("PostQuitMessage")
	procSetTimer           = user32.proc("SetTimer")
	procKillTimer          = user32.proc("KillTimer")
	procRegisterWndMsg     = user32.proc("RegisterWindowMessageW")
	procLoadCursor         = user32.proc("LoadCursorW")
	procGetSystemMetrics   = user32.proc("GetSystemMetrics")
	procCreatePopupMenu    = user32.proc("CreatePopupMenu")
	procAppendMenu         = user32.proc("AppendMenuW")
	procTrackPopupMenuEx   = user32.proc("TrackPopupMenuEx")
	procDestroyMenu        = user32.proc("DestroyMenu")
	procSetForegroundWin   = user32.proc("SetForegroundWindow")
	procGetCursorPos       = user32.proc("GetCursorPos")
	procBeginPaint         = user32.proc("BeginPaint")
	procEndPaint           = user32.proc("EndPaint")
	procFillRect           = user32.proc("FillRect")
	procInvalidateRect     = user32.proc("InvalidateRect")
	procShowWindow         = user32.proc("ShowWindow")
	procSetWindowPos       = user32.proc("SetWindowPos")
	procDrawText           = user32.proc("DrawTextW")
	procCreateIconFromRes  = user32.proc("CreateIconFromResourceEx")
	procDestroyIcon        = user32.proc("DestroyIcon")
	procMonitorFromPoint   = user32.proc("MonitorFromPoint")
	procGetMonitorInfo     = user32.proc("GetMonitorInfoW")
	procGetDpiForWindow    = user32.proc("GetDpiForWindow")
	procSetDpiAwareCtx     = user32.proc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware = user32.proc("SetProcessDPIAware")
	procPostMessage        = user32.proc("PostMessageW")
	procGetSysColorBrush   = user32.proc("GetSysColorBrush")
	procMessageBox         = user32.proc("MessageBoxW")
	procFindWindow         = user32.proc("FindWindowW")

	procGetModuleHandle = kernel32dll.proc("GetModuleHandleW")

	procShellNotifyIcon = shell32.proc("Shell_NotifyIconW")
	procShellNotifyRect = shell32.proc("Shell_NotifyIconGetRect")

	procCreateSolidBrush = gdi32.proc("CreateSolidBrush")
	procDeleteObject     = gdi32.proc("DeleteObject")
	procSelectObject     = gdi32.proc("SelectObject")
	procCreateFontIndir  = gdi32.proc("CreateFontIndirectW")
	procSetTextColor     = gdi32.proc("SetTextColor")
	procSetBkMode        = gdi32.proc("SetBkMode")
	procCreateCompatDC   = gdi32.proc("CreateCompatibleDC")
	procCreateCompatBmp  = gdi32.proc("CreateCompatibleBitmap")
	procDeleteDC         = gdi32.proc("DeleteDC")
	procBitBlt           = gdi32.proc("BitBlt")
	procRoundRect        = gdi32.proc("RoundRect")
	procCreatePen        = gdi32.proc("CreatePen")
	procGetStockObject   = gdi32.proc("GetStockObject")

	procDwmSetWindowAttr = dwmapi.proc("DwmSetWindowAttribute")
)

// ModuleHandle vráti popisovač bežiaceho programu.
func ModuleHandle() uintptr {
	h, _ := procGetModuleHandle.Call(0)
	return h
}

// FindWindow nájde okno podľa triedy. Vráti 0, ak také okno neexistuje.
func FindWindow(class string) HWND {
	h, _ := procFindWindow.Call(uintptr(unsafe.Pointer(Str(class))), 0)
	return h
}

// Typy okna so správou.
const (
	MBOK            = 0x0000
	MBIconError     = 0x0010
	MBIconInfo      = 0x0040
	MBSetForeground = 0x00010000
)

// MessageBox zobrazí okno so správou. Bez neho by sa chyba pri štarte
// stratila – aplikácia beží bez konzoly, takže výpis nemá kam ísť.
func MessageBox(title, text string, flags uint32) {
	procMessageBox.Call(0, uintptr(unsafe.Pointer(Str(text))),
		uintptr(unsafe.Pointer(Str(title))), uintptr(flags|MBSetForeground))
}

// AskYesNo položí otázku s tlačidlami Áno a Nie. Predvolené je Nie, nech
// potvrdenie Enterom nikdy nič nezruší.
func AskYesNo(title, text string) bool {
	const (
		mbYesNo        = 0x0004
		mbIconQuestion = 0x0020
		mbDefButton2   = 0x0100
		idYes          = 6
	)
	r, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(Str(text))),
		uintptr(unsafe.Pointer(Str(title))),
		mbYesNo|mbIconQuestion|mbDefButton2|MBSetForeground)
	return r == idYes
}

// RegisterClass zaregistruje triedu okna a vráti jej atóm.
func RegisterClass(c *WndClassEx) uintptr {
	c.Size = uint32(unsafe.Sizeof(*c))
	r, _ := procRegisterClassEx.Call(uintptr(unsafe.Pointer(c)))
	return r
}

// CreateWindow vytvorí okno.
func CreateWindow(exStyle uint32, class, title string, style uint32, x, y, w, h int32, parent, menu, instance uintptr) HWND {
	r, _ := procCreateWindowEx.Call(uintptr(exStyle), uintptr(unsafe.Pointer(Str(class))),
		uintptr(unsafe.Pointer(Str(title))), uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, menu, instance, 0)
	return r
}

// DefWindowProc je predvolené spracovanie správy.
func DefWindowProc(hwnd HWND, msg uint32, w, l uintptr) uintptr {
	r, _ := procDefWindowProc.Call(hwnd, uintptr(msg), w, l)
	return r
}

// DestroyWindow zruší okno.
func DestroyWindow(hwnd HWND) { procDestroyWindow.Call(hwnd) }

// RunMessageLoop spracúva správy, kým okno nezavolá PostQuitMessage.
func RunMessageLoop() {
	var msg MSG
	for {
		r, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = chyba
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// PostQuit ukončí slučku správ.
func PostQuit(code int32) { procPostQuitMessage.Call(uintptr(code)) }

// PostMessage pošle správu do fronty okna.
func PostMessage(hwnd HWND, msg uint32, w, l uintptr) {
	procPostMessage.Call(hwnd, uintptr(msg), w, l)
}

// SetTimer nastaví opakovaný časovač okna.
func SetTimer(hwnd HWND, id uintptr, ms uint32) { procSetTimer.Call(hwnd, id, uintptr(ms), 0) }

// KillTimer časovač zruší.
func KillTimer(hwnd HWND, id uintptr) { procKillTimer.Call(hwnd, id) }

// RegisterWindowMessage zaregistruje systémovú správu podľa mena.
// Potrebujeme „TaskbarCreated“: po reštarte Prieskumníka treba ikonu pridať znova.
func RegisterWindowMessage(name string) uint32 {
	r, _ := procRegisterWndMsg.Call(uintptr(unsafe.Pointer(Str(name))))
	return uint32(r)
}

// LoadArrowCursor vráti štandardný kurzor.
func LoadArrowCursor() uintptr {
	r, _ := procLoadCursor.Call(0, IDCArrow)
	return r
}

// GetSystemMetrics vráti systémový rozmer (napr. veľkosť malej ikony).
func GetSystemMetrics(index int32) int32 {
	r, _ := procGetSystemMetrics.Call(uintptr(index))
	return int32(r)
}

// EnableDPIAwareness zapne škálovanie podľa DPI monitora. Bez toho by bola
// ikona na obrazovkách s vyšším rozlíšením rozmazaná.
func EnableDPIAwareness() {
	const perMonitorAwareV2 = ^uintptr(3) // -4
	if procSetDpiAwareCtx.Available() {
		if r, _ := procSetDpiAwareCtx.Call(perMonitorAwareV2); r != 0 {
			return
		}
	}
	procSetProcessDPIAware.Call()
}

// DPIForWindow vráti DPI okna (96 = 100 %).
func DPIForWindow(hwnd HWND) int32 {
	if procGetDpiForWindow.Available() {
		if r, _ := procGetDpiForWindow.Call(hwnd); r > 0 {
			return int32(r)
		}
	}
	return 96
}

// --- ikona v oznamovacej oblasti --------------------------------------

const (
	nimAdd        = 0
	nimModify     = 1
	nimDelete     = 2
	nimSetVersion = 4

	nifMessage = 0x01
	nifIcon    = 0x02
	nifTip     = 0x04
	nifInfo    = 0x10
	nifShowTip = 0x80

	niifInfo = 0x01

	notifyIconVersion4 = 4
)

type notifyIconData struct {
	Size            uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	VersionTimeout  uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        GUID
	BalloonIcon     uintptr
}

type notifyIconIdentifier struct {
	Size     uint32
	_        uint32
	Wnd      uintptr
	ID       uint32
	GuidItem GUID
}

// TrayIcon je ikona v oznamovacej oblasti panela úloh.
type TrayIcon struct {
	hwnd  HWND
	id    uint32
	msg   uint32
	icon  uintptr
	added bool
}

// OK hovorí, či sa ikonu podarilo do panela pridať.
func (t *TrayIcon) OK() bool { return t.added }

// NewTrayIcon pridá ikonu do panela. CallbackMessage je správa, ktorou
// bude panel hlásiť kliknutia.
func NewTrayIcon(hwnd HWND, id, callbackMessage uint32) *TrayIcon {
	t := &TrayIcon{hwnd: hwnd, id: id, msg: callbackMessage}
	t.Add()
	return t
}

func (t *TrayIcon) data(flags uint32) *notifyIconData {
	d := &notifyIconData{Wnd: t.hwnd, ID: t.id, Flags: flags, CallbackMessage: t.msg, Icon: t.icon}
	d.Size = uint32(unsafe.Sizeof(*d))
	return d
}

// Add zaregistruje ikonu. Volá sa aj po reštarte Prieskumníka.
func (t *TrayIcon) Add() bool {
	flags := uint32(nifMessage | nifTip | nifShowTip)
	if t.icon != 0 {
		// S príznakom NIF_ICON a prázdnou ikonou by registrácia zlyhala.
		flags |= nifIcon
	}
	d := t.data(flags)
	r, _ := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(d)))
	if r == 0 {
		return false
	}
	t.added = true
	// Verzia 4 dáva presnejšie hlásenia o kliknutí vrátane polohy kurzora.
	v := t.data(0)
	v.VersionTimeout = notifyIconVersion4
	procShellNotifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(v)))
	return true
}

// Update vymení ikonu a text bubliny.
func (t *TrayIcon) Update(icon uintptr, tip string) {
	old := t.icon
	t.icon = icon
	d := t.data(nifIcon | nifTip | nifShowTip)
	CopyStr(d.Tip[:], tip)
	r, _ := procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(d)))
	if r == 0 {
		// Ikona v paneli nie je (napr. po reštarte Prieskumníka) – pridáme ju.
		t.Add()
		procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(d)))
	}
	if old != 0 && old != icon {
		procDestroyIcon.Call(old)
	}
}

// Balloon zobrazí bublinové upozornenie pri ikone. Používa sa pri prvom
// spustení: Windows 11 nové ikony schováva pod šípku a bez upozornenia by
// používateľ nevedel, že aplikácia beží.
func (t *TrayIcon) Balloon(title, text string) {
	d := t.data(nifInfo)
	CopyStr(d.InfoTitle[:], title)
	CopyStr(d.Info[:], text)
	d.InfoFlags = niifInfo
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(d)))
}

// Remove odstráni ikonu z panela.
func (t *TrayIcon) Remove() {
	d := t.data(0)
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(d)))
	if t.icon != 0 {
		procDestroyIcon.Call(t.icon)
		t.icon = 0
	}
}

// Rect vráti obdĺžnik ikony v paneli. Podľa neho sa umiestni okno s
// podrobnosťami. Ak sa nedá zistiť (skrytá ikona), vráti false.
func (t *TrayIcon) Rect() (RECT, bool) {
	if !procShellNotifyRect.Available() {
		return RECT{}, false
	}
	id := notifyIconIdentifier{Wnd: t.hwnd, ID: t.id}
	id.Size = uint32(unsafe.Sizeof(id))
	var r RECT
	hr, _ := procShellNotifyRect.Call(uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&r)))
	if hr != 0 { // S_OK == 0
		return RECT{}, false
	}
	return r, true
}

// CreateIconFromResource vyrobí ikonu z bajtov v pamäti (hlavička DIB +
// dáta), takže ju netreba mať ako súbor.
func CreateIconFromResource(data []byte, w, h int32) uintptr {
	if len(data) == 0 {
		return 0
	}
	const lrDefaultColor = 0
	r, _ := procCreateIconFromRes.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		1, 0x00030000, uintptr(w), uintptr(h), lrDefaultColor)
	return r
}

var procDrawIconEx = user32.proc("DrawIconEx")

// DrawIcon vykreslí ikonu do plochy v danej veľkosti.
func (c *Canvas) DrawIcon(hIcon uintptr, x, y, w, h int32) {
	const diNormal = 3
	procDrawIconEx.Call(c.hdc, uintptr(x), uintptr(y), hIcon, uintptr(w), uintptr(h), 0, 0, diNormal)
}

// DestroyIcon uvoľní ikonu.
func DestroyIcon(h uintptr) { procDestroyIcon.Call(h) }

// --- kontextová ponuka -------------------------------------------------

// Menu je kontextová ponuka ikony.
type Menu struct{ h uintptr }

// Príznaky položiek ponuky.
const (
	mfString    = 0x0000
	mfSeparator = 0x0800
	mfChecked   = 0x0008
	mfDisabled  = 0x0002
	mfGrayed    = 0x0001

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmBottomAlign = 0x0020
	tpmRightAlign  = 0x0008
)

// NewMenu vytvorí prázdnu ponuku.
func NewMenu() *Menu {
	h, _ := procCreatePopupMenu.Call()
	return &Menu{h: h}
}

// Item pridá položku. Ak je disabled, položka slúži len ako text.
func (m *Menu) Item(id uint32, text string, checked, disabled bool) {
	flags := uintptr(mfString)
	if checked {
		flags |= mfChecked
	}
	if disabled {
		flags |= mfDisabled | mfGrayed
	}
	procAppendMenu.Call(m.h, flags, uintptr(id), uintptr(unsafe.Pointer(Str(text))))
}

// Separator pridá oddeľovač.
func (m *Menu) Separator() { procAppendMenu.Call(m.h, mfSeparator, 0, 0) }

// Track zobrazí ponuku pri kurzore a vráti id zvolenej položky (0 = nič).
func (m *Menu) Track(hwnd HWND) uint32 {
	var p POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	// Bez prepnutia do popredia ponuka nezmizne, keď sa klikne inam.
	procSetForegroundWin.Call(hwnd)
	r, _ := procTrackPopupMenuEx.Call(m.h, tpmRightButton|tpmReturnCmd|tpmBottomAlign|tpmRightAlign,
		uintptr(p.X), uintptr(p.Y), hwnd, 0)
	// Bez tejto prázdnej správy ponuka zostane „visieť“, kým používateľ
	// neklikne druhýkrát – známa chyba pri ikonách v oznamovacej oblasti.
	PostMessage(hwnd, 0 /* WM_NULL */, 0, 0)
	return uint32(r)
}

// Destroy uvoľní ponuku.
func (m *Menu) Destroy() {
	if m.h != 0 {
		procDestroyMenu.Call(m.h)
		m.h = 0
	}
}

// CursorPos vráti polohu kurzora na obrazovke.
func CursorPos() POINT {
	var p POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

// WorkArea vráti pracovnú plochu monitora pod daným bodom (bez panela úloh).
func WorkArea(p POINT) RECT {
	const monitorDefaultToNearest = 2
	h, _ := procMonitorFromPoint.Call(uintptr(p.X), uintptr(p.Y), monitorDefaultToNearest)
	mi := MonitorInfo{}
	mi.Size = uint32(unsafe.Sizeof(mi))
	if h != 0 {
		if r, _ := procGetMonitorInfo.Call(h, uintptr(unsafe.Pointer(&mi))); r != 0 {
			return mi.Work
		}
	}
	return RECT{Left: 0, Top: 0, Right: 1024, Bottom: 768}
}
