package icon

import "testing"

// plátno s kresbou v danom obdĺžniku
func canvasWithInk(canvas, x0, y0, w, h int) []byte {
	m := make([]byte, canvas*canvas)
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			m[y*canvas+x] = 255
		}
	}
	return m
}

func TestMaskBounds(t *testing.T) {
	m := canvasWithInk(10, 2, 3, 4, 5)
	minX, minY, maxX, maxY, ok := MaskBounds(m, 10, 10, 8)
	if !ok {
		t.Fatal("obrys sa nenašiel")
	}
	if minX != 2 || minY != 3 || maxX != 5 || maxY != 7 {
		t.Errorf("obrys = (%d,%d)-(%d,%d), chcem (2,3)-(5,7)", minX, minY, maxX, maxY)
	}
}

func TestMaskBoundsEmpty(t *testing.T) {
	if _, _, _, _, ok := MaskBounds(make([]byte, 100), 10, 10, 8); ok {
		t.Error("prázdna maska nemá obrys")
	}
}

func TestMaskBoundsIgnoresNoise(t *testing.T) {
	m := make([]byte, 100)
	m[55] = 5 // slabý pixel pod prahom
	if _, _, _, _, ok := MaskBounds(m, 10, 10, 8); ok {
		t.Error("pixely pod prahom sa nemajú počítať do obrysu")
	}
}

// Kresba mimo stredu plátna musí skončiť v strede ikony.
func TestFitMaskCentersInk(t *testing.T) {
	// plátno 30×30, kresba 6×4 hore vľavo
	src := canvasWithInk(30, 1, 2, 6, 4)
	mask, inkW, inkH, ok := FitMask(src, 30, 10)
	if !ok {
		t.Fatal("orezanie zlyhalo")
	}
	if inkW != 6 || inkH != 4 {
		t.Fatalf("rozmery kresby = %d×%d, chcem 6×4", inkW, inkH)
	}
	minX, minY, maxX, maxY, found := MaskBounds(mask, 10, 10, 8)
	if !found {
		t.Fatal("v ikone nie je kresba")
	}
	if minX != 2 || maxX != 7 {
		t.Errorf("vodorovne (%d–%d), chcem (2–7)", minX, maxX)
	}
	if minY != 3 || maxY != 6 {
		t.Errorf("zvisle (%d–%d), chcem (3–6)", minY, maxY)
	}
}

// Keď je znak väčší než ikona, výsledok sa oreže a volajúci sa to dozvie
// z rozmerov kresby – vtedy má znak prekresliť menším písmom.
func TestFitMaskReportsOversizedInk(t *testing.T) {
	src := canvasWithInk(30, 5, 5, 20, 18)
	mask, inkW, inkH, ok := FitMask(src, 30, 10)
	if !ok {
		t.Fatal("orezanie zlyhalo")
	}
	if inkW != 20 || inkH != 18 {
		t.Errorf("rozmery kresby = %d×%d, chcem 20×18", inkW, inkH)
	}
	if len(mask) != 100 {
		t.Errorf("maska má %d bajtov, chcem 100", len(mask))
	}
	// Orezanie nesmie siahnuť mimo poľa – to overí už samotný beh testu.
	for _, v := range mask {
		if v != 255 {
			t.Fatal("vnútro veľkej kresby má byť celé vyplnené")
		}
	}
}

func TestFitMaskEmptyCanvas(t *testing.T) {
	if _, _, _, ok := FitMask(make([]byte, 900), 30, 10); ok {
		t.Error("z prázdneho plátna nemá vzniknúť ikona")
	}
}

func TestFitMaskOddSizes(t *testing.T) {
	src := canvasWithInk(21, 4, 4, 3, 7)
	mask, _, _, ok := FitMask(src, 21, 7)
	if !ok {
		t.Fatal("orezanie zlyhalo")
	}
	if len(mask) != 49 {
		t.Fatalf("maska má %d bajtov, chcem 49", len(mask))
	}
	minX, minY, maxX, maxY, _ := MaskBounds(mask, 7, 7, 8)
	if minX != 2 || maxX != 4 || minY != 0 || maxY != 6 {
		t.Errorf("kresba (%d,%d)-(%d,%d), chcem (2,0)-(4,6)", minX, minY, maxX, maxY)
	}
}
