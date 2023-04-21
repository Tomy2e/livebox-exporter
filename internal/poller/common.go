package poller

import "math"

const maxMbits = 2150

func sanitizeMbits(mbits float64) float64 {
	return math.Min(mbits, maxMbits)
}
