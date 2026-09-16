//go:build windows

package winui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/pantr1x/bateria/internal/win"
)

// Aplikácia beží bez konzoly, takže bežný pád by sa prejavil len tým, že
// ikona zmizne – bez akejkoľvek správy. Preto sa chyba ukáže v okne a zapíše
// do súboru v priečinku dočasných súborov.

var reported bool

// guard zachytí pád v obsluhe správ okna, ohlási ho a aplikáciu korektne
// ukončí. Volá sa ako `defer guard("…")`.
func guard(where string) {
	r := recover()
	if r == nil {
		return
	}
	Report(fmt.Errorf("%s: %v", where, r), debug.Stack())
	win.PostQuit(2)
}

// Report ohlási chybu používateľovi a zapíše ju do súboru. Druhý a ďalší
// raz už okno neotvára, nech sa správy nekopia na seba.
func Report(err error, stack []byte) {
	if err == nil {
		return
	}
	text := fmt.Sprintf("%s\n%v\n\n%s\n", time.Now().Format(time.RFC3339), err, stack)
	path := filepath.Join(os.TempDir(), "bateria-chyba.txt")
	if f, ferr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
		f.WriteString(text)
		f.Close()
	}
	if reported {
		return
	}
	reported = true
	win.MessageBox("Batéria", fmt.Sprintf("%v\n\nPodrobnosti sú v súbore:\n%s", err, path),
		win.MBIconError)
}
