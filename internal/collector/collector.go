package collector

import (
	"log"
	"reflect"
	"time"
)

const slowCollectThreshold = 10 * time.Second

func getType(myvar any) string {
	t := reflect.TypeOf(myvar)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	}

	return t.Name()
}

func warnOnSlowCollect(collector any, startTime time.Time) {
	if ts := time.Since(startTime); ts > slowCollectThreshold {
		log.Printf("WARN: Collect was slow (%s) for %s", ts, getType(collector))
	}
}
