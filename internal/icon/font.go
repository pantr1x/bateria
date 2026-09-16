package icon

import "image/color"

// font je jednoduché bitmapové písmo. Pri 16 pixeloch nemá vyhladzovanie
// zmysel – text je čitateľný, len keď leží presne na mriežke pixelov.
// Znaky môžu mať rôznu šírku (dvojbodka je úzka), preto si šírku každý
// znak nesie sám v dĺžke svojich riadkov.
type font struct {
	h      int
	glyphs map[rune][]string
}

// font3x5 je najmenšie čitateľné písmo, používa sa pri 16 px.
var font3x5 = font{h: 5, glyphs: map[rune][]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", "###", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", "..#", "..#", "..#"},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	':': {".", "#", ".", "#", "."},
	'h': {"#..", "#..", "##.", "#.#", "#.#"},
	'%': {"#.#", "..#", ".#.", "#..", "#.#"},
}}

// font5x8 sa používa, keď je na ikonu viac miesta (vyššie rozlíšenie).
var font5x8 = font{h: 8, glyphs: map[rune][]string{
	'0': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'2': {".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
	'3': {"####.", "....#", "....#", ".###.", "....#", "....#", "....#", "####."},
	'4': {"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#.", "...#."},
	'5': {"#####", "#....", "#....", "####.", "....#", "....#", "#...#", ".###."},
	'6': {".###.", "#...#", "#....", "####.", "#...#", "#...#", "#...#", ".###."},
	'7': {"#####", "....#", "...#.", "..#..", "..#..", ".#...", ".#...", ".#..."},
	'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", "#...#", ".###."},
	'9': {".###.", "#...#", "#...#", "#...#", ".####", "....#", "#...#", ".###."},
	':': {".", ".", "#", ".", ".", "#", ".", "."},
	'h': {"#...", "#...", "#...", "###.", "#..#", "#..#", "#..#", "#..#"},
	'%': {"##..#", "##.#.", "..#..", ".#...", "#..##", "....#", "#..##", "#..##"},
}}

// width vráti šírku textu v bodoch písma (vrátane medzier medzi znakmi).
func (f *font) width(s string) int {
	w, first := 0, true
	for _, r := range s {
		g, ok := f.glyphs[r]
		if !ok || len(g) == 0 {
			continue
		}
		if !first {
			w++ // medzera medzi znakmi
		}
		first = false
		w += len(g[0])
	}
	return w
}

// draw vykreslí text tak, že každý bod písma je malý štvorec. Súradnice by
// mali byť celé čísla, inak sa text rozmaže.
func (f *font) draw(c *canvas, x, y, scale float64, s string, col color.NRGBA) {
	cursor := x
	first := true
	for _, r := range s {
		g, ok := f.glyphs[r]
		if !ok || len(g) == 0 {
			continue
		}
		if !first {
			cursor += scale
		}
		first = false
		for row, line := range g {
			for i, ch := range line {
				if ch != '#' {
					continue
				}
				px := cursor + float64(i)*scale
				py := y + float64(row)*scale
				c.fill(box{x0: px, y0: py, x1: px + scale, y1: py + scale}, col)
			}
		}
		cursor += float64(len(g[0])) * scale
	}
}

// fitText vyberie najväčšie písmo a mierku, pri ktorých sa text ešte zmestí
// do daného priestoru. Vráti nil, keď sa nezmestí ani to najmenšie.
func fitText(s string, maxW, maxH float64) (*font, float64) {
	var best *font
	var bestScale float64
	for _, f := range []*font{&font3x5, &font5x8} {
		for scale := 1.0; scale <= 6; scale++ {
			w := float64(f.width(s)) * scale
			h := float64(f.h) * scale
			if w > maxW || h > maxH {
				break
			}
			if best == nil || h > float64(best.h)*bestScale {
				best, bestScale = f, scale
			}
		}
	}
	return best, bestScale
}
