//go:build windows

package win

import (
	"sync"
	"syscall"
	"unsafe"
)

// Zdieľaná pamäť, cez ktorú aplikácia posiela text pre hodiny v paneli
// knižnici bateria-hook.dll zavedenej v Prieskumníkovi.

const clockShareName = `Local\BateriaClockText`

const clockShareVersion = 1

// clockText musí presne zodpovedať štruktúre ClockText v taskbarclock/hook.cpp.
type clockText struct {
	Version  int32
	Enabled  int32
	Sequence int32
	Reserved int32
	Text     [64]uint16
}

// Kontrola pri preklade: štruktúra musí mať presne 144 bajtov ako v C++.
const (
	_ = uint(unsafe.Sizeof(clockText{}) - 144)
	_ = uint(144 - unsafe.Sizeof(clockText{}))
)

const (
	pageReadWrite = 0x04
	fileMapWrite  = 0x0002
	fileMapRead   = 0x0004
)

var (
	procCreateFileMapping = kernel32.NewProc("CreateFileMappingW")
	procMapViewOfFile     = kernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile   = kernel32.NewProc("UnmapViewOfFile")

	clockShare struct {
		once   sync.Once
		handle uintptr
		view   *clockText
		ok     bool
	}
)

// openClockShare vytvorí (alebo otvorí) zdieľanú pamäť. Robí sa raz.
func openClockShare() bool {
	clockShare.once.Do(func() {
		size := uint32(unsafe.Sizeof(clockText{}))
		h, _, _ := procCreateFileMapping.Call(
			^uintptr(0), // INVALID_HANDLE_VALUE = pamäť, nie súbor
			0, pageReadWrite, 0, uintptr(size),
			uintptr(unsafe.Pointer(Str(clockShareName))))
		if h == 0 {
			return
		}
		view, _, _ := procMapViewOfFile.Call(h, fileMapWrite, 0, 0, uintptr(size))
		if view == 0 {
			syscall.CloseHandle(syscall.Handle(h))
			return
		}
		clockShare.handle = h
		clockShare.view = (*clockText)(unsafe.Pointer(view))
		clockShare.view.Version = clockShareVersion
		clockShare.ok = true
	})
	return clockShare.ok
}

// WriteClockText zapíše text pre hodiny do zdieľanej pamäte. enabled=false
// spraví hák priechodným (hodiny sa vrátia do pôvodného stavu).
func WriteClockText(text string, enabled bool) bool {
	if !openClockShare() {
		return false
	}
	v := clockShare.view
	var buf [64]uint16
	CopyStr(buf[:], text)
	v.Text = buf
	if enabled {
		v.Enabled = 1
	} else {
		v.Enabled = 0
	}
	v.Sequence++
	return true
}

// ClockShareReady hovorí, či sa zdieľaná pamäť podarilo pripraviť.
func ClockShareReady() bool { return openClockShare() }
