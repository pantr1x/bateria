package icon

// Alfa maska je jeden bajt na pixel, riadky idú zhora nadol. Takto z GDI
// prichádza znak vykreslený systémovým písmom symbolov.

// MaskBounds nájde obrys kresby v maske – teda najmenší obdĺžnik, mimo
// ktorého sú už len priehľadné pixely. Vráti false, keď je maska prázdna.
func MaskBounds(mask []byte, w, h int, threshold byte) (minX, minY, maxX, maxY int, ok bool) {
	if w <= 0 || h <= 0 || len(mask) < w*h {
		return 0, 0, 0, 0, false
	}
	minX, minY, maxX, maxY = w, h, -1, -1
	for y := 0; y < h; y++ {
		row := mask[y*w : y*w+w]
		for x, a := range row {
			if a <= threshold {
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < 0 {
		return 0, 0, 0, 0, false
	}
	return minX, minY, maxX, maxY, true
}

// FitMask vystrihne z väčšieho plátna samotnú kresbu a položí ju do stredu
// masky dst×dst. Vracia aj rozmery kresby, aby volajúci vedel, či sa vôbec
// zmestila – ak nie, znak sa dá vykresliť menším písmom a skúsiť znova.
//
// Stred sa počíta z obrysu kresby, nie z riadku písma: riadok je vyšší než
// samotný znak a ikona by v paneli sedela mimo stredu.
func FitMask(src []byte, canvas, dst int) (mask []byte, inkW, inkH int, ok bool) {
	minX, minY, maxX, maxY, found := MaskBounds(src, canvas, canvas, 8)
	if !found || dst <= 0 {
		return nil, 0, 0, false
	}
	inkW, inkH = maxX-minX+1, maxY-minY+1

	out := make([]byte, dst*dst)
	offX := (dst-inkW)/2 - minX
	offY := (dst-inkH)/2 - minY
	for y := 0; y < dst; y++ {
		sy := y - offY
		if sy < 0 || sy >= canvas {
			continue
		}
		for x := 0; x < dst; x++ {
			sx := x - offX
			if sx < 0 || sx >= canvas {
				continue
			}
			out[y*dst+x] = src[sy*canvas+sx]
		}
	}
	return out, inkW, inkH, true
}
