//go:build !windows

package battery

import "errors"

// ErrUnsupported znamená, že na tejto platforme sa stav batérie nečíta.
// Aplikácia je pre Windows; ostatné platformy sú tu preto, aby sa dala
// logika odhadov a kreslenia ikony zostaviť a otestovať aj inde.
var ErrUnsupported = errors.New("čítanie batérie je podporované len vo Windowse")

// Read vráti chybu mimo Windowsu.
func Read() (Status, error) { return Status{}, ErrUnsupported }
