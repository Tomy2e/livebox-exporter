package poller

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

// Poller is an interface that allows polling a system and updating Prometheus
// metrics.
type Poller interface {
	Poll(ctx context.Context) error
	Collectors() []prometheus.Collector
}

// Pollers is a list of pollers.
type Pollers []Poller

// Collectors returns the collectors of all pollers.
func (p Pollers) Collectors() (c []prometheus.Collector) {
	for _, poller := range p {
		c = append(c, poller.Collectors()...)
	}

	return
}

// Poll runs all pollers in parallel.
func (p Pollers) Poll(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	for _, poller := range p {
		poller := poller
		eg.Go(func() error {
			return poller.Poll(ctx)
		})
	}

	return eg.Wait()
}
