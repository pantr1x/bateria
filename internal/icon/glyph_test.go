package icon

import "testing"

func TestSystemGlyphLevels(t *testing.T) {
	cases := []struct {
		percent  float64
		charging bool
		want     rune
	}{
		{0, false, 0xE850},
		{4, false, 0xE850}, // zaokrúhľuje sa na najbližší desiatok
		{5, false, 0xE851},
		{50, false, 0xE855},
		{99, false, 0xE85A},
		{100, false, 0xE85A},
		{0, true, 0xE85B},
		{50, true, 0xE860},
		{94, true, 0xE864},
		{100, true, 0xE83E}, // desiaty stupeň nabíjania je v tabuľke inde
	}
	for _, c := range cases {
		got, _ := SystemGlyph(c.percent, c.charging, true)
		if got != c.want {
			t.Errorf("SystemGlyph(%.0f, nabíja=%v) = %#x, chcem %#x",
				c.percent, c.charging, got, c.want)
		}
	}
}

func TestSystemGlyphStaysInRange(t *testing.T) {
	for p := -50.0; p <= 150; p += 3 {
		for _, charging := range []bool{false, true} {
			g, _ := SystemGlyph(p, charging, true)
			lo, hi := glyphBattery0, glyphBattery0+10
			if charging {
				lo, hi = glyphCharging0, glyphCharging0+9
				if g == glyphCharging10 {
					continue
				}
			}
			if g < lo || g > hi {
				t.Errorf("pri %.0f %% (nabíja=%v) vyšiel znak %#x mimo rozsahu", p, charging, g)
			}
		}
	}
}

// Bez batérie sa systémový znak nepoužíva – kreslí sa vlastná ikona.
func TestSystemGlyphNoBattery(t *testing.T) {
	if g, fb := SystemGlyph(0, false, false); g != 0 || fb != 0 {
		t.Errorf("bez batérie = %#x/%#x, chcem 0/0", g, fb)
	}
}
