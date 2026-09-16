//go:build windows

// Command bateria zobrazuje v oznamovacej oblasti Windowsu ikonu batérie
// s časom do plného nabitia a do vybitia.
package main

import (
	"os"

	"github.com/pantr1x/bateria/internal/winui"
)

func main() {
	if err := winui.Run(); err != nil {
		os.Exit(1)
	}
}
