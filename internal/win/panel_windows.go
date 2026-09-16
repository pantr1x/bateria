//go:build windows

package win

import "unsafe"

// Komunikácia s programom bateria-panel.exe, ktorý kreslí text v paneli
// úloh. Posiela sa mu hotový text – sám nič nepočíta.

// PanelClass je trieda okna, ktoré panel počúva.
const PanelClass = "BateriaPanelHost"

// PanelEventMessage je meno správy, ktorou panel hlási späť udalosti.
const PanelEventMessage = "BateriaPanelEvent"

// Udalosti od panela.
const (
	PanelEventMenu   = 1 // kliknutie pravým tlačidlom na text
	PanelEventOffset = 2 // používateľ text posunul, v lParam je nové odsadenie
)

const (
	panelUpdateVersion = 1
	panelFlagQuit      = 1

	wmCopyData      = 0x004A
	smtoAbortIfHung = 0x0002
)

// panelUpdate musí presne zodpovedať štruktúre PanelUpdate v panel.cpp.
type panelUpdate struct {
	Version   int32
	Offset    int32
	TextColor uint32
	Flags     uint32
	Text      [64]uint16
}

// Kontrola pri preklade: štruktúra musí mať presne toľko bajtov ako
// PanelUpdate v panel/panel.cpp. Keby sa rozišli, panel by čítal nezmysly.
const (
	_ = uint(unsafe.Sizeof(panelUpdate{}) - 144)
	_ = uint(144 - unsafe.Sizeof(panelUpdate{}))
)

type copyDataStruct struct {
	Data  uintptr
	Size  uint32
	Bytes uintptr
}

var procSendMessageTimeout = user32.proc("SendMessageTimeoutW")

// PanelRunning hovorí, či program s textom beží.
func PanelRunning() bool { return FindWindow(PanelClass) != 0 }

// UpdatePanel pošle panelu text, farbu a odsadenie od pravého okraja.
// Odsadenie -1 znamená „nechaj, ako je“. Vráti false, keď panel nebeží.
func UpdatePanel(text string, color Color, offset int32) bool {
	return sendPanel(text, color, offset, 0)
}

// ClosePanel požiada panel, aby sa ukončil.
func ClosePanel() { sendPanel("", 0, -1, panelFlagQuit) }

func sendPanel(text string, color Color, offset int32, flags uint32) bool {
	hwnd := FindWindow(PanelClass)
	if hwnd == 0 {
		return false
	}
	u := panelUpdate{
		Version:   panelUpdateVersion,
		Offset:    offset,
		TextColor: uint32(color),
		Flags:     flags,
	}
	CopyStr(u.Text[:], text)

	cds := copyDataStruct{
		Size:  uint32(unsafe.Sizeof(u)),
		Bytes: uintptr(unsafe.Pointer(&u)),
	}
	var result uintptr
	// S časovým limitom: keby panel neodpovedal, aplikácia sa na ňom
	// nesmie zaseknúť.
	r, _ := procSendMessageTimeout.Call(hwnd, wmCopyData, 0,
		uintptr(unsafe.Pointer(&cds)), smtoAbortIfHung, 300,
		uintptr(unsafe.Pointer(&result)))
	// Panel odpovie jednotkou, len keď má v paneli úloh skutočné okno.
	// Keby sa vytvoriť nepodarilo, aplikácia to musí vedieť a ukázať čas
	// aspoň v ikone.
	return r != 0 && result != 0
}
