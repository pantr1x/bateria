// bateria-panel.exe – text so zostávajúcim časom priamo v paneli úloh.
//
// Okno s textom sa vytvára ako POTOMOK okna panela úloh (Shell_TrayWnd).
// Nie je to teda voľne plávajúce okno „prilepené navrch": Windows ho oreže
// na plochu panela, posúva ho spolu s ním, skryje ho, keď sa panel skryje,
// a zruší ho, keď panel zanikne.
//
// Polohu si okno hľadá samo: umiestni sa tesne naľavo od systémovej oblasti
// (wifi/zvuk/batéria a hodiny), takže text vyjde hneď vedľa nej bez toho, aby
// ho používateľ musel niekam ťahať. Keď sa oblasť nenájde, použije sa
// rozumné odsadenie od pravého okraja.
//
// Zámerne je to samostatný program, nie súčasť hlavnej aplikácie. Okno
// potomka cudzieho procesu zdieľa s panelom vstupnú frontu, takže keby sa
// jeho vlákno na čokoľvek zaseklo, zaseklo by aj panel úloh. Tento program
// preto nerobí nič iné než kreslí text, ktorý dostane – žiadne čítanie
// batérie, žiadne súbory, žiadne čakanie.
//
// Preklad (na Linuxe aj vo Windowse s MinGW):
//   x86_64-w64-mingw32-g++ -O2 -s -static -mwindows -municode \
//       -o bateria-panel.exe panel.cpp -lgdi32

#include <windows.h>
#include <stdint.h>

static const wchar_t kHostClass[]  = L"BateriaPanelHost";
static const wchar_t kPanelClass[] = L"BateriaPanel";
static const wchar_t kAppClass[]   = L"BateriaTrayWindow";

// Údaje, ktoré posiela hlavná aplikácia cez WM_COPYDATA.
struct PanelUpdate {
    int32_t  version;    // 1
    int32_t  gap;        // dodatočná medzera vľavo od systémovej oblasti (body)
    uint32_t textColor;  // COLORREF
    uint32_t flags;      // 1 = ukonči sa
    wchar_t  text[64];
};
static_assert(sizeof(PanelUpdate) == 144, "PanelUpdate sa rozišiel s Go stranou");

static const int kUpdateVersion = 1;
static const uint32_t kFlagQuit = 1;

// Udalosti, ktoré panel hlási späť hlavnej aplikácii.
static const WPARAM kEventMenu = 1; // používateľ klikol pravým tlačidlom

static HWND      g_host        = NULL;
static HWND      g_panel       = NULL;
static HWND      g_taskbar     = NULL;
static HFONT     g_font        = NULL;
static int       g_fontHeight  = 0;
static wchar_t   g_text[64]    = L"";
static COLORREF  g_fg          = RGB(255, 255, 255);
static int       g_gap         = 8;
static UINT      g_taskbarCreated = 0;
static UINT      g_panelEvent     = 0;
static int       g_appMissing  = 0;
static RECT      g_lastRect    = {0, 0, 0, 0};

static const UINT kTimerId       = 1;
static const int  kTimerMs       = 1000;
static const int  kPaddingPx     = 10;
static const int  kAppMissesQuit = 8; // po ôsmich sekundách bez aplikácie končíme

// enableDpiAwareness zapne škálovanie podľa monitora, inak by sme počítali
// polohu v nesprávnych bodoch na obrazovkách s vyšším rozlíšením.
static void enableDpiAwareness() {
    typedef BOOL(WINAPI * SetCtx)(HANDLE);
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (user32) {
        SetCtx setCtx = (SetCtx)(void*)GetProcAddress(user32, "SetProcessDpiAwarenessContext");
        if (setCtx && setCtx((HANDLE)(INT_PTR)-4)) return; // PER_MONITOR_AWARE_V2
    }
    SetProcessDPIAware();
}

// notifyApp pošle udalosť hlavnej aplikácii. Keď nebeží, nestane sa nič.
static void notifyApp(WPARAM event, LPARAM value) {
    if (!g_panelEvent) return;
    HWND app = FindWindowW(kAppClass, NULL);
    if (app) PostMessageW(app, g_panelEvent, event, value);
}

// makeFont vyrobí písmo podľa výšky panela, nech text sedí k hodinám.
static void makeFont(int taskbarHeight) {
    int height = taskbarHeight / 4;
    if (height < 11) height = 11;
    if (height > 28) height = 28;
    if (g_font && height == g_fontHeight) return;

    LOGFONTW lf;
    ZeroMemory(&lf, sizeof(lf));
    lf.lfHeight = -height;
    lf.lfWeight = FW_NORMAL;
    lf.lfCharSet = DEFAULT_CHARSET;
    lf.lfQuality = CLEARTYPE_QUALITY;
    lstrcpynW(lf.lfFaceName, L"Segoe UI Variable Text", LF_FACESIZE);
    HFONT created = CreateFontIndirectW(&lf);
    if (!created) {
        lstrcpynW(lf.lfFaceName, L"Segoe UI", LF_FACESIZE);
        created = CreateFontIndirectW(&lf);
    }
    if (!created) return;
    if (g_font) DeleteObject(g_font);
    g_font = created;
    g_fontHeight = height;
}

static int textWidth(const wchar_t* text) {
    HDC dc = GetDC(NULL);
    if (!dc) return 80;
    HGDIOBJ old = SelectObject(dc, g_font);
    SIZE size = {0, 0};
    GetTextExtentPoint32W(dc, text, lstrlenW(text), &size);
    SelectObject(dc, old);
    ReleaseDC(NULL, dc);
    return size.cx;
}

// sampleBackground odkukne farbu panela vedľa nášho okna, aby text splynul
// s pozadím – panel býva tmavý, svetlý aj priehľadný s nádychom zvýrazňovacej
// farby a hádať by sa to nedalo.
static COLORREF sampleBackground(const RECT& panelScreenRect) {
    int y = (panelScreenRect.top + panelScreenRect.bottom) / 2;
    int x = panelScreenRect.left - 6;
    if (x < 0) x = panelScreenRect.right + 6;

    HDC screen = GetDC(NULL);
    if (!screen) return RGB(32, 32, 32);
    COLORREF c = GetPixel(screen, x, y);
    ReleaseDC(NULL, screen);

    if (c == CLR_INVALID) {
        HKEY key;
        DWORD light = 0, size = sizeof(light), type = 0;
        if (RegOpenKeyExW(HKEY_CURRENT_USER,
                L"Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize",
                0, KEY_READ, &key) == ERROR_SUCCESS) {
            RegQueryValueExW(key, L"SystemUsesLightTheme", NULL, &type, (LPBYTE)&light, &size);
            RegCloseKey(key);
        }
        return light ? RGB(243, 243, 243) : RGB(32, 32, 32);
    }
    return c;
}

// trayLeftInTaskbar zistí, kde v paneli začína systémová oblasť (wifi, zvuk,
// batéria a hodiny). Náš text patrí tesne naľavo od nej. Vráti -1, keď sa
// oblasť nenájde – vtedy sa použije odsadenie od pravého okraja.
static int trayLeftInTaskbar(const RECT& taskbarClient) {
    if (!g_taskbar) return -1;
    // Systémová oblasť je potomok panela s triedou TrayNotifyWnd; existuje
    // aj vo Windows 11, hoci jej vnútro je už XAML.
    HWND tray = FindWindowExW(g_taskbar, NULL, L"TrayNotifyWnd", NULL);
    if (!tray) return -1;
    RECT r;
    if (!GetWindowRect(tray, &r)) return -1;

    POINT p = {r.left, r.top};
    ScreenToClient(g_taskbar, &p);
    if (p.x <= taskbarClient.left || p.x > taskbarClient.right) return -1;
    return p.x;
}

static void layoutPanel() {
    if (!g_panel || !g_taskbar || !IsWindow(g_taskbar)) return;
    RECT client;
    if (!GetClientRect(g_taskbar, &client)) return;
    int height = client.bottom - client.top;
    if (height <= 0) return;
    makeFont(height);

    int width = textWidth(g_text) + kPaddingPx * 2;
    if (width < 24) width = 24;

    // Pravý okraj textu je tesne naľavo od systémovej oblasti; keď sa nenájde,
    // odsadíme sa od pravého okraja panela tak, aby sme minuli hodiny.
    int trayLeft = trayLeftInTaskbar(client);
    int right = (trayLeft >= 0) ? trayLeft - g_gap : client.right - 220;
    int x = right - width;
    if (x < 0) x = 0;
    if (x + width > client.right) x = client.right - width;

    RECT want = {x, client.top, x + width, client.top + height};
    if (want.left != g_lastRect.left || want.top != g_lastRect.top ||
        want.right != g_lastRect.right || want.bottom != g_lastRect.bottom) {
        // Presúvame len pri skutočnej zmene: opakované SetWindowPos na
        // potomkovi panela zbytočne prekresľuje a bliká.
        g_lastRect = want;
        SetWindowPos(g_panel, HWND_TOP, x, client.top, width, height,
                     SWP_NOACTIVATE | SWP_SHOWWINDOW);
    }
    InvalidateRect(g_panel, NULL, FALSE);
}

static void paintPanel(HWND hwnd) {
    PAINTSTRUCT ps;
    HDC hdc = BeginPaint(hwnd, &ps);
    if (!hdc) return;

    RECT rc;
    GetClientRect(hwnd, &rc);
    RECT screenRect = rc;
    MapWindowPoints(hwnd, NULL, (POINT*)&screenRect, 2);

    // Kreslíme cez pomocnú plochu, inak by text pri každej zmene preblikol.
    HDC mem = CreateCompatibleDC(hdc);
    HBITMAP bmp = CreateCompatibleBitmap(hdc, rc.right, rc.bottom);
    HGDIOBJ oldBmp = SelectObject(mem, bmp);

    HBRUSH brush = CreateSolidBrush(sampleBackground(screenRect));
    FillRect(mem, &rc, brush);
    DeleteObject(brush);

    HGDIOBJ oldFont = SelectObject(mem, g_font);
    SetBkMode(mem, TRANSPARENT);
    SetTextColor(mem, g_fg);
    DrawTextW(mem, g_text, -1, &rc,
              DT_CENTER | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX);
    SelectObject(mem, oldFont);

    BitBlt(hdc, 0, 0, rc.right, rc.bottom, mem, 0, 0, SRCCOPY);
    SelectObject(mem, oldBmp);
    DeleteObject(bmp);
    DeleteDC(mem);
    EndPaint(hwnd, &ps);
}

static LRESULT CALLBACK panelProc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
    switch (msg) {
    case WM_PAINT:
        paintPanel(hwnd);
        return 0;
    case WM_ERASEBKGND:
        return 1; // pozadie kreslíme sami
    case WM_MOUSEACTIVATE:
        return MA_NOACTIVATE; // kliknutie nesmie prebrať zameranie panelu
    case WM_NCHITTEST:
        // Ľavé kliknutie nechávame prejsť tomu, čo je pod textom, nech nič
        // neprekáža; pravé tlačidlo spracujeme pre ponuku.
        return HTCLIENT;
    case WM_LBUTTONUP:
        // Preposlať kliknutie panelu úloh pod nami by bolo krehké; radšej
        // nerobíme nič, text je len na pozeranie.
        return 0;
    case WM_RBUTTONUP:
        notifyApp(kEventMenu, 0);
        return 0;
    }
    return DefWindowProcW(hwnd, msg, wp, lp);
}

static void ensurePanel() {
    if (g_panel && IsWindow(g_panel) && g_taskbar && IsWindow(g_taskbar)) return;

    if (g_panel && !IsWindow(g_panel)) g_panel = NULL;
    g_taskbar = FindWindowW(L"Shell_TrayWnd", NULL);
    if (!g_taskbar) return;

    if (g_panel) {
        DestroyWindow(g_panel);
        g_panel = NULL;
    }
    g_panel = CreateWindowExW(0, kPanelClass, L"", WS_CHILD,
                              0, 0, 10, 10, g_taskbar, NULL,
                              GetModuleHandleW(NULL), NULL);
    if (g_panel) {
        g_lastRect.left = g_lastRect.right = 0; // vynúť prvé umiestnenie
        layoutPanel();
    }
}

static LRESULT CALLBACK hostProc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
    if (g_taskbarCreated && msg == g_taskbarCreated) {
        g_panel = NULL;
        g_taskbar = NULL;
        ensurePanel();
        return 0;
    }
    switch (msg) {
    case WM_COPYDATA: {
        COPYDATASTRUCT* cds = (COPYDATASTRUCT*)lp;
        if (!cds || cds->cbData != sizeof(PanelUpdate) || !cds->lpData) return 0;
        PanelUpdate update;
        memcpy(&update, cds->lpData, sizeof(update));
        if (update.version != kUpdateVersion) return 0;
        if (update.flags & kFlagQuit) {
            PostQuitMessage(0);
            return 1;
        }
        update.text[63] = 0;
        lstrcpynW(g_text, update.text, 64);
        g_fg = (COLORREF)update.textColor;
        if (update.gap >= 0 && update.gap <= 400) g_gap = update.gap;
        g_appMissing = 0;
        ensurePanel();
        g_lastRect.left = g_lastRect.right = 0; // text sa zmenil, prepočítaj
        layoutPanel();
        return (g_panel && IsWindow(g_panel)) ? 1 : 0;
    }
    case WM_TIMER:
        if (wp == kTimerId) {
            ensurePanel();
            layoutPanel();
            if (FindWindowW(kAppClass, NULL) == NULL) {
                if (++g_appMissing >= kAppMissesQuit) PostQuitMessage(0);
            } else {
                g_appMissing = 0;
            }
        }
        return 0;
    case WM_DESTROY:
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcW(hwnd, msg, wp, lp);
}

int WINAPI wWinMain(HINSTANCE inst, HINSTANCE, PWSTR, int) {
    if (FindWindowW(kHostClass, NULL)) return 0; // druhá inštancia netreba
    enableDpiAwareness();

    WNDCLASSEXW host = {sizeof(host)};
    host.lpfnWndProc = hostProc;
    host.hInstance = inst;
    host.lpszClassName = kHostClass;
    if (!RegisterClassExW(&host)) return 1;

    WNDCLASSEXW panel = {sizeof(panel)};
    panel.lpfnWndProc = panelProc;
    panel.hInstance = inst;
    panel.hCursor = LoadCursorW(NULL, IDC_ARROW);
    panel.lpszClassName = kPanelClass;
    if (!RegisterClassExW(&panel)) return 1;

    g_taskbarCreated = RegisterWindowMessageW(L"TaskbarCreated");
    g_panelEvent = RegisterWindowMessageW(L"BateriaPanelEvent");

    g_host = CreateWindowExW(WS_EX_TOOLWINDOW, kHostClass, L"Bateria panel",
                             WS_OVERLAPPED, 0, 0, 0, 0, NULL, NULL, inst, NULL);
    if (!g_host) return 1;

    makeFont(48);
    ensurePanel();
    SetTimer(g_host, kTimerId, kTimerMs, NULL);

    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    if (g_panel && IsWindow(g_panel)) DestroyWindow(g_panel);
    if (g_font) DeleteObject(g_font);
    return 0;
}
