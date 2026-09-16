//go:build windows

package win

import "unsafe"

// Color je farba v tvare, aký používa GDI (0x00BBGGRR).
type Color uint32

// RGB poskladá farbu zo zložiek.
func RGB(r, g, b uint8) Color {
	return Color(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}

// Príznaky pre DrawText.
const (
	DTLeft        = 0x0000
	DTCenter      = 0x0001
	DTRight       = 0x0002
	DTVCenter     = 0x0004
	DTSingleLine  = 0x0020
	DTCalcRect    = 0x0400
	DTNoPrefix    = 0x0800
	DTWordBreak   = 0x0010
	DTEndEllipsis = 0x8000

	srcCopy     = 0x00CC0020
	transparent = 1
	nullPen     = 8
)

// Font je písmo pre kreslenie textu.
type Font struct{ h uintptr }

type logFont struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         uint8
	Underline      uint8
	StrikeOut      uint8
	CharSet        uint8
	OutPrecision   uint8
	ClipPrecision  uint8
	Quality        uint8
	PitchAndFamily uint8
	FaceName       [32]uint16
}

// NewFont vytvorí písmo. Výška je v pixeloch, váha 400 = bežné, 600 = polotučné.
func NewFont(face string, heightPx, weight int32) *Font {
	const (
		defaultCharSet   = 1
		clearTypeQuality = 5
		variablePitch    = 2
	)
	lf := logFont{
		Height:         -heightPx, // záporná výška = výška znaku, nie riadku
		Weight:         weight,
		CharSet:        defaultCharSet,
		Quality:        clearTypeQuality,
		PitchAndFamily: variablePitch,
	}
	CopyStr(lf.FaceName[:], face)
	h, _ := procCreateFontIndir.Call(uintptr(unsafe.Pointer(&lf)))
	return &Font{h: h}
}

// Close uvoľní písmo.
func (f *Font) Close() {
	if f != nil && f.h != 0 {
		procDeleteObject.Call(f.h)
		f.h = 0
	}
}

// Canvas je plocha na kreslenie (kontext zariadenia).
type Canvas struct {
	hdc uintptr
	W   int32
	H   int32
}

// Fill vyplní obdĺžnik farbou.
func (c *Canvas) Fill(r RECT, col Color) {
	br, _ := procCreateSolidBrush.Call(uintptr(col))
	procFillRect.Call(c.hdc, uintptr(unsafe.Pointer(&r)), br)
	procDeleteObject.Call(br)
}

// RoundRect nakreslí zaoblený obdĺžnik bez obrysu.
func (c *Canvas) RoundRect(r RECT, radius int32, col Color) {
	br, _ := procCreateSolidBrush.Call(uintptr(col))
	pen, _ := procGetStockObject.Call(nullPen)
	oldBr, _ := procSelectObject.Call(c.hdc, br)
	oldPen, _ := procSelectObject.Call(c.hdc, pen)
	// RoundRect kreslí pravý a dolný okraj exkluzívne, preto +1.
	procRoundRect.Call(c.hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right+1), uintptr(r.Bottom+1),
		uintptr(radius*2), uintptr(radius*2))
	procSelectObject.Call(c.hdc, oldBr)
	procSelectObject.Call(c.hdc, oldPen)
	procDeleteObject.Call(br)
}

// Text vypíše text do obdĺžnika.
func (c *Canvas) Text(r RECT, s string, f *Font, col Color, flags uint32) {
	old, _ := procSelectObject.Call(c.hdc, f.h)
	procSetBkMode.Call(c.hdc, transparent)
	procSetTextColor.Call(c.hdc, uintptr(col))
	u := Str(s)
	procDrawText.Call(c.hdc, uintptr(unsafe.Pointer(u)), ^uintptr(0),
		uintptr(unsafe.Pointer(&r)), uintptr(flags|DTNoPrefix))
	procSelectObject.Call(c.hdc, old)
}

// TextWidth vráti šírku textu v pixeloch.
func (c *Canvas) TextWidth(s string, f *Font) int32 {
	old, _ := procSelectObject.Call(c.hdc, f.h)
	r := RECT{}
	u := Str(s)
	procDrawText.Call(c.hdc, uintptr(unsafe.Pointer(u)), ^uintptr(0),
		uintptr(unsafe.Pointer(&r)), DTCalcRect|DTSingleLine|DTNoPrefix)
	procSelectObject.Call(c.hdc, old)
	return r.Width()
}

// PaintDoubleBuffered zavolá draw nad pomocnou plochou a výsledok naraz
// prenesie do okna. Bez toho okno pri prekresľovaní bliká.
func PaintDoubleBuffered(hwnd HWND, w, h int32, draw func(*Canvas)) {
	var ps PaintStruct
	hdc, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	mem, _ := procCreateCompatDC.Call(hdc)
	bmp, _ := procCreateCompatBmp.Call(hdc, uintptr(w), uintptr(h))
	old, _ := procSelectObject.Call(mem, bmp)

	draw(&Canvas{hdc: mem, W: w, H: h})

	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, srcCopy)
	procSelectObject.Call(mem, old)
	procDeleteObject.Call(bmp)
	procDeleteDC.Call(mem)
	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

// Invalidate si vyžiada prekreslenie okna.
func Invalidate(hwnd HWND) { procInvalidateRect.Call(hwnd, 0, 0) }

// ShowWindow zobrazí alebo skryje okno.
func ShowWindow(hwnd HWND, cmd int32) { procShowWindow.Call(hwnd, uintptr(cmd)) }

// SetWindowPos presunie okno.
func SetWindowPos(hwnd HWND, x, y, w, h int32, flags uint32) {
	const hwndTopmost = ^uintptr(0) // -1
	procSetWindowPos.Call(hwnd, hwndTopmost, uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(flags))
}

// SetForeground prepne okno do popredia.
func SetForeground(hwnd HWND) { procSetForegroundWin.Call(hwnd) }

// Atribúty okna pre Desktop Window Manager.
const (
	dwmaUseImmersiveDarkMode = 20
	dwmaCornerPreference     = 33
	dwmwcpRound              = 2
)

// SetWindowRoundedDark zapne zaoblené rohy a tmavý režim okna (Windows 11).
// Na starších systémoch volanie nič nepokazí, len sa ignoruje.
func SetWindowRoundedDark(hwnd HWND, dark bool) {
	if !procDwmSetWindowAttr.Available() {
		return
	}
	pref := int32(dwmwcpRound)
	procDwmSetWindowAttr.Call(hwnd, dwmaCornerPreference, uintptr(unsafe.Pointer(&pref)), 4)
	v := int32(0)
	if dark {
		v = 1
	}
	procDwmSetWindowAttr.Call(hwnd, dwmaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&v)), 4)
}
