package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
)

// pngFromSize je veľkosť, od ktorej sa obrázok v súbore .ico ukladá ako PNG.
// Nekomprimovaný 256×256 by sám zabral 256 kB.
const pngFromSize = 64

// EncodeResource zapíše obrázok v tvare, v akom ikonu očakáva Windows:
// BITMAPINFOHEADER s dvojnásobnou výškou, potom farebné dáta (BGRA zdola
// nahor) a nakoniec 1-bitová maska. Presne tieto bajty berie
// CreateIconFromResourceEx, takže ikonu vieme vyrobiť v pamäti bez súboru.
func EncodeResource(img *image.NRGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	xorSize := w * h * 4
	maskStride := ((w + 31) / 32) * 4
	andSize := maskStride * h

	out := make([]byte, 40+xorSize+andSize)
	le := binary.LittleEndian
	le.PutUint32(out[0:], 40)               // biSize
	le.PutUint32(out[4:], uint32(w))        // biWidth
	le.PutUint32(out[8:], uint32(2*h))      // biHeight = farba + maska
	le.PutUint16(out[12:], 1)               // biPlanes
	le.PutUint16(out[14:], 32)              // biBitCount
	le.PutUint32(out[16:], 0)               // biCompression = BI_RGB
	le.PutUint32(out[20:], uint32(xorSize)) // biSizeImage

	p := 40
	for y := h - 1; y >= 0; y-- { // DIB ide zdola nahor
		row := img.Pix[img.PixOffset(b.Min.X, b.Min.Y+y):]
		for x := 0; x < w; x++ {
			r, g, bl, a := row[x*4], row[x*4+1], row[x*4+2], row[x*4+3]
			out[p], out[p+1], out[p+2], out[p+3] = bl, g, r, a
			p += 4
		}
	}
	// Maska zostáva nulová: priehľadnosť riešime alfa kanálom.
	return out
}

// EncodeICO zabalí obrázky do súboru .ico. Používa sa pre ikonu aplikácie
// a na náhľady; samotný panel úloh dostáva ikonu cez EncodeResource.
func EncodeICO(imgs ...*image.NRGBA) []byte {
	le := binary.LittleEndian
	head := make([]byte, 6+16*len(imgs))
	le.PutUint16(head[0:], 0)                 // rezervované
	le.PutUint16(head[2:], 1)                 // typ 1 = ikona
	le.PutUint16(head[4:], uint16(len(imgs))) // počet obrázkov

	body := []byte{}
	offset := len(head)
	for i, img := range imgs {
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		data := EncodeResource(img)
		if w >= pngFromSize {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err == nil {
				data = buf.Bytes()
			}
		}
		e := head[6+16*i:]
		e[0] = byte(w % 256) // 256 sa zapisuje ako 0
		e[1] = byte(h % 256)
		e[2], e[3] = 0, 0
		le.PutUint16(e[4:], 1)
		le.PutUint16(e[6:], 32)
		le.PutUint32(e[8:], uint32(len(data)))
		le.PutUint32(e[12:], uint32(offset))
		body = append(body, data...)
		offset += len(data)
	}
	return append(head, body...)
}
