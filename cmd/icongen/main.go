// Command icongen vygeneruje ikonu aplikácie (assets/app.ico) a obrázok
// s náhľadmi všetkých stavov (docs/ikony.png). Beží na ľubovoľnej platforme.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/pantr1x/bateria/internal/icon"
)

func main() {
	out := flag.String("out", ".", "koreňový priečinok projektu")
	flag.Parse()

	if err := writeAppIcon(filepath.Join(*out, "assets", "app.ico")); err != nil {
		log.Fatal(err)
	}
	if err := writePreview(filepath.Join(*out, "docs", "ikony.png")); err != nil {
		log.Fatal(err)
	}
}

func writeAppIcon(path string) error {
	var imgs []*image.NRGBA
	for _, size := range []int{16, 20, 24, 32, 48, 64, 128, 256} {
		imgs = append(imgs, icon.Render(icon.Spec{
			Size: size, Theme: icon.DarkTaskbar(), Percent: 72, Present: true, Charging: true,
		}))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, icon.EncodeICO(imgs...), 0o644)
}

type shot struct {
	spec  icon.Spec
	label string
}

// writePreview vyskladá kontaktný list: každý stav na tmavom aj svetlom
// paneli, zväčšený, aby bolo vidieť, čo robí vyhladzovanie hrán.
func writePreview(path string) error {
	states := []struct {
		percent  float64
		charging bool
		present  bool
	}{
		{100, false, true}, {72, true, true}, {55, false, true},
		{35, true, true}, {15, false, true}, {6, false, true}, {0, false, false},
	}

	const (
		zoom = 6
		cell = 16*zoom + 12
	)
	rows := []struct {
		theme icon.Theme
		bg    color.NRGBA
		mode  icon.Mode
	}{
		{icon.DarkTaskbar(), color.NRGBA{32, 32, 32, 255}, icon.ModeBattery},
		{icon.LightTaskbar(), color.NRGBA{243, 243, 243, 255}, icon.ModeBattery},
		{icon.DarkTaskbar(), color.NRGBA{32, 32, 32, 255}, icon.ModePercent},
		{icon.LightTaskbar(), color.NRGBA{243, 243, 243, 255}, icon.ModePercent},
	}

	w := cell * len(states)
	h := cell * len(rows)
	sheet := image.NewNRGBA(image.Rect(0, 0, w, h))
	for r, row := range rows {
		draw.Draw(sheet, image.Rect(0, r*cell, w, (r+1)*cell), &image.Uniform{row.bg}, image.Point{}, draw.Src)
		for c, st := range states {
			img := icon.Render(icon.Spec{
				Size: 16, Mode: row.mode, Theme: row.theme,
				Percent: st.percent, Charging: st.charging, Present: st.present,
			})
			// Zväčšenie bez interpolácie – nech je vidieť skutočné pixely.
			for y := 0; y < 16*zoom; y++ {
				for x := 0; x < 16*zoom; x++ {
					sheet.Set(c*cell+6+x, r*cell+6+y, blend(row.bg, img.NRGBAAt(x/zoom, y/zoom)))
				}
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, sheet)
}

func blend(bg, fg color.NRGBA) color.NRGBA {
	a := float64(fg.A) / 255
	mix := func(f, b uint8) uint8 { return uint8(float64(f)*a + float64(b)*(1-a)) }
	return color.NRGBA{mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B), 255}
}
