package naming

import (
	"fmt"
	"strings"

	"github.com/Roham1299/snag/internal/clip"
)

// Clipped names a clip "Title (1.02-1.40)" (dots, as Windows forbids colons). name is a safe
// name without extension, shortened so the range always shows.
func Clipped(name string, r clip.Range) string {
	part := " (" + clipTime(r.From) + "-" + clipTime(r.To) + ")"
	name = strings.TrimRight(truncateUTF8(name, maxNameBytes-len(part)), " ")
	return name + part
}

// clipTime is a moment in a video as m.ss, or h.mm.ss from an hour on.
func clipTime(seconds float64) string {
	t := int(max(seconds, 0))
	if t >= 3600 {
		return fmt.Sprintf("%d.%02d.%02d", t/3600, t/60%60, t%60)
	}
	return fmt.Sprintf("%d.%02d", t/60, t%60)
}
