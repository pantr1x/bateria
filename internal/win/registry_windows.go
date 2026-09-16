//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

var (
	procRegSetValueEx = advapi32.proc("RegSetValueExW")
	procRegDeleteVal  = advapi32.proc("RegDeleteValueW")
)

const (
	hkeyCurrentUser = syscall.HKEY_CURRENT_USER

	themeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "Bateria"
)

// TaskbarUsesLightTheme hovorí, či je panel úloh svetlý. Podľa toho sa volí
// farba ikony – na svetlom paneli by biela ikona zanikla.
func TaskbarUsesLightTheme() bool {
	// SystemUsesLightTheme sa týka panela úloh; AppsUseLightTheme okien.
	if v, ok := readDWORD(themeKey, "SystemUsesLightTheme"); ok {
		return v != 0
	}
	// Staršie systémy túto hodnotu nemajú a panel je tam tmavý.
	return false
}

// AppsUseLightTheme hovorí, či majú okná svetlý vzhľad.
func AppsUseLightTheme() bool {
	if v, ok := readDWORD(themeKey, "AppsUseLightTheme"); ok {
		return v != 0
	}
	return true
}

func readDWORD(path, name string) (uint32, bool) {
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(hkeyCurrentUser, Str(path), 0, syscall.KEY_READ, &h); err != nil {
		return 0, false
	}
	defer syscall.RegCloseKey(h)
	var typ, val, size uint32
	size = 4
	if err := syscall.RegQueryValueEx(h, Str(name), nil, &typ,
		(*byte)(unsafe.Pointer(&val)), &size); err != nil {
		return 0, false
	}
	if typ != syscall.REG_DWORD {
		return 0, false
	}
	return val, true
}

// AutostartEnabled zistí, či je aplikácia v zozname spúšťanom po prihlásení.
func AutostartEnabled() bool {
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(hkeyCurrentUser, Str(runKey), 0, syscall.KEY_READ, &h); err != nil {
		return false
	}
	defer syscall.RegCloseKey(h)
	var typ, size uint32
	err := syscall.RegQueryValueEx(h, Str(runValue), nil, &typ, nil, &size)
	return err == nil && size > 0
}

// SetAutostart zapne alebo vypne spúšťanie po prihlásení do Windowsu.
func SetAutostart(on bool, exePath string) error {
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(hkeyCurrentUser, Str(runKey), 0, syscall.KEY_WRITE, &h); err != nil {
		return err
	}
	defer syscall.RegCloseKey(h)

	if !on {
		r, errno := procRegDeleteVal.Call(uintptr(h), uintptr(unsafe.Pointer(Str(runValue))))
		const errFileNotFound = 2
		if r != 0 && r != errFileNotFound {
			return errno
		}
		return nil
	}
	// Cesta v úvodzovkách, inak by sa cesta s medzerami rozpadla na argumenty.
	value := syscall.StringToUTF16(`"` + exePath + `"`)
	r, errno := procRegSetValueEx.Call(uintptr(h), uintptr(unsafe.Pointer(Str(runValue))), 0,
		syscall.REG_SZ, uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)*2))
	if r != 0 {
		return errno
	}
	return nil
}
