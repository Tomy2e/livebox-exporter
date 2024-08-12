package collector

import (
	"log"
	"time"

	"github.com/Tomy2e/livebox-exporter/pkg/reflect"
)

const slowCollectThreshold = 10 * time.Second

func warnOnSlowCollect(collector any, startTime time.Time) {
	if ts := time.Since(startTime); ts > slowCollectThreshold {
		log.Printf("WARN: Collect was slow (%s) for %s", ts, reflect.GetType(collector))
	}
}
