package rinse

import (
	"fmt"
	"math"
)

func prettyByteSize(b int64) string {
	if b < 1000 {
		return fmt.Sprintf("%dB", b)
	}
	bf := float64(b)
	for _, unit := range "KMGTPEZ" {
		bf /= 1024.0
		if math.Abs(bf) < 10 {
			return fmt.Sprintf("%1.2f%cB", bf, unit)
		}
		if math.Abs(bf) < 100 {
			return fmt.Sprintf("%2.1f%cB", bf, unit)
		}
		if math.Abs(bf) < 1000 {
			return fmt.Sprintf("%3.0f%cB", bf, unit)
		}
	}
	return fmt.Sprintf("%.1fYB", bf)
}
