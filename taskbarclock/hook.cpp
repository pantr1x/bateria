// bateria-hook.dll – vloží zostávajúci čas batérie priamo do hodín v paneli
// úloh, takže ho panel vykreslí vo svojom vlastnom prvku pri čase.
//
// Ako to funguje: panel úloh skladá text hodín tak, že si najprv vypýta
// aktuálny čas cez GetLocalTime a potom ho naformátuje cez GetTimeFormatEx.
// Obe sú verejné, zdokumentované funkcie Windowsu. Knižnica ich v bežiacom
// Prieskumníkovi presmeruje na seba a do naformátovaného reťazca pridá náš
// text. To isté robí vo Windows 11 aj známy mod Taskbar Clock Customization
// pre Windhawk – nepotrebuje na to žiadne nezdokumentované vnútornosti
// panela ani ladiace symboly z Microsoftu.
//
// Presmerovanie je zámerne cez tabuľku importov (IAT), nie prepisom kódu:
// je to jediný zápis ukazovateľa, úplne vratný a bez rozoberania inštrukcií.
// Keď sa netrafí, len sa nič nepridá – nič sa nerozbije.
//
// Preklad (jeden príkaz, viď taskbarclock/README.md):
//   g++ -O2 -s -shared -static -municode -o bateria-hook.dll hook.cpp -lkernel32 -luser32

#include <windows.h>
#include <tlhelp32.h>
#include <stdint.h>

// --- zdieľaný text -----------------------------------------------------

// ClockText musí presne zodpovedať štruktúre v internal/win/shmem_windows.go.
struct ClockText {
    volatile int32_t version;
    volatile int32_t enabled;
    volatile int32_t sequence;
    int32_t          reserved;
    wchar_t          text[64];
};
static_assert(sizeof(ClockText) == 144, "ClockText sa rozisiel s Go stranou");

static const wchar_t kShareName[] = L"Local\\BateriaClockText";
static const int32_t kShareVersion = 1;

static HANDLE     g_share  = NULL;
static ClockText* g_shared = NULL;

// --- pôvodné funkcie ---------------------------------------------------

typedef VOID(WINAPI* GetLocalTime_t)(LPSYSTEMTIME);
typedef int(WINAPI* GetTimeFormatEx_t)(LPCWSTR, DWORD, const SYSTEMTIME*, LPCWSTR, LPWSTR, int);

static GetLocalTime_t    g_origGetLocalTime    = NULL;
static GetTimeFormatEx_t g_origGetTimeFormatEx = NULL;

// Čas, ktorý si panel naposledy vypýtal, a to na tom istom vlákne. Podľa
// neho poznáme, že nasledujúce formátovanie patrí hodinám v paneli, a nie
// hocijakému inému času v Prieskumníkovi.
static thread_local SYSTEMTIME g_lastTime;
static thread_local bool       g_haveTime = false;

static const wchar_t kSeparator[]  = L"  ";
static const int     kSeparatorLen = 2;

// sameMinute porovná čas bez sekúnd a milisekúnd – hodiny sekundy zvyčajne
// nezobrazujú a formátovať sa môže o chlp neskôr.
static bool sameMinute(const SYSTEMTIME& a, const SYSTEMTIME& b) {
    return a.wYear == b.wYear && a.wMonth == b.wMonth && a.wDay == b.wDay &&
           a.wHour == b.wHour && a.wMinute == b.wMinute;
}

// currentText skopíruje text zo zdieľanej pamäte. Vráti počet znakov, alebo
// 0, keď text nie je alebo je vypnutý.
static int currentText(wchar_t* out, int max) {
    if (!g_shared || g_shared->version != kShareVersion || !g_shared->enabled) return 0;
    int n = 0;
    while (n < max - 1 && n < 63) {
        wchar_t c = g_shared->text[n];
        if (!c) break;
        out[n++] = c;
    }
    out[n] = 0;
    return n;
}

// --- náhrady -----------------------------------------------------------

static VOID WINAPI GetLocalTime_Hook(LPSYSTEMTIME time) {
    g_origGetLocalTime(time);
    if (time) {
        g_lastTime = *time;
        g_haveTime = true;
    }
}

static int WINAPI GetTimeFormatEx_Hook(LPCWSTR locale, DWORD flags, const SYSTEMTIME* time,
                                       LPCWSTR format, LPWSTR out, int cch) {
    int n = g_origGetTimeFormatEx(locale, flags, time, format, out, cch);

    // Pridávame len k formátovaniu, ktoré patrí hodinám v paneli: rovnaké
    // vlákno a ten istý čas, aký si panel pred chvíľou vypýtal.
    if (n <= 0 || !time || !g_haveTime || !sameMinute(*time, g_lastTime)) return n;

    wchar_t text[64];
    int textLen = currentText(text, 64);
    if (textLen <= 0) return n;

    int total = textLen + kSeparatorLen + n;
    if (cch == 0) {
        // Volajúci si len zisťuje potrebnú veľkosť – povieme mu ju aj s naším
        // textom, inak by nabudúce poslal krátky vyrovnávací pamäť.
        return total + 1;
    }
    if (!out || total + 1 > cch) return n; // nezmestí sa, necháme pôvodný text

    // Text dávame pred čas, nech je čo najbližšie k ikone batérie.
    memmove(out + textLen + kSeparatorLen, out, (size_t)(n + 1) * sizeof(wchar_t));
    memcpy(out, text, (size_t)textLen * sizeof(wchar_t));
    memcpy(out + textLen, kSeparator, (size_t)kSeparatorLen * sizeof(wchar_t));
    return total;
}

// --- tabuľka importov --------------------------------------------------

// patchImports prejde tabuľku importov modulu a vymení ukazovateľ na danú
// funkciu. Keď je restore true, vráti tam pôvodnú funkciu.
static void patchImports(HMODULE module, const char* name, void* replacement,
                         void* original, bool restore) {
    if (!module) return;
    PIMAGE_DOS_HEADER dos = (PIMAGE_DOS_HEADER)module;
    if (dos->e_magic != IMAGE_DOS_SIGNATURE) return;
    PIMAGE_NT_HEADERS nt = (PIMAGE_NT_HEADERS)((BYTE*)module + dos->e_lfanew);
    if (nt->Signature != IMAGE_NT_SIGNATURE) return;

    IMAGE_DATA_DIRECTORY dir = nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT];
    if (!dir.VirtualAddress || !dir.Size) return;

    PIMAGE_IMPORT_DESCRIPTOR desc = (PIMAGE_IMPORT_DESCRIPTOR)((BYTE*)module + dir.VirtualAddress);
    for (; desc->Name; ++desc) {
        if (!desc->OriginalFirstThunk || !desc->FirstThunk) continue;
        PIMAGE_THUNK_DATA named = (PIMAGE_THUNK_DATA)((BYTE*)module + desc->OriginalFirstThunk);
        PIMAGE_THUNK_DATA entry = (PIMAGE_THUNK_DATA)((BYTE*)module + desc->FirstThunk);
        for (; named->u1.AddressOfData; ++named, ++entry) {
            if (IMAGE_SNAP_BY_ORDINAL(named->u1.Ordinal)) continue;
            PIMAGE_IMPORT_BY_NAME imported =
                (PIMAGE_IMPORT_BY_NAME)((BYTE*)module + named->u1.AddressOfData);
            if (lstrcmpA((LPCSTR)imported->Name, name) != 0) continue;

            void* want = restore ? original : replacement;
            void* has = (void*)entry->u1.Function;
            if (has == want) return;
            if (restore && has != replacement) return; // meníme len svoje

            DWORD old = 0;
            if (!VirtualProtect(&entry->u1.Function, sizeof(void*), PAGE_READWRITE, &old)) return;
            entry->u1.Function = (ULONGLONG)(ULONG_PTR)want;
            VirtualProtect(&entry->u1.Function, sizeof(void*), old, &old);
            return;
        }
    }
}

// patchAllModules prejde všetky načítané moduly Prieskumníka. Nevieme dopredu,
// ktorý z nich hodiny kreslí, a časom pribúdajú ďalšie.
static void patchAllModules(bool restore) {
    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPMODULE, GetCurrentProcessId());
    if (snap == INVALID_HANDLE_VALUE) return;

    HMODULE self = NULL;
    GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                           GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
                       (LPCWSTR)&patchAllModules, &self);

    MODULEENTRY32W entry;
    entry.dwSize = sizeof(entry);
    if (Module32FirstW(snap, &entry)) {
        do {
            if (entry.hModule == self) continue; // seba nechávame na pokoji
            patchImports(entry.hModule, "GetLocalTime", (void*)GetLocalTime_Hook,
                         (void*)g_origGetLocalTime, restore);
            patchImports(entry.hModule, "GetTimeFormatEx", (void*)GetTimeFormatEx_Hook,
                         (void*)g_origGetTimeFormatEx, restore);
        } while (Module32NextW(snap, &entry));
    }
    CloseHandle(snap);
}

// --- zavedenie ---------------------------------------------------------

// isExplorer zisťuje, či bežíme v Prieskumníkovi. Do vlastného procesu si
// knižnicu zavádza aj samotná aplikácia (kvôli adrese funkcie pre hák), tam
// ale nemá čo meniť.
static bool isExplorer() {
    wchar_t path[MAX_PATH];
    DWORD n = GetModuleFileNameW(NULL, path, MAX_PATH);
    if (!n || n >= MAX_PATH) return false;
    for (DWORD i = n; i > 0; --i) {
        if (path[i - 1] == L'\\') return lstrcmpiW(path + i, L"explorer.exe") == 0;
    }
    return false;
}

static void openShare() {
    if (g_shared) return;
    g_share = OpenFileMappingW(FILE_MAP_READ, FALSE, kShareName);
    if (!g_share) return;
    g_shared = (ClockText*)MapViewOfFile(g_share, FILE_MAP_READ, 0, 0, sizeof(ClockText));
    if (!g_shared) {
        CloseHandle(g_share);
        g_share = NULL;
    }
}

static void setup() {
    HMODULE kernel = GetModuleHandleW(L"kernel32.dll");
    if (!kernel) return;
    g_origGetLocalTime    = (GetLocalTime_t)(void*)GetProcAddress(kernel, "GetLocalTime");
    g_origGetTimeFormatEx = (GetTimeFormatEx_t)(void*)GetProcAddress(kernel, "GetTimeFormatEx");
    if (!g_origGetLocalTime || !g_origGetTimeFormatEx) return;
    openShare();
    patchAllModules(false);
}

// CallWndProc je vstupný bod pre SetWindowsHookEx – cezeň si Windows sám
// zavedie knižnicu do Prieskumníka. Slúži aj na občasné dohľadanie modulov,
// ktoré sa načítali až neskôr.
extern "C" __declspec(dllexport) LRESULT CALLBACK CallWndProc(int code, WPARAM wp, LPARAM lp) {
    if (code == HC_ACTION && g_origGetTimeFormatEx) {
        static ULONGLONG lastScan = 0;
        ULONGLONG now = GetTickCount64();
        if (now - lastScan > 5000) {
            lastScan = now;
            openShare();
            patchAllModules(false);
        }
    }
    return CallNextHookEx(NULL, code, wp, lp);
}

BOOL WINAPI DllMain(HINSTANCE inst, DWORD reason, LPVOID) {
    switch (reason) {
    case DLL_PROCESS_ATTACH:
        DisableThreadLibraryCalls(inst);
        if (isExplorer()) setup();
        break;
    case DLL_PROCESS_DETACH:
        if (g_origGetTimeFormatEx) patchAllModules(true);
        if (g_shared) UnmapViewOfFile(g_shared);
        if (g_share) CloseHandle(g_share);
        break;
    }
    return TRUE;
}
