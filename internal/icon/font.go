package icon

import "image/color"

// font je jednoduché bitmapové písmo pre číslice. Pri 16 pixeloch nemá
// vyhladzovanie zmysel – číslo je čitateľné, len keď leží na mriežke pixelov.
type font struct {
	w, h  int
	glyph [10][]string
}

// font5x8 sa používa pre jedno- a dvojciferné čísla.
var font5x8 = font{w: 5, h: 8, glyph: [10][]string{
	{".###.", "#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	{"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	{".###.", "#...#", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
	{"####.", "....#", "....#", ".###.", "....#", "....#", "....#", "####."},
	{"...#.", "..##.", ".#.#.", "#..#.", "#####", "...#.", "...#.", "...#."},
	{"#####", "#....", "#....", "####.", "....#", "....#", "#...#", ".###."},
	{".###.", "#...#", "#....", "####.", "#...#", "#...#", "#...#", ".###."},
	{"#####", "....#", "...#.", "..#..", "..#..", ".#...", ".#...", ".#..."},
	{".###.", "#...#", "#...#", ".###.", "#...#", "#...#", "#...#", ".###."},
	{".###.", "#...#", "#...#", "#...#", ".####", "....#", "#...#", ".###."},
}}

// font3x5 sa používa pre 100 %, kde na širšie číslice nie je miesto.
var font3x5 = font{w: 3, h: 5, glyph: [10][]string{
	{"###", "#.#", "#.#", "#.#", "###"},
	{".#.", "##.", ".#.", ".#.", "###"},
	{"###", "..#", "###", "#..", "###"},
	{"###", "..#", "###", "..#", "###"},
	{"#.#", "#.#", "###", "..#", "..#"},
	{"###", "#..", "###", "..#", "###"},
	{"###", "#..", "###", "#.#", "###"},
	{"###", "..#", "..#", "..#", "..#"},
	{"###", "#.#", "###", "#.#", "###"},
	{"###", "#.#", "###", "..#", "###"},
}}

// draw vykreslí číslicu tak, že každý jej bod je malý obdĺžnik.
func (f *font) draw(c *canvas, x, y, scale float64, digit int, col color.NRGBA) {
	if digit < 0 || digit > 9 {
		return
	}
	for row, line := range f.glyph[digit] {
		for col2, ch := range line {
			if ch != '#' {
				continue
			}
			px := x + float64(col2)*scale
			py := y + float64(row)*scale
			c.fill(box{x0: px, y0: py, x1: px + scale, y1: py + scale}, col)
		}
	}
}
