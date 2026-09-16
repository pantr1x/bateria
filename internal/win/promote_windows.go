//go:build windows

package win

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

// Windows 11 si viditeľnosť ikon v oznamovacej oblasti pamätá v registri
// pod „Control Panel\NotifyIconSettings“. Každá aplikácia tam má podkľúč
// s cestou k programu a hodnotou IsPromoted: 1 znamená ikonu priamo
// v paneli úloh, 0 alebo chýbajúca hodnota znamená schovanie do prepadovej
// ponuky pod šípkou. Nové ikony Windows schováva predvolene, preto sa
// aplikácia po prvom spustení „stratí“.
//
// Podkľúč vytvára samotný Prieskumník, keď ikonu prvýkrát uvidí. Preto sa
// hľadá podľa cesty k programu a nastavuje sa až chvíľu po pridaní ikony.

const notifyIconSettingsKey = `Control Panel\NotifyIconSettings`

// errNoMoreItems ukončuje prechádzanie podkľúčov.
const errNoMoreItems = syscall.Errno(259)

// ErrTrayPromotionUnsupported znamená, že tento Windows viditeľnosť ikon
// v registri takto nedrží (Windows 10 a staršie).
var ErrTrayPromotionUnsupported = errors.New("tento Windows nastavenie viditeľnosti ikon v registri nemá")

// ErrTrayEntryNotFound znamená, že Prieskumník o ikone ešte nevie.
var ErrTrayEntryNotFound = errors.New("Windows si ikonu tejto aplikácie zatiaľ nezapamätal")

// SetTrayIconPromoted zapne alebo vypne trvalé zobrazenie ikony v paneli
// úloh. Mení len záznam patriaci k zadanému programu, ničoho iného sa
// nedotkne.
func SetTrayIconPromoted(exePath string, on bool) error {
	root, name, err := findTrayEntry(exePath)
	if err != nil {
		return err
	}
	defer syscall.RegCloseKey(root)
	return setPromoted(root, name, on)
}

// TrayIconPromoted povie, či je ikona nastavená na zobrazenie priamo
// v paneli úloh.
func TrayIconPromoted(exePath string) (bool, error) {
	root, name, err := findTrayEntry(exePath)
	if err != nil {
		return false, err
	}
	defer syscall.RegCloseKey(root)

	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(root, Str(name), 0, syscall.KEY_READ, &h); err != nil {
		return false, err
	}
	defer syscall.RegCloseKey(h)

	var typ, val, size uint32
	size = 4
	if err := syscall.RegQueryValueEx(h, Str("IsPromoted"), nil, &typ,
		(*byte)(unsafe.Pointer(&val)), &size); err != nil {
		return false, nil // hodnota chýba = ikona je schovaná
	}
	return val != 0, nil
}

// findTrayEntry nájde podkľúč patriaci k programu. Volajúci musí vrátený
// kľúč zavrieť.
func findTrayEntry(exePath string) (syscall.Handle, string, error) {
	var root syscall.Handle
	if err := syscall.RegOpenKeyEx(hkeyCurrentUser, Str(notifyIconSettingsKey), 0,
		syscall.KEY_READ, &root); err != nil {
		return 0, "", ErrTrayPromotionUnsupported
	}
	names, err := subKeyNames(root)
	if err != nil {
		syscall.RegCloseKey(root)
		return 0, "", err
	}
	// Presná zhoda cesty je spoľahlivá; keď sa program medzitým presunul,
	// skúsime aspoň zhodu podľa názvu súboru.
	for _, exact := range []bool{true, false} {
		for _, name := range names {
			if matchesExe(root, name, exePath, exact) {
				return root, name, nil
			}
		}
	}
	syscall.RegCloseKey(root)
	return 0, "", ErrTrayEntryNotFound
}

func subKeyNames(root syscall.Handle) ([]string, error) {
	var names []string
	buf := make([]uint16, 256)
	for i := uint32(0); i < 4096; i++ {
		n := uint32(len(buf))
		err := syscall.RegEnumKeyEx(root, i, &buf[0], &n, nil, nil, nil, nil)
		if err == errNoMoreItems {
			break
		}
		if err != nil {
			if len(names) > 0 {
				break // čo sme stihli prečítať, to stačí
			}
			return nil, err
		}
		names = append(names, FromStr(buf[:n]))
	}
	return names, nil
}

func matchesExe(root syscall.Handle, subKey, exePath string, exact bool) bool {
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(root, Str(subKey), 0, syscall.KEY_READ, &h); err != nil {
		return false
	}
	defer syscall.RegCloseKey(h)

	got, ok := readString(h, "ExecutablePath")
	if !ok || got == "" {
		return false
	}
	if exact {
		return strings.EqualFold(got, exePath)
	}
	return strings.EqualFold(baseName(got), baseName(exePath))
}

func setPromoted(root syscall.Handle, subKey string, on bool) error {
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(root, Str(subKey), 0,
		syscall.KEY_READ|syscall.KEY_WRITE, &h); err != nil {
		return err
	}
	defer syscall.RegCloseKey(h)

	value := uint32(0)
	if on {
		value = 1
	}
	r, errno := procRegSetValueEx.Call(uintptr(h), uintptr(unsafe.Pointer(Str("IsPromoted"))), 0,
		syscall.REG_DWORD, uintptr(unsafe.Pointer(&value)), 4)
	if r != 0 {
		return errno
	}
	return nil
}

// readString prečíta textovú hodnotu z otvoreného kľúča.
func readString(h syscall.Handle, name string) (string, bool) {
	var typ, size uint32
	if err := syscall.RegQueryValueEx(h, Str(name), nil, &typ, nil, &size); err != nil {
		return "", false
	}
	if typ != syscall.REG_SZ && typ != syscall.REG_EXPAND_SZ {
		return "", false
	}
	buf := make([]byte, size+2)
	if err := syscall.RegQueryValueEx(h, Str(name), nil, &typ, &buf[0], &size); err != nil {
		return "", false
	}
	u := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[0])), len(buf)/2)
	return FromStr(u), true
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}
