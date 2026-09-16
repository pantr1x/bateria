package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
	"time"
)

func alphaAt(img *image.NRGBA, x, y int) uint8 {
	return img.Pix[img.PixOffset(x, y)+3]
}

func coverage(img *image.NRGBA) float64 {
	var sum float64
	for i := 3; i < len(img.Pix); i += 4 {
		sum += float64(img.Pix[i]) / 255
	}
	return sum
}

func TestRenderSizeAndTransparency(t *testing.T) {
	for _, size := range []int{16, 20, 24, 32} {
		img := Render(Spec{Size: size, Theme: DarkTaskbar(), Percent: 50, Present: true})
		if got := img.Bounds().Dx(); got != size {
			t.Errorf("šírka = %d, chcem %d", got, size)
		}
		if a := alphaAt(img, 0, 0); a != 0 {
			t.Errorf("ľavý horný roh má byť priehľadný, alfa = %d", a)
		}
		if coverage(img) < float64(size)*1.5 {
			t.Errorf("veľkosť %d: nakreslilo sa priveľmi málo bodov (%.1f)", size, coverage(img))
		}
	}
}

// Čím vyššie nabitie, tým väčšia výplň – to je celé posolstvo ikony.
func TestFillGrowsWithPercent(t *testing.T) {
	prev := -1.0
	for _, p := range []float64{0, 10, 35, 70, 100} {
		img := Render(Spec{Size: 32, Theme: DarkTaskbar(), Percent: p, Present: true})
		c := coverage(img)
		if c <= prev {
			t.Errorf("pri %.0f %% je pokrytie %.1f, pri predchádzajúcom kroku %.1f", p, c, prev)
		}
		prev = c
	}
}

func TestChargingDiffersFromIdle(t *testing.T) {
	a := Render(Spec{Size: 16, Theme: DarkTaskbar(), Percent: 50, Present: true})
	b := Render(Spec{Size: 16, Theme: DarkTaskbar(), Percent: 50, Present: true, Charging: true})
	same := true
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("ikona pri nabíjaní musí vyzerať inak (blesk)")
	}
}

func TestPercentModeDrawsDigits(t *testing.T) {
	for _, p := range []float64{7, 42, 100} {
		img := Render(Spec{Size: 16, Mode: ModePercent, Theme: DarkTaskbar(), Percent: p, Present: true})
		if coverage(img) < 10 {
			t.Errorf("pri %.0f %% sa číslo nevykreslilo (pokrytie %.1f)", p, coverage(img))
		}
	}
}

func TestEncodeResourceHeader(t *testing.T) {
	img := Render(Spec{Size: 16, Theme: DarkTaskbar(), Percent: 50, Present: true})
	data := EncodeResource(img)
	le := binary.LittleEndian
	if got := le.Uint32(data[0:]); got != 40 {
		t.Errorf("biSize = %d, chcem 40", got)
	}
	if got := le.Uint32(data[4:]); got != 16 {
		t.Errorf("biWidth = %d, chcem 16", got)
	}
	if got := le.Uint32(data[8:]); got != 32 {
		t.Errorf("biHeight = %d, chcem 32 (dvojnásobok kvôli maske)", got)
	}
	if got := le.Uint16(data[14:]); got != 32 {
		t.Errorf("biBitCount = %d, chcem 32", got)
	}
	want := 40 + 16*16*4 + 4*16
	if len(data) != want {
		t.Errorf("dĺžka = %d, chcem %d", len(data), want)
	}
}

// Prvý riadok dát je v DIB spodný riadok obrázka – ľahko sa to pomýli.
func TestEncodeResourceIsBottomUpBGRA(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	set := func(x, y int, r, g, b, a uint8) {
		i := img.PixOffset(x, y)
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, a
	}
	set(0, 0, 10, 20, 30, 255) // vľavo hore
	set(0, 1, 40, 50, 60, 128) // vľavo dole
	data := EncodeResource(img)
	first := data[40 : 40+4]
	if first[0] != 60 || first[1] != 50 || first[2] != 40 || first[3] != 128 {
		t.Errorf("prvý pixel = %v, chcem BGRA spodného riadka {60 50 40 128}", first)
	}
}

// Veľké obrázky sa v .ico ukladajú ako PNG, inak by súbor zbytočne narástol.
func TestEncodeICOUsesPNGForLargeSizes(t *testing.T) {
	small := Render(Spec{Size: 32, Theme: DarkTaskbar(), Percent: 50, Present: true})
	large := Render(Spec{Size: 256, Theme: DarkTaskbar(), Percent: 50, Present: true})
	data := EncodeICO(small, large)
	le := binary.LittleEndian

	offSmall := le.Uint32(data[6+12:])
	if got := le.Uint32(data[offSmall:]); got != 40 {
		t.Errorf("32 px sa mal uložiť ako DIB, začína %d", got)
	}
	offLarge := le.Uint32(data[6+16+12:])
	pngMagic := []byte{0x89, 'P', 'N', 'G'}
	if !bytes.Equal(data[offLarge:offLarge+4], pngMagic) {
		t.Errorf("256 px sa mal uložiť ako PNG, začína % x", data[offLarge:offLarge+4])
	}
	if e := data[6+16:]; e[0] != 0 {
		t.Errorf("256 px sa v hlavičke zapisuje ako 0, je %d", e[0])
	}
	if len(data) > 100*1024 {
		t.Errorf("súbor má %d bajtov, to je zbytočne veľa", len(data))
	}
}

func TestEncodeICOStructure(t *testing.T) {
	a := Render(Spec{Size: 16, Theme: DarkTaskbar(), Percent: 50, Present: true})
	b := Render(Spec{Size: 32, Theme: DarkTaskbar(), Percent: 50, Present: true})
	data := EncodeICO(a, b)
	le := binary.LittleEndian
	if le.Uint16(data[2:]) != 1 {
		t.Error("typ súboru má byť 1 (ikona)")
	}
	if n := le.Uint16(data[4:]); n != 2 {
		t.Fatalf("počet obrázkov = %d, chcem 2", n)
	}
	for i, wantSize := range []byte{16, 32} {
		e := data[6+16*i:]
		if e[0] != wantSize {
			t.Errorf("obrázok %d má šírku %d, chcem %d", i, e[0], wantSize)
		}
		off, ln := le.Uint32(e[12:]), le.Uint32(e[8:])
		if int(off+ln) > len(data) {
			t.Errorf("obrázok %d ukazuje mimo súboru (%d+%d > %d)", i, off, ln, len(data))
		}
		if got := le.Uint32(data[off:]); got != 40 {
			t.Errorf("obrázok %d nezačína hlavičkou DIB", i)
		}
	}
}

func TestMaskImageColorsAndAlpha(t *testing.T) {
	mask := []byte{0, 128, 255, 64}
	img := MaskImage(mask, 2, color.NRGBA{10, 20, 30, 255})
	if got := img.NRGBAAt(0, 0); got.A != 0 {
		t.Errorf("nulová maska má byť priehľadná, je %v", got)
	}
	got := img.NRGBAAt(1, 0)
	if got.R != 10 || got.G != 20 || got.B != 30 || got.A != 128 {
		t.Errorf("pixel = %v, chcem farbu {10 20 30} s alfou 128", got)
	}
	if a := img.NRGBAAt(0, 1).A; a != 255 {
		t.Errorf("plná maska má mať alfu 255, má %d", a)
	}
}

func TestMaskImageTooShortIsSafe(t *testing.T) {
	img := MaskImage([]byte{1, 2}, 4, color.NRGBA{255, 255, 255, 255})
	if img.Bounds().Dx() != 4 {
		t.Errorf("aj pri krátkej maske má vzniknúť obrázok 4×4")
	}
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			t.Error("krátka maska sa nesmie čítať mimo rozsahu")
			break
		}
	}
}

func TestTimeText(t *testing.T) {
	cases := []struct {
		remaining time.Duration
		percent   float64
		want      string
	}{
		{2*time.Hour + 13*time.Minute, 60, "2:13"},
		{45 * time.Minute, 30, "0:45"},
		{9*time.Hour + 59*time.Minute, 90, "9:59"},
		{10 * time.Hour, 95, "10h"},
		{12*time.Hour + 30*time.Minute, 99, "12h"},
		{0, 73, "73%"}, // odhad zatiaľ nie je
		{-time.Minute, 5, "5%"},
		{90 * time.Second, 50, "0:02"}, // zaokrúhľuje sa na minúty
	}
	for _, c := range cases {
		if got := TimeText(c.remaining, c.percent); got != c.want {
			t.Errorf("TimeText(%v, %.0f) = %q, chcem %q", c.remaining, c.percent, got, c.want)
		}
	}
}

// Text sa musí zmestiť do ikony aj v tej najmenšej veľkosti.
func TestTimeTextFitsInIcon(t *testing.T) {
	for _, size := range []int{16, 20, 24, 32} {
		for _, text := range []string{"2:13", "10h", "100", "73%", "0:45"} {
			f, scale := fitText(text, float64(size)-2, float64(size)-5)
			if f == nil {
				t.Errorf("veľkosť %d: text %q sa nezmestil", size, text)
				continue
			}
			if w := float64(f.width(text)) * scale; w > float64(size)-2 {
				t.Errorf("veľkosť %d: text %q má šírku %.0f", size, text, w)
			}
		}
	}
}

func TestTimeModeDrawsSomething(t *testing.T) {
	img := Render(Spec{Size: 16, Mode: ModeTime, Theme: DarkTaskbar(),
		Percent: 60, Present: true, Remaining: 2*time.Hour + 13*time.Minute})
	if coverage(img) < 10 {
		t.Errorf("čas sa nevykreslil (pokrytie %.1f)", coverage(img))
	}
}
