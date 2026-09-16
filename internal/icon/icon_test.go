package icon

import (
	"encoding/binary"
	"image"
	"testing"
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
