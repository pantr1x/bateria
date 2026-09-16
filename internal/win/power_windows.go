//go:build windows

package win

import (
	"errors"
	"syscall"
	"unsafe"
)

// --- GetSystemPowerStatus ---------------------------------------------

// SystemPowerStatus je to, čo o napájaní hlási samotný Windows.
type SystemPowerStatus struct {
	ACLineStatus        byte // 0 = na batérii, 1 = v sieti, 255 = neznáme
	BatteryFlag         byte
	BatteryLifePercent  byte   // 0–100, 255 = neznáme
	SystemStatusFlag    byte   // šetrič energie
	BatteryLifeTime     uint32 // sekundy do vybitia, 0xFFFFFFFF = neznáme
	BatteryFullLifeTime uint32 // v praxi takmer vždy neznáme
}

// Príznaky v BatteryFlag.
const (
	BatteryFlagHigh      = 1
	BatteryFlagLow       = 2
	BatteryFlagCritical  = 4
	BatteryFlagCharging  = 8
	BatteryFlagNoBattery = 128
	BatteryFlagUnknown   = 255

	// Unknown je hodnota, ktorou Windows hovorí „neviem“.
	Unknown = 0xFFFFFFFF
)

var procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")

// GetSystemPowerStatus prečíta stav napájania.
func GetSystemPowerStatus() (SystemPowerStatus, error) {
	var s SystemPowerStatus
	r, _, err := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s)))
	if r == 0 {
		return s, err
	}
	return s, nil
}

// --- priamy prístup k ovládaču batérie --------------------------------

// Toto je jediný spoľahlivý zdroj okamžitého toku energie (mW). Bez neho
// sa čas do plného nabitia nedá spočítať – Windows ho cez
// GetSystemPowerStatus nehlási.

var guidDeviceBattery = GUID{
	Data1: 0x72631e54, Data2: 0x78A4, Data3: 0x11d0,
	Data4: [8]byte{0xbc, 0xf7, 0x00, 0xaa, 0x00, 0xb7, 0xb3, 0x2a},
}

const (
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010

	ioctlBatteryQueryTag         = 0x00294040
	ioctlBatteryQueryInformation = 0x00294044
	ioctlBatteryQueryStatus      = 0x0029404C

	levelBatteryInformation   = 0
	levelBatteryEstimatedTime = 3

	// BATTERY_CAPACITY_RELATIVE: kapacity nie sú v mWh, ale v bezrozmerných
	// jednotkách. Pomer (a teda čas) platí, watty nie.
	capabilityRelative = 0x40000000

	powerStateOnLine      = 0x00000001
	powerStateDischarging = 0x00000002
	powerStateCharging    = 0x00000004
	powerStateCritical    = 0x00000008

	unknownRate = -0x80000000
)

var (
	procSetupDiGetClassDevsW             = setupapi.proc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces      = setupapi.proc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetailW = setupapi.proc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfoList     = setupapi.proc("SetupDiDestroyDeviceInfoList")
)

type spDeviceInterfaceData struct {
	CbSize             uint32
	InterfaceClassGuid GUID
	Flags              uint32
	Reserved           uintptr
}

type batteryQueryInformation struct {
	BatteryTag       uint32
	InformationLevel uint32
	AtRate           int32
}

type batteryInformation struct {
	Capabilities        uint32
	Technology          uint8
	Reserved            [3]uint8
	Chemistry           [4]uint8
	DesignedCapacity    uint32
	FullChargedCapacity uint32
	DefaultAlert1       uint32
	DefaultAlert2       uint32
	CriticalBias        uint32
	CycleCount          uint32
}

type batteryWaitStatus struct {
	BatteryTag   uint32
	Timeout      uint32
	PowerState   uint32
	LowCapacity  uint32
	HighCapacity uint32
}

type batteryStatus struct {
	PowerState uint32
	Capacity   uint32
	Voltage    uint32
	Rate       int32
}

// BatteryReading je odčítanie jednej fyzickej batérie priamo z ovládača.
type BatteryReading struct {
	Charging    bool
	Discharging bool
	OnLine      bool
	Critical    bool

	Capacity     uint32 // mWh
	FullCapacity uint32 // mWh
	DesignedCap  uint32 // mWh
	Voltage      uint32 // mV
	CycleCount   uint32
	Relative     bool
	Chemistry    string

	// Rate je okamžitý tok v mW, kladný pri nabíjaní. Platné len ak RateKnown.
	Rate      int32
	RateKnown bool

	// EstimatedSeconds je odhad ovládača, koľko batéria vydrží. 0 = nemá.
	EstimatedSeconds uint32
}

// ErrNoBatteryDevice znamená, že v systéme nie je žiadna batéria.
var ErrNoBatteryDevice = errors.New("v systéme nie je žiadna batéria")

// ReadBatteries prečíta stav všetkých batérií cez rozhranie ovládača.
func ReadBatteries() ([]BatteryReading, error) {
	if !procSetupDiGetClassDevsW.Available() {
		return nil, errors.New("setupapi.dll nie je k dispozícii")
	}
	hdev, _ := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&guidDeviceBattery)), 0, 0, digcfPresent|digcfDeviceInterface)
	if hdev == 0 || hdev == ^uintptr(0) {
		return nil, errors.New("zoznam zariadení batérie sa nepodarilo získať")
	}
	defer procSetupDiDestroyDeviceInfoList.Call(hdev)

	var out []BatteryReading
	for i := 0; i < 16; i++ { // horná hranica, nech sa cyklus nikdy nezacyklí
		did := spDeviceInterfaceData{}
		did.CbSize = uint32(unsafe.Sizeof(did))
		r, _ := procSetupDiEnumDeviceInterfaces.Call(hdev, 0,
			uintptr(unsafe.Pointer(&guidDeviceBattery)), uintptr(i), uintptr(unsafe.Pointer(&did)))
		if r == 0 {
			break
		}
		path, err := interfacePath(hdev, &did)
		if err != nil || path == "" {
			continue
		}
		if b, err := readOne(path); err == nil {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		return nil, ErrNoBatteryDevice
	}
	return out, nil
}

// interfacePath zistí cestu k zariadeniu, cez ktorú sa dá otvoriť.
func interfacePath(hdev uintptr, did *spDeviceInterfaceData) (string, error) {
	var need uint32
	procSetupDiGetDeviceInterfaceDetailW.Call(hdev, uintptr(unsafe.Pointer(did)), 0, 0,
		uintptr(unsafe.Pointer(&need)), 0)
	if need < 6 {
		return "", errors.New("neplatná veľkosť popisu zariadenia")
	}
	buf := make([]byte, need)
	// cbSize je veľkosť hlavičky štruktúry, nie celého vyrovnávacieho pamäte:
	// 8 bajtov na 64-bitovom systéme, 6 na 32-bitovom.
	cb := uint32(8)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		cb = 6
	}
	*(*uint32)(unsafe.Pointer(&buf[0])) = cb
	r, _ := procSetupDiGetDeviceInterfaceDetailW.Call(hdev, uintptr(unsafe.Pointer(did)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(need), uintptr(unsafe.Pointer(&need)), 0)
	if r == 0 {
		return "", errors.New("popis zariadenia sa nepodarilo prečítať")
	}
	// Reťazec s cestou začína hneď za cbSize, teda na 4. bajte.
	chars := (need - 4) / 2
	u := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[4])), chars)
	return FromStr(u), nil
}

func readOne(path string) (BatteryReading, error) {
	var b BatteryReading
	h, err := syscall.CreateFile(Str(path),
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return b, err
	}
	defer syscall.CloseHandle(h)

	// 1. Značka batérie – mení sa pri výmene, všetky ďalšie dotazy ju chcú.
	var wait, tag uint32
	var ret uint32
	if err := syscall.DeviceIoControl(h, ioctlBatteryQueryTag,
		(*byte)(unsafe.Pointer(&wait)), 4,
		(*byte)(unsafe.Pointer(&tag)), 4, &ret, nil); err != nil {
		return b, err
	}
	if tag == 0 {
		return b, errors.New("batéria nie je pripojená")
	}

	// 2. Nemenné údaje: kapacity, počet cyklov, chémia.
	q := batteryQueryInformation{BatteryTag: tag, InformationLevel: levelBatteryInformation}
	var info batteryInformation
	if err := syscall.DeviceIoControl(h, ioctlBatteryQueryInformation,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), &ret, nil); err != nil {
		return b, err
	}
	b.FullCapacity = info.FullChargedCapacity
	b.DesignedCap = info.DesignedCapacity
	b.CycleCount = info.CycleCount
	b.Relative = info.Capabilities&capabilityRelative != 0
	b.Chemistry = string(trimZeros(info.Chemistry[:]))

	// 3. Okamžitý stav vrátane toku energie.
	ws := batteryWaitStatus{BatteryTag: tag}
	var st batteryStatus
	if err := syscall.DeviceIoControl(h, ioctlBatteryQueryStatus,
		(*byte)(unsafe.Pointer(&ws)), uint32(unsafe.Sizeof(ws)),
		(*byte)(unsafe.Pointer(&st)), uint32(unsafe.Sizeof(st)), &ret, nil); err != nil {
		return b, err
	}
	b.Charging = st.PowerState&powerStateCharging != 0
	b.Discharging = st.PowerState&powerStateDischarging != 0
	b.OnLine = st.PowerState&powerStateOnLine != 0
	b.Critical = st.PowerState&powerStateCritical != 0
	if st.Capacity != Unknown {
		b.Capacity = st.Capacity
	}
	if st.Voltage != Unknown {
		b.Voltage = st.Voltage
	}
	if st.Rate != unknownRate && st.Rate != 0 {
		rate := st.Rate
		// Niektoré ovládače hlásia pri vybíjaní kladné číslo. Znamienko
		// určíme podľa stavu, nie podľa dôvery v ovládač.
		if b.Discharging && rate > 0 {
			rate = -rate
		}
		if b.Charging && rate < 0 {
			rate = -rate
		}
		b.Rate = rate
		b.RateKnown = true
	}

	// 4. Odhad samotného ovládača (ak ho vie), ako záložný zdroj.
	q = batteryQueryInformation{BatteryTag: tag, InformationLevel: levelBatteryEstimatedTime}
	var secs uint32
	if err := syscall.DeviceIoControl(h, ioctlBatteryQueryInformation,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		(*byte)(unsafe.Pointer(&secs)), 4, &ret, nil); err == nil && secs != Unknown {
		b.EstimatedSeconds = secs
	}
	return b, nil
}

func trimZeros(b []uint8) []uint8 {
	for i, v := range b {
		if v == 0 {
			return b[:i]
		}
	}
	return b
}
