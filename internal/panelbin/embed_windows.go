//go:build windows

// Package panelbin nesie v sebe program bateria-panel.exe, ktorý kreslí text
// priamo v paneli úloh. Je zabalený v hlavnom programe, aby sa dal rozdávať
// jediný súbor – pri spustení sa rozbalí do priečinka aplikácie.
package panelbin

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed bateria-panel.exe
var binary []byte

// Name je meno rozbaleného súboru.
const Name = "bateria-panel.exe"

// Extract rozbalí program vedľa nastavení a vráti cestu k nemu. Keď tam už
// je v rovnakej veľkosti, nechá ho tak – práve bežiaci súbor sa vo Windowse
// prepísať nedá.
func Extract(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, Name)
	if info, err := os.Stat(path); err == nil && info.Size() == int64(len(binary)) {
		return path, nil
	}
	if err := os.WriteFile(path, binary, 0o755); err != nil {
		// Keď sa prepísať nedá (napr. beží), použijeme ten, čo tam je.
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
		return "", err
	}
	return path, nil
}
