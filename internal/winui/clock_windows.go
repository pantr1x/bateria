//go:build windows

package winui

import (
	"fmt"
	"math"

	"github.com/pantr1x/bateria/internal/battery"
)

// clockText je text, ktorý sa vloží do hodín v paneli úloh. Farbu preberá
// od hodín, takže sa tu rieši len obsah.
func clockText(st battery.Status) string {
	if !st.Present {
		return "bez batérie"
	}
	if _, d, ok := st.Remaining(); ok {
		return battery.FormatDuration(d)
	}
	if st.RuntimeOnBattery > 0 {
		// Plná batéria v sieti: ukážeme, ako dlho by vydržala po odpojení.
		return "~ " + battery.FormatDuration(st.RuntimeOnBattery)
	}
	return fmt.Sprintf("%d %%", int(math.Round(st.Percent)))
}
