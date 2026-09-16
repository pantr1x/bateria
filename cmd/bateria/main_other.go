//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "Bateria je aplikácia pre Windows.")
	os.Exit(1)
}
