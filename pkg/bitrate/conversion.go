package bitrate

// BitsPer30SecsToMbits converts bits/30secs to Mbit/s.
func BitsPer30SecsToMbits(v int) float64 {
	return float64(v) / 30000000
}

// BytesPerSecToMbits converts B/s to Mbit/s.
func BytesPerSecToMbits(bytes float64) float64 {
	return bytes * 8 / 1000000
}
