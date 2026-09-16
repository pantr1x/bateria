// Package icon vykresľuje ikonu batérie pre oznamovaciu oblasť Windowsu.
//
// Kreslí sa cez znamienkové vzdialenostné funkcie (SDF), takže hrany sú
// vyhladené a ten istý tvar vyjde pekne v 16, 20, 24 aj 32 pixeloch –
// to je dôležité, lebo veľkosť ikony v paneli závisí od mierky DPI.
package icon

import (
	"image"
	"image/color"
	"math"
)

// Mode určuje, čo je v ikone vidieť.
type Mode int

const (
	// ModeBattery – klasický obrys batérie s naplnením, ako má Windows.
	ModeBattery Mode = iota
	// ModePercent – číslo v percentách a tenký prúžok nabitia pod ním.
	ModePercent
)

// Theme sú farby ikony. Panel úloh býva tmavý (svetlá ikona) alebo svetlý.
type Theme struct {
	FG       color.NRGBA // obrys a výplň pri bežnom stave
	Accent   color.NRGBA // nabíjanie
	Low      color.NRGBA // nízke nabitie
	Critical color.NRGBA // kriticky nízke nabitie
}

// DarkTaskbar je téma pre tmavý panel úloh (predvolené nastavenie Windowsu).
func DarkTaskbar() Theme {
	return Theme{
		FG:       color.NRGBA{236, 236, 236, 255},
		Accent:   color.NRGBA{108, 218, 122, 255},
		Low:      color.NRGBA{252, 191, 74, 255},
		Critical: color.NRGBA{255, 99, 88, 255},
	}
}

// LightTaskbar je téma pre svetlý panel úloh.
func LightTaskbar() Theme {
	return Theme{
		FG:       color.NRGBA{32, 32, 32, 255},
		Accent:   color.NRGBA{16, 124, 16, 255},
		Low:      color.NRGBA{157, 93, 0, 255},
		Critical: color.NRGBA{196, 43, 28, 255},
	}
}

// Spec je zadanie pre jedno vykreslenie.
type Spec struct {
	Size     int
	Mode     Mode
	Theme    Theme
	Percent  float64
	Charging bool
	Present  bool
}

// Prahy, pri ktorých ikona mení farbu.
const (
	criticalBelow = 10
	lowBelow      = 20
)

// levelColor je farba výplne podľa stavu nabitia. Nabíjanie sem zámerne
// nevstupuje: zelený blesk na zelenej výplni by splynul do škvrny.
func (s Spec) levelColor() color.NRGBA {
	switch {
	case s.Percent < criticalBelow:
		return s.Theme.Critical
	case s.Percent < lowBelow:
		return s.Theme.Low
	default:
		return s.Theme.FG
	}
}

// textColor je farba čísla v percentuálnom režime – tam nabíjanie ukazuje
// samotná farba, blesk je len malý doplnok v rohu.
func (s Spec) textColor() color.NRGBA {
	if s.Charging {
		return s.Theme.Accent
	}
	return s.levelColor()
}

// Render vykreslí ikonu. Výsledok má nepremultiplikovanú alfu, presne ako
// to chcú 32-bitové ikony Windowsu.
func Render(s Spec) *image.NRGBA {
	if s.Size <= 0 {
		s.Size = 16
	}
	c := newCanvas(s.Size)
	// Všetky rozmery sú v „šestnástkach“ – zapíšeme ich tak, ako by ikona
	// mala 16 px, a k je mierka na skutočnú veľkosť.
	k := float64(s.Size) / 16

	if s.Mode == ModePercent && s.Present {
		drawPercent(c, s, k)
		return c.img
	}
	drawBattery(c, s, k)
	return c.img
}

func drawBattery(c *canvas, s Spec, k float64) {
	fg := s.levelColor()
	outline := s.Theme.FG
	if s.Percent < lowBelow && s.Present {
		outline = fg
	}

	body := box{x0: 0.9 * k, y0: 4.0 * k, x1: 13.1 * k, y1: 12.0 * k, r: 2.0 * k}
	stroke := 1.1 * k
	nub := box{x0: 13.1 * k, y0: 6.4 * k, x1: 14.9 * k, y1: 9.6 * k, r: 0.7 * k}

	c.stroke(body, stroke, outline)
	c.fill(nub, outline)

	if !s.Present {
		// Žiadna batéria – preškrtnutý obrys.
		c.line(2.2*k, 11.0*k, 11.8*k, 5.0*k, 1.2*k, outline)
		return
	}

	// Vnútorná plocha, v ktorej rastie výplň.
	pad := 0.55 * k
	in := box{
		x0: body.x0 + stroke/2 + pad, y0: body.y0 + stroke/2 + pad,
		x1: body.x1 - stroke/2 - pad, y1: body.y1 - stroke/2 - pad,
		r: 0.8 * k,
	}
	frac := clamp(s.Percent/100, 0, 1)
	if frac > 0 {
		w := (in.x1 - in.x0) * frac
		// Aj pri 1 % nech je vidieť aspoň prúžok.
		if w < 1.1*k {
			w = 1.1 * k
		}
		c.fill(box{x0: in.x0, y0: in.y0, x1: in.x0 + w, y1: in.y1, r: math.Min(in.r, w/2)}, fg)
	}

	if s.Charging {
		// Blesk s priehľadným lemom, aby bol čitateľný aj cez výplň.
		// Pri 16 px je blesk vpísaný do obrysu už len škvrna, preto cez
		// obrys zámerne prečnieva – rovnako ako ikona nabíjania vo Windowse.
		// Priehľadný lem ho oddelí od výplne, inak by s ňou splynul.
		b := boltPath(7.0*k, 3.2*k, 6.0*k, 9.6*k)
		c.erasePoly(b, 0.55*k)
		c.fillPoly(b, s.Theme.Accent)
	}
}

func drawPercent(c *canvas, s Spec, k float64) {
	fg := s.textColor()
	n := int(math.Round(clamp(s.Percent, 0, 100)))
	digits := itoa(n)

	// Číslice sa kreslia na celé pixely a s celočíselnou mierkou, inak sú
	// pri 16 px rozmazané a nečitateľné.
	scale := math.Max(1, math.Round(k))
	f := &font5x8
	if len(digits) == 3 {
		f = &font3x5
	}
	gw := float64(f.w) * scale
	gh := float64(f.h) * scale
	gap := scale
	w := gw*float64(len(digits)) + gap*float64(len(digits)-1)

	barH := math.Max(2, math.Round(1.6*k))
	barY1 := float64(c.size) - math.Max(1, math.Round(0.9*k))
	barY0 := barY1 - barH

	x := math.Round((float64(c.size) - w) / 2)
	y := math.Round((barY0 - gh) / 2)
	for i, d := range digits {
		f.draw(c, x+float64(i)*(gw+gap), y, scale, int(d-'0'), fg)
	}

	// Prúžok nabitia pod číslom.
	x0 := math.Round(1.5 * k)
	x1 := float64(c.size) - x0
	track := s.Theme.FG
	track.A = 70
	c.fill(box{x0: x0, y0: barY0, x1: x1, y1: barY1, r: barH / 2}, track)
	frac := clamp(s.Percent/100, 0, 1)
	if wBar := (x1 - x0) * frac; wBar > 0 {
		if wBar < barH {
			wBar = barH
		}
		c.fill(box{x0: x0, y0: barY0, x1: x0 + wBar, y1: barY1, r: barH / 2}, fg)
	}
	if s.Charging {
		// Malý blesk v rohu, aby bolo nabíjanie vidieť aj bez obrysu batérie.
		b := boltPath(float64(c.size)-2.6*k, 0.4*k, 2.6*k, 5.2*k)
		c.erasePoly(b, 0.7*k)
		c.fillPoly(b, s.Theme.Accent)
	}
}

// --- kreslenie ---------------------------------------------------------

type canvas struct {
	img  *image.NRGBA
	size int
}

func newCanvas(size int) *canvas {
	return &canvas{img: image.NewNRGBA(image.Rect(0, 0, size, size)), size: size}
}

// box je zaoblený obdĺžnik.
type box struct{ x0, y0, x1, y1, r float64 }

// sd je znamienková vzdialenosť bodu od tvaru: záporná vnútri.
func (b box) sd(px, py float64) float64 {
	cx, cy := (b.x0+b.x1)/2, (b.y0+b.y1)/2
	hx, hy := (b.x1-b.x0)/2, (b.y1-b.y0)/2
	r := math.Min(b.r, math.Min(hx, hy))
	qx, qy := math.Abs(px-cx)-hx+r, math.Abs(py-cy)-hy+r
	return math.Min(math.Max(qx, qy), 0) + math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) - r
}

func (c *canvas) fill(b box, col color.NRGBA) {
	c.paint(func(x, y float64) float64 { return b.sd(x, y) }, col)
}

func (c *canvas) stroke(b box, width float64, col color.NRGBA) {
	half := width / 2
	c.paint(func(x, y float64) float64 { return math.Abs(b.sd(x, y)) - half }, col)
}

func (c *canvas) line(x0, y0, x1, y1, width float64, col color.NRGBA) {
	half := width / 2
	c.paint(func(x, y float64) float64 { return sdSegment(x, y, x0, y0, x1, y1) - half }, col)
}

func (c *canvas) fillPoly(p []point, col color.NRGBA) {
	c.paint(func(x, y float64) float64 { return sdPolygon(p, x, y) }, col)
}

// erasePoly vyreže z už nakresleného obrazu tvar zväčšený o lem.
func (c *canvas) erasePoly(p []point, grow float64) {
	c.scan(func(x, y float64) float64 { return sdPolygon(p, x, y) - grow }, func(i int, cov float64) {
		a := float64(c.img.Pix[i+3]) * (1 - cov)
		c.img.Pix[i+3] = uint8(math.Round(a))
	})
}

// paint zmieša farbu cez alfu podľa pokrytia pixela.
func (c *canvas) paint(sd func(x, y float64) float64, col color.NRGBA) {
	c.scan(sd, func(i int, cov float64) {
		sa := float64(col.A) / 255 * cov
		if sa <= 0 {
			return
		}
		da := float64(c.img.Pix[i+3]) / 255
		oa := sa + da*(1-sa)
		if oa <= 0 {
			return
		}
		for ch, sc := range [3]float64{float64(col.R), float64(col.G), float64(col.B)} {
			dc := float64(c.img.Pix[i+ch])
			c.img.Pix[i+ch] = uint8(math.Round((sc*sa + dc*da*(1-sa)) / oa))
		}
		c.img.Pix[i+3] = uint8(math.Round(oa * 255))
	})
}

// scan prejde pixely a pre každý dopočíta pokrytie z SDF (analytické
// vyhladenie hrán: hrana prechádza pixelom, pokrytie je jej podiel).
func (c *canvas) scan(sd func(x, y float64) float64, apply func(i int, cov float64)) {
	for y := 0; y < c.size; y++ {
		for x := 0; x < c.size; x++ {
			d := sd(float64(x)+0.5, float64(y)+0.5)
			cov := clamp(0.5-d, 0, 1)
			if cov <= 0 {
				continue
			}
			apply(c.img.PixOffset(x, y), cov)
		}
	}
}

type point struct{ x, y float64 }

// boltPath vráti tvar blesku vpísaný do obdĺžnika so stredom v cx,
// horným okrajom yTop, šírkou w a výškou h.
func boltPath(cx, yTop, w, h float64) []point {
	unit := []point{
		{0.70, 0.00}, {0.00, 0.58}, {0.38, 0.58},
		{0.30, 1.00}, {1.00, 0.42}, {0.62, 0.42},
	}
	out := make([]point, len(unit))
	for i, p := range unit {
		out[i] = point{cx - w/2 + p.x*w, yTop + p.y*h}
	}
	return out
}

func sdSegment(px, py, ax, ay, bx, by float64) float64 {
	pax, pay := px-ax, py-ay
	bax, bay := bx-ax, by-ay
	h := clamp((pax*bax+pay*bay)/(bax*bax+bay*bay), 0, 1)
	return math.Hypot(pax-bax*h, pay-bay*h)
}

// sdPolygon je vzdialenosť od mnohouholníka, záporná vnútri.
func sdPolygon(p []point, px, py float64) float64 {
	d := math.Inf(1)
	inside := false
	for i, j := 0, len(p)-1; i < len(p); j, i = i, i+1 {
		d = math.Min(d, sdSegment(px, py, p[i].x, p[i].y, p[j].x, p[j].y))
		if (p[i].y > py) != (p[j].y > py) &&
			px < (p[j].x-p[i].x)*(py-p[i].y)/(p[j].y-p[i].y)+p[i].x {
			inside = !inside
		}
	}
	if inside {
		return -d
	}
	return d
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
