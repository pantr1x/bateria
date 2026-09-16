//go:build windows

package win

import "unsafe"

// Tento súbor vykresľuje znak zo systémového písma symbolov do alfa masky.
//
// GDI pri kreslení textu nevypĺňa alfa kanál – keby sme text nakreslili
// rovno do 32-bitového obrázka, ikona by bola celá priehľadná. Preto sa
// znak nakreslí bielou na čierne pozadie a alfa sa odvodí z jasu pixela.
// Pri symbolových písmach to sedí presne: znak je jednofarebná silueta.

var (
	procCreateDIBSection = gdi32.proc("CreateDIBSection")
	procTextOut          = gdi32.proc("TextOutW")
	procGetGlyphIndices  = gdi32.proc("GetGlyphIndicesW")
	procGdiFlush         = gdi32.proc("GdiFlush")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

const (
	antialiasedQuality = 4
	defaultCharset     = 1
	symbolCharset      = 2

	ggiMarkNonexistingGlyphs = 1
	glyphNotFound            = 0xFFFF
	gdiError                 = 0xFFFFFFFF
)

// GlyphAlpha vykreslí znak písma veľkosťou em do štvorcového plátna a vráti
// jeho alfa masku (jeden bajt na pixel, riadky zhora nadol). Znak sa kreslí
// od tretiny plátna, takže okolo neho zostáva miesto a dá sa spoľahlivo
// nájsť jeho skutočný obrys.
//
// Vráti false, keď písmo alebo znak v systéme nie sú – volajúci potom siahne
// po vlastnej kreslenej ikone.
func GlyphAlpha(face string, r rune, em, canvas int32) ([]byte, bool) {
	if em <= 0 || canvas <= 0 || canvas > 2048 {
		return nil, false
	}
	// Niektoré verzie písma sú registrované ako symbolové, iné ako bežné.
	for _, charset := range []uint8{defaultCharset, symbolCharset} {
		if m, ok := glyphAlpha(face, r, em, canvas, charset); ok {
			return m, true
		}
	}
	return nil, false
}

func glyphAlpha(face string, r rune, em, canvas int32, charset uint8) ([]byte, bool) {
	dc, _ := procCreateCompatDC.Call(0)
	if dc == 0 {
		return nil, false
	}
	defer procDeleteDC.Call(dc)

	bmp, bits := createDIBSection(dc, canvas, canvas)
	if bmp == 0 || bits == nil {
		return nil, false
	}
	defer procDeleteObject.Call(bmp)
	oldBmp, _ := procSelectObject.Call(dc, bmp)
	defer procSelectObject.Call(dc, oldBmp)

	font := newSymbolFont(face, em, charset)
	if font == 0 {
		return nil, false
	}
	defer procDeleteObject.Call(font)
	oldFont, _ := procSelectObject.Call(dc, font)
	defer procSelectObject.Call(dc, oldFont)

	if !glyphExists(dc, r) {
		return nil, false
	}

	procSetBkMode.Call(dc, transparent)
	procSetTextColor.Call(dc, 0x00FFFFFF)
	ch := uint16(r)
	origin := canvas / 3
	procTextOut.Call(dc, uintptr(origin), uintptr(origin), uintptr(unsafe.Pointer(&ch)), 1)
	// Bez vyprázdnenia fronty GDI by sme čítali ešte nedokreslený obrázok.
	procGdiFlush.Call()

	n := int(canvas) * int(canvas)
	px := unsafe.Slice((*byte)(bits), n*4)
	alpha := make([]byte, n)
	for i := 0; i < n; i++ {
		b, g, r8 := px[i*4], px[i*4+1], px[i*4+2]
		a := b
		if g > a {
			a = g
		}
		if r8 > a {
			a = r8
		}
		alpha[i] = a
	}
	return alpha, true
}

func createDIBSection(dc uintptr, w, h int32) (uintptr, unsafe.Pointer) {
	bi := bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    w,
		Height:   -h, // záporná výška = riadky zhora nadol
		Planes:   1,
		BitCount: 32,
	}
	var bits unsafe.Pointer
	const dibRGBColors = 0
	bmp, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	return bmp, bits
}

func newSymbolFont(face string, em int32, charset uint8) uintptr {
	lf := logFont{
		Height:  -em,
		Weight:  400,
		CharSet: charset,
		// Vyhladzovanie v odtieňoch sivej: ClearType by pridal farebné okraje,
		// ktoré by po prevode na alfu vyzerali ako farebný lem.
		Quality: antialiasedQuality,
	}
	CopyStr(lf.FaceName[:], face)
	h, _ := procCreateFontIndir.Call(uintptr(unsafe.Pointer(&lf)))
	return h
}

// glyphExists overí, či písmo daný znak naozaj má. Keby chýbal, GDI by
// nakreslil prázdny obdĺžnik a používateľ by v paneli videl štvorček.
func glyphExists(dc uintptr, r rune) bool {
	ch := uint16(r)
	idx := uint16(glyphNotFound)
	ret, _ := procGetGlyphIndices.Call(dc, uintptr(unsafe.Pointer(&ch)), 1,
		uintptr(unsafe.Pointer(&idx)), ggiMarkNonexistingGlyphs)
	if uint32(ret) == gdiError {
		return false
	}
	return idx != glyphNotFound
}
