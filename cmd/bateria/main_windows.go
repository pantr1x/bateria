//go:build windows

// Command bateria zobrazuje v oznamovacej oblasti Windowsu ikonu batérie
// s časom do plného nabitia a do vybitia.
//
// Prepínač -diag vypíše, čo aplikácia v systéme vidí; hodí sa, keď sa ikona
// v paneli neobjaví.
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

	if diagRequested() {
		win.MessageBox("Batéria – diagnostika", winui.Diagnose(), win.MBIconInfo)
		return
	}
	if err := winui.Run(); err != nil {
		winui.Report(fmt.Errorf("aplikáciu sa nepodarilo spustiť: %w", err), nil)
		os.Exit(1)
	}
}

// diagRequested prijíma -diag, --diag aj /diag – podľa toho, ako je kto
// zvyknutý písať prepínače vo Windowse.
func diagRequested() bool {
	for _, a := range os.Args[1:] {
		if strings.EqualFold(strings.TrimLeft(a, "-/"), "diag") {
			return true
		}
	}
	return false
}
