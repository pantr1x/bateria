//go:build windows

// Command bateria zobrazuje v oznamovacej oblasti Windowsu ikonu batérie
// s časom do plného nabitia a do vybitia.
//
// Prepínače:
//
//	-diag   vypíše, čo aplikácia v systéme vidí (keď sa ikona neobjaví)
//	-quit   ukončí bežiacu inštanciu
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/pantr1x/bateria/internal/win"
	"github.com/pantr1x/bateria/internal/winui"
)

func main() {
	// Bez konzoly by pád aplikácie nezanechal žiadnu stopu.
	defer func() {
		if r := recover(); r != nil {
			winui.Report(fmt.Errorf("neočakávaná chyba: %v", r), debug.Stack())
			os.Exit(2)
		}
	}()

	switch {
	case hasFlag("diag"):
		win.MessageBox("Batéria – diagnostika", winui.Diagnose(), win.MBIconInfo)
		return
	case hasFlag("quit", "stop", "ukonci"):
		if winui.Quit() {
			win.MessageBox("Batéria", "Bežiaca aplikácia bola ukončená.", win.MBIconInfo)
		} else {
			win.MessageBox("Batéria", "Žiadna bežiaca aplikácia sa nenašla.", win.MBIconInfo)
		}
		return
	}

	if err := winui.Run(); err != nil {
		winui.Report(fmt.Errorf("aplikáciu sa nepodarilo spustiť: %w", err), nil)
		os.Exit(1)
	}
}

// hasFlag prijíma prepínače písané ako -x, --x aj /x – podľa toho, ako je
// kto zvyknutý.
func hasFlag(names ...string) bool {
	for _, a := range os.Args[1:] {
		got := strings.TrimLeft(a, "-/")
		for _, n := range names {
			if strings.EqualFold(got, n) {
				return true
			}
		}
	}
	return false
}
