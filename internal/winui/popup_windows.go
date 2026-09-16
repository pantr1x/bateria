//go:build windows

package winui

import (
	"fmt"
	"math"
	"syscall"

	"github.com/pantr1x/bateria/internal/battery"
	"github.com/pantr1x/bateria/internal/icon"
	"github.com/pantr1x/bateria/internal/win"
)

const popupClassName = "BateriaDetailWindow"

// popup je okienko s podrobnosťami, ktoré sa otvorí kliknutím na ikonu.
type popup struct {
	a       *App
	hwnd    win.HWND
	visible bool

	fontDPI                    int32
	fontBig, fontSub, fontBody *win.Font
	iconHandle                 uintptr
	iconKey                    string
}

var (
	activePopup     *popup
	popupProc       = syscall.NewCallback(popupWndProc)
	popupRegistered bool
)

func newPopup(a *App) *popup {
	inst := win.ModuleHandle()
	if !popupRegistered {
		class := win.WndClassEx{
			Style:     win.CSDropShadow,
			WndProc:   popupProc,
			Instance:  inst,
			Cursor:    win.LoadArrowCursor(),
			ClassName: win.Str(popupClassName),
		}
		if win.RegisterClass(&class) == 0 {
			return nil
		}
		popupRegistered = true
	}
	p := &popup{a: a}
	p.hwnd = win.CreateWindow(win.WSEXToolWindow|win.WSEXTopmost, popupClassName, "Batéria",
		win.WSPopup, 0, 0, 10, 10, 0, 0, inst)
	if p.hwnd == 0 {
		return nil
	}
	win.SetWindowRoundedDark(p.hwnd, !win.AppsUseLightTheme())
	activePopup = p
	return p
}

func popupWndProc(hwnd win.HWND, msg uint32, wparam, lparam uintptr) uintptr {
	p := activePopup
	if p == nil {
		return win.DefWindowProc(hwnd, msg, wparam, lparam)
	}
	switch msg {
	case win.WMPaint:
		p.paint()
		return 0
	case win.WMEraseBkgnd:
		return 1 // pozadie kreslíme sami, inak okno bliká
	case win.WMActivate:
		if win.LoWord(wparam) == win.WAInactive {
			p.hide()
		}
		return 0
	case win.WMKeyDown:
		if wparam == win.VKEscape {
			p.hide()
		}
		return 0
	case win.WMClose:
		p.hide()
		return 0
	case win.WMDPIChanged:
		p.releaseFonts()
		p.place()
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wparam, lparam)
}

func (p *popup) show() {
	win.SetWindowRoundedDark(p.hwnd, !win.AppsUseLightTheme())
	p.place()
	win.ShowWindow(p.hwnd, win.SWShow)
	win.SetForeground(p.hwnd)
	p.visible = true
	win.Invalidate(p.hwnd)
}

func (p *popup) hide() {
	if !p.visible {
		return
	}
	win.ShowWindow(p.hwnd, win.SWHide)
	p.visible = false
}

func (p *popup) refresh() { win.Invalidate(p.hwnd) }

// place umiestni okno nad ikonu v paneli a udrží ho v pracovnej ploche.
func (p *popup) place() {
	w, h := p.size()
	const margin = 12
	var x, y int32

	if r, ok := p.a.tray.Rect(); ok && r.Width() > 0 {
		work := win.WorkArea(win.POINT{X: (r.Left + r.Right) / 2, Y: (r.Top + r.Bottom) / 2})
		x = r.Right - w
		y = r.Top - h - margin
		if y < work.Top {
			// Panel úloh je hore – okno patrí pod ikonu.
			y = r.Bottom + margin
		}
		x, y = clampToWork(x, y, w, h, work, margin)
	} else {
		// Ikonu nevidíme (býva schovaná v prepadovej ponuke) – ideme ku kurzoru.
		c := win.CursorPos()
		work := win.WorkArea(c)
		x, y = c.X-w/2, c.Y-h-margin
		x, y = clampToWork(x, y, w, h, work, margin)
	}
	win.SetWindowPos(p.hwnd, x, y, w, h, win.SWPNoActivate)
}

func clampToWork(x, y, w, h int32, work win.RECT, margin int32) (int32, int32) {
	if x+w > work.Right-margin {
		x = work.Right - margin - w
	}
	if x < work.Left+margin {
		x = work.Left + margin
	}
	if y+h > work.Bottom-margin {
		y = work.Bottom - margin - h
	}
	if y < work.Top+margin {
		y = work.Top + margin
	}
	return x, y
}

// scale prepočíta logické pixely na skutočné podľa DPI okna.
func (p *popup) scale(v int32) int32 {
	return v * win.DPIForWindow(p.hwnd) / 96
}

func (p *popup) size() (int32, int32) {
	rows := int32(len(Details(p.a.status)))
	w := p.scale(320)
	// 16 okraj + 58 hlavička + 8 prúžok + 14 medzera + 16 spodný okraj
	h := p.scale(112) + rows*p.scale(24)
	return w, h
}

func (p *popup) fonts() (big, sub, body *win.Font) {
	dpi := win.DPIForWindow(p.hwnd)
	if p.fontBig == nil || p.fontDPI != dpi {
		p.releaseFonts()
		p.fontDPI = dpi
		p.fontBig = win.NewFont("Segoe UI Semibold", p.scale(30), 600)
		p.fontSub = win.NewFont("Segoe UI", p.scale(13), 400)
		p.fontBody = win.NewFont("Segoe UI", p.scale(13), 400)
	}
	return p.fontBig, p.fontSub, p.fontBody
}

func (p *popup) releaseFonts() {
	p.fontBig.Close()
	p.fontSub.Close()
	p.fontBody.Close()
	p.fontBig, p.fontSub, p.fontBody = nil, nil, nil
}

// paletu okna odvodzujeme od vzhľadu aplikácií, nie panela úloh.
type palette struct {
	bg, text, dim, track, level win.Color
}

func (p *popup) palette() palette {
	s := p.a.status
	light := win.AppsUseLightTheme()
	pal := palette{
		bg: win.RGB(32, 32, 32), text: win.RGB(255, 255, 255),
		dim: win.RGB(158, 158, 158), track: win.RGB(60, 60, 60),
	}
	theme := icon.DarkTaskbar()
	if light {
		pal = palette{
			bg: win.RGB(249, 249, 249), text: win.RGB(26, 26, 26),
			dim: win.RGB(96, 96, 96), track: win.RGB(219, 219, 219),
		}
		theme = icon.LightTaskbar()
	}
	c := theme.FG
	switch {
	case s.State == battery.StateCharging:
		c = theme.Accent
	case s.Percent < 10:
		c = theme.Critical
	case s.Percent < 20:
		c = theme.Low
	}
	pal.level = win.RGB(c.R, c.G, c.B)
	if !light && s.State != battery.StateCharging && s.Percent >= 20 {
		pal.level = win.RGB(255, 255, 255)
	}
	return pal
}

func (p *popup) paint() {
	w, h := p.size()
	pal := p.palette()
	big, sub, body := p.fonts()
	st := p.a.status
	percent, subtitle := Headline(st)
	rows := Details(st)

	pad := p.scale(16)
	win.PaintDoubleBuffered(p.hwnd, w, h, func(c *win.Canvas) {
		c.Fill(win.RECT{Left: 0, Top: 0, Right: w, Bottom: h}, pal.bg)

		// Hlavička: veľké percento, pod ním stav, vpravo ikona batérie.
		c.Text(win.RECT{Left: pad, Top: pad, Right: w - pad, Bottom: pad + p.scale(38)},
			percent, big, pal.text, win.DTLeft|win.DTSingleLine)
		c.Text(win.RECT{Left: pad, Top: pad + p.scale(38), Right: w - pad, Bottom: pad + p.scale(58)},
			subtitle, sub, pal.dim, win.DTLeft|win.DTSingleLine|win.DTEndEllipsis)
		if hIcon := p.bigIcon(); hIcon != 0 {
			s := p.scale(40)
			c.DrawIcon(hIcon, w-pad-s, pad+p.scale(8), s, s)
		}

		// Prúžok nabitia.
		barTop := pad + p.scale(64)
		barH := p.scale(8)
		bar := win.RECT{Left: pad, Top: barTop, Right: w - pad, Bottom: barTop + barH}
		c.RoundRect(bar, barH/2, pal.track)
		frac := math.Max(0, math.Min(1, st.Percent/100))
		if st.Present && frac > 0 {
			fw := int32(float64(bar.Width()) * frac)
			if fw < barH {
				fw = barH
			}
			c.RoundRect(win.RECT{Left: bar.Left, Top: bar.Top, Right: bar.Left + fw, Bottom: bar.Bottom},
				barH/2, pal.level)
		}

		// Riadky s podrobnosťami.
		y := barTop + barH + p.scale(14)
		rowH := p.scale(24)
		for _, r := range rows {
			rect := win.RECT{Left: pad, Top: y, Right: w - pad, Bottom: y + rowH}
			c.Text(rect, r.Label, body, pal.dim, win.DTLeft|win.DTSingleLine|win.DTVCenter)
			c.Text(rect, r.Value, body, pal.text, win.DTRight|win.DTSingleLine|win.DTVCenter)
			y += rowH
		}
	})
}

// bigIcon pripraví väčšiu ikonu batérie do hlavičky okna.
func (p *popup) bigIcon() uintptr {
	st := p.a.status
	size := p.scale(40)
	theme := icon.DarkTaskbar()
	if win.AppsUseLightTheme() {
		theme = icon.LightTaskbar()
	}
	charging := st.State == battery.StateCharging
	key := fmt.Sprintf("%d|%d|%v|%v|%v", size, int(math.Round(st.Percent)),
		charging, st.Present, win.AppsUseLightTheme())
	if key == p.iconKey && p.iconHandle != 0 {
		return p.iconHandle
	}
	img := icon.Render(icon.Spec{Size: int(size), Theme: theme,
		Percent: st.Percent, Charging: charging, Present: st.Present})
	h := win.CreateIconFromResource(icon.EncodeResource(img), size, size)
	if h == 0 {
		return p.iconHandle
	}
	if p.iconHandle != 0 {
		win.DestroyIcon(p.iconHandle)
	}
	p.iconHandle, p.iconKey = h, key
	return h
}
