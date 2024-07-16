package bitrate

import (
	"time"
)

// Bitrate allows calculating bitrates for a set of network interfaces.
// This implementation is not thread-safe.
type Bitrate struct {
	measures                map[string]*measure
	minDelayBetweenMeasures time.Duration
}

// New returns a new bitrate measurer.
func New(minDelayBetweenMeasures time.Duration) *Bitrate {
	return &Bitrate{
		measures:                make(map[string]*measure),
		minDelayBetweenMeasures: minDelayBetweenMeasures,
	}
}

// mesure saves the counter values at a specific point in time.
type measure struct {
	Counters
	Last time.Time
}

// Counters contain Tx and Rx counters for a network interface.
type Counters struct {
	Tx, Rx uint64
}

// Swap swaps Tx and Rx counters.
func (c *Counters) Swap() {
	c.Rx, c.Tx = c.Tx, c.Rx
}

// Bitrates for Tx and Rx.
type Bitrates struct {
	// Tx bitrate, can be nil if not available.
	Tx *BitrateSpec
	// Rx bitrate, can be nil if not available.
	Rx *BitrateSpec
}

// BitrateSpec contains the value of the bitrate
type BitrateSpec struct {
	// Value of the bitrate (in Mbit/s). Will be 0 if Reset is true.
	Value float64
	// Reset is true when the counter was reset.
	Reset bool
}

// ShouldMeasure returns true if a measure should be done.
func (b *Bitrate) ShouldMeasure(name string) bool {
	last, ok := b.measures[name]
	if !ok {
		return true
	}

	return time.Since(last.Last) > b.minDelayBetweenMeasures
}

// Measure saves the current measure and returns the current RX/TX bitrates.
func (b *Bitrate) Measure(name string, current *Counters) *Bitrates {
	br := &Bitrates{}

	last, ok := b.measures[name]

	// Only calculate bitrates if there is a previous measure.
	if ok && !last.Last.IsZero() {
		elapsed := time.Since(last.Last)

		if elapsed.Seconds() > 0 && elapsed.Minutes() <= 6 {
			if diff := int64(current.Rx - last.Rx); diff >= 0 {
				br.Rx = &BitrateSpec{
					Value: BytesPerSecToMbits(float64(diff) / elapsed.Seconds()),
				}
			} else {
				br.Rx = &BitrateSpec{
					Reset: true,
				}
			}

			if diff := int64(current.Tx - last.Tx); diff >= 0 {
				br.Tx = &BitrateSpec{
					Value: BytesPerSecToMbits(float64(diff) / elapsed.Seconds()),
				}
			} else {
				br.Tx = &BitrateSpec{
					Reset: true,
				}
			}
		}

		// Sanitize bitrates: we assume bitrates cannot be above 10000 Mbit/s.
		if br.Rx != nil && br.Rx.Value > 10000 {
			br.Rx = nil
		}

		if br.Tx != nil && br.Tx.Value > 10000 {
			br.Tx = nil
		}
	}

	// Save this measure as the latest.
	b.measures[name] = &measure{
		Counters: *current,
		Last:     time.Now(),
	}

	return br
}
