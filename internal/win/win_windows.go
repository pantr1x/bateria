//go:build windows

// Package win obsahuje minimálne väzby na Win32 API, ktoré aplikácia
// potrebuje. Zámerne bez externých závislostí: `go build` tak funguje aj
// bez pripojenia na sieť.
package win

import (
	"sync"
	"syscall"
	"unsafe"
)

// --- načítanie systémových knižníc ------------------------------------

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procLoadLibEx  = kernel32.NewProc("LoadLibraryExW")
	procGetProcAdr = kernel32.NewProc("GetProcAddress")
)

// loadLibrarySearchSystem32 obmedzí hľadanie knižnice na priečinok System32.
// Bez toho by sa dala podstrčiť knižnica z priečinka s programom.
const loadLibrarySearchSystem32 = 0x00000800

type dll struct {
	name string
	once sync.Once
	h    uintptr
}

func newDLL(name string) *dll { return &dll{name: name} }

func (d *dll) handle() uintptr {
	d.once.Do(func() {
		n, err := syscall.UTF16PtrFromString(d.name)
		if err != nil {
			return
		}
		h, _, _ := procLoadLibEx.Call(uintptr(unsafe.Pointer(n)), 0, loadLibrarySearchSystem32)
		d.h = h
	})
	return d.h
}

// Proc je odložene načítaná funkcia zo systémovej knižnice.
type Proc struct {
	d    *dll
	name string
	once sync.Once
	addr uintptr
}

func (d *dll) proc(name string) *Proc { return &Proc{d: d, name: name} }

func (p *Proc) address() uintptr {
	p.once.Do(func() {
		h := p.d.handle()
		if h == 0 {
			return
		}
		b := append([]byte(p.name), 0)
		a, _, _ := procGetProcAdr.Call(h, uintptr(unsafe.Pointer(&b[0])))
		p.addr = a
	})
	return p.addr
}

// Available hovorí, či funkcia v tejto verzii Windowsu existuje.
func (p *Proc) Available() bool { return p.address() != 0 }

// Call zavolá funkciu. Ak v systéme nie je, vráti 0 a ERROR_PROC_NOT_FOUND.
func (p *Proc) Call(args ...uintptr) (uintptr, syscall.Errno) {
	a := p.address()
	if a == 0 {
		return 0, syscall.Errno(127) // ERROR_PROC_NOT_FOUND
	}
	r, _, err := syscall.SyscallN(a, args...)
	return r, err
}

var (
	// kernel32dll je tá istá knižnica ako kernel32 vyššie, len cez vlastný
	// zavádzač – aby mali všetky funkcie rovnaké rozhranie.
	kernel32dll = newDLL("kernel32.dll")
	user32      = newDLL("user32.dll")
	gdi32       = newDLL("gdi32.dll")
	shell32     = newDLL("shell32.dll")
	advapi32    = newDLL("advapi32.dll")
	setupapi    = newDLL("setupapi.dll")
	dwmapi      = newDLL("dwmapi.dll")
)

// --- základné typy -----------------------------------------------------

type (
	// HANDLE je všeobecný systémový popisovač.
	HANDLE = uintptr
	// HWND je popisovač okna.
	HWND = uintptr
)

// POINT je bod na obrazovke.
type POINT struct{ X, Y int32 }

// RECT je obdĺžnik (pravý a dolný okraj sú exkluzívne).
type RECT struct{ Left, Top, Right, Bottom int32 }

// Width vráti šírku obdĺžnika.
func (r RECT) Width() int32 { return r.Right - r.Left }

// Height vráti výšku obdĺžnika.
func (r RECT) Height() int32 { return r.Bottom - r.Top }

// GUID je systémový identifikátor rozhrania.
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// MSG je správa z fronty okna.
type MSG struct {
	Hwnd    HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

// Str prevedie reťazec na ukazovateľ na UTF-16, ako ho chce Win32.
func Str(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		z := uint16(0)
		return &z
	}
	return p
}

// FromStr prevedie UTF-16 pole na reťazec.
func FromStr(b []uint16) string {
	for i, v := range b {
		if v == 0 {
			return syscall.UTF16ToString(b[:i])
		}
	}
	return syscall.UTF16ToString(b)
}

// CopyStr skopíruje reťazec do poľa UTF-16 pevnej dĺžky a ukončí ho nulou.
func CopyStr(dst []uint16, s string) {
	src := syscall.StringToUTF16(s)
	n := len(src)
	if n > len(dst) {
		n = len(dst)
		src[n-1] = 0
	}
	copy(dst[:n], src[:n])
}

// LoWord vráti dolných 16 bitov, HiWord horných – používa sa pri
// rozbaľovaní parametrov správ okna.
func LoWord(v uintptr) uint16 { return uint16(v & 0xffff) }

// HiWord vráti horných 16 bitov 32-bitovej časti hodnoty.
func HiWord(v uintptr) uint16 { return uint16((v >> 16) & 0xffff) }
