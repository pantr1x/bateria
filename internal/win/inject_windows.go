//go:build windows

package win

import (
	"os"
	"path/filepath"
	"time"
	"unsafe"
)

// Zavedenie knižnice bateria-hook.dll do Prieskumníka. Používa sa na to
// SetWindowsHookEx nad vláknom okna panela úloh – Windows knižnicu zavedie do
// explorer.exe sám, bez zápisu do cudzieho procesu. To isté API používajú aj
// bežné doplnky rozhrania (napr. rozšírenia klávesnice).

// HookDLLName je meno knižnice, ktorú aplikácia hľadá vedľa seba.
const HookDLLName = "bateria-hook.dll"

const whCallWndProc = 4

var (
	procSetWindowsHookEx    = user32.proc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.proc("UnhookWindowsHookEx")
	procGetWindowThreadPID  = user32.proc("GetWindowThreadProcessId")
	procLoadLibrary         = kernel32dll.proc("LoadLibraryW")
	procFreeLibrary         = kernel32dll.proc("FreeLibrary")
)

// ClockHook drží zavedený hák, aby sa dal pri ukončení odpojiť.
type ClockHook struct {
	hook uintptr
	dll  uintptr
}

// HookDLLPath vráti cestu ku knižnici vedľa spustiteľného súboru.
func HookDLLPath() string {
	exe, err := os.Executable()
	if err != nil {
		return HookDLLName
	}
	return filepath.Join(filepath.Dir(exe), HookDLLName)
}

// HookDLLPresent hovorí, či je knižnica vedľa programu.
func HookDLLPresent() bool {
	_, err := os.Stat(HookDLLPath())
	return err == nil
}

// InstallClockHook zavedie knižnicu do vlákna panela úloh. Vráti nil, keď
// knižnica nie je alebo sa zaviesť nepodarí – aplikácia potom beží ďalej len
// s ikonou.
func InstallClockHook() *ClockHook {
	path := HookDLLPath()
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	// Zdieľaná pamäť musí existovať skôr, než knižnica v Prieskumníku začne
	// čítať text.
	openClockShare()

	dll, _ := procLoadLibrary.Call(uintptr(unsafe.Pointer(Str(path))))
	if dll == 0 {
		return nil
	}
	proc := dllProcAddress(dll, "CallWndProc")
	if proc == 0 {
		procFreeLibrary.Call(dll)
		return nil
	}
	taskbar := FindWindow("Shell_TrayWnd")
	if taskbar == 0 {
		procFreeLibrary.Call(dll)
		return nil
	}
	var pid uint32
	tid, _ := procGetWindowThreadPID.Call(taskbar, uintptr(unsafe.Pointer(&pid)))
	if tid == 0 {
		procFreeLibrary.Call(dll)
		return nil
	}
	hook, _ := procSetWindowsHookEx.Call(whCallWndProc, proc, dll, tid)
	if hook == 0 {
		procFreeLibrary.Call(dll)
		return nil
	}
	return &ClockHook{hook: hook, dll: dll}
}

// Remove odpojí hák a uvoľní knižnicu. Predtým vypne text, nech sa hodiny
// vrátia do pôvodného stavu.
func (h *ClockHook) Remove() {
	if h == nil {
		return
	}
	WriteClockText("", false)
	// Krátka pauza, nech dobehnú prípadné volania v Prieskumníku.
	time.Sleep(200 * time.Millisecond)
	if h.hook != 0 {
		procUnhookWindowsHookEx.Call(h.hook)
		h.hook = 0
	}
	if h.dll != 0 {
		procFreeLibrary.Call(h.dll)
		h.dll = 0
	}
}

// Active hovorí, či je hák zavedený.
func (h *ClockHook) Active() bool { return h != nil && h.hook != 0 }

// dllProcAddress nájde exportovanú funkciu v načítanej knižnici.
func dllProcAddress(dll uintptr, name string) uintptr {
	b := append([]byte(name), 0)
	addr, _, _ := procGetProcAdr.Call(dll, uintptr(unsafe.Pointer(&b[0])))
	return addr
}
