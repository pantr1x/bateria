package icon

import "math"

// Windows kreslí ikonu batérie v paneli úloh znakom zo systémového písma
// symbolov – vo Windowse 11 je to „Segoe Fluent Icons“, vo Windowse 10
// „Segoe MDL2 Assets“. Keď použijeme ten istý znak, ikona vyzerá presne ako
// tá vstavaná vrátane oblúkov, hrúbky čiar aj blesku pri nabíjaní.
const (
	// FontFluent je písmo symbolov Windowsu 11.
	FontFluent = "Segoe Fluent Icons"
	// FontMDL2 je jeho predchodca vo Windowse 10.
	FontMDL2 = "Segoe MDL2 Assets"
)

// Znaky batérie. Battery0 až Battery10 idú za sebou, rovnako
// BatteryCharging0 až BatteryCharging9 – desiaty stupeň nabíjania je ale
// v tabuľke inde, preto sa uvádza zvlášť.
const (
	glyphBattery0   rune = 0xE850
	glyphCharging0  rune = 0xE85B
	glyphCharging10 rune = 0xE83E
	glyphBatteryOff rune = 0xE996 // „batéria nezistená“
)

// SystemGlyph vráti znak, ktorým Windows kreslí daný stav batérie, a náhradný
// znak pre prípad, že prvý v písme chýba (staršie verzie písma niektoré
// znaky nemajú).
func SystemGlyph(percent float64, charging, present bool) (primary, fallback rune) {
	if !present {
		// Pre „batéria nezistená“ nemá systémové písmo vo všetkých verziách
		// ten istý znak, preto sa v tomto prípade kreslí vlastná ikona.
		return 0, 0
	}
	level := int(math.Round(clamp(percent, 0, 100) / 10))
	if level < 0 {
		level = 0
	}
	if level > 10 {
		level = 10
	}
	if !charging {
		return glyphBattery0 + rune(level), 0
	}
	if level == 10 {
		// Pri plnom nabíjaní skúsime zvláštny znak a ako náhradu deviaty stupeň.
		return glyphCharging10, glyphCharging0 + 9
	}
	return glyphCharging0 + rune(level), glyphBattery0 + rune(level)
}
