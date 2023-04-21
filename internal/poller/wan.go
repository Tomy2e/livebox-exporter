package poller

import (
	"context"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-exporter/pkg/bitrate"
	"github.com/prometheus/client_golang/prometheus"
)

var _ Poller = &WANMbits{}

// WANMbits is an experimental poller to get the current bandwidth usage on the
// WAN interface of the Livebox.
type WANMbits struct {
	client           livebox.Client
	bitrate          *bitrate.Bitrate
	txMbits, rxMbits prometheus.Gauge
}

// NewWANMbits returns a new WANMbits poller.
func NewWANMbits(client livebox.Client) *WANMbits {
	return &WANMbits{
		client:  client,
		bitrate: bitrate.New(InterfaceHomeLanMbitsMinDelay),
		txMbits: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "livebox_wan_tx_mbits",
			Help: "Transmitted Mbits per second on the WAN interface.",
		}),
		rxMbits: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "livebox_wan_rx_mbits",
			Help: "Received Mbits per second on the WAN interface.",
		}),
	}
}

// Collectors returns all metrics.
func (im *WANMbits) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		im.txMbits,
		im.rxMbits,
	}
}

// Poll polls the current bandwidth usage on the WAN interface.
func (im *WANMbits) Poll(ctx context.Context) error {
	var stats struct {
		Status struct {
			BytesReceived uint64 `json:"BytesReceived"`
			BytesSent     uint64 `json:"BytesSent"`
		} `json:"status"`
	}

	if err := im.client.Request(
		ctx,
		request.New("HomeLan", "getWANCounters", nil),
		&stats,
	); err != nil {
		return err
	}

	counters := &bitrate.Counters{
		Tx: stats.Status.BytesSent,
		Rx: stats.Status.BytesReceived,
	}

	counters.Swap()

	bitrates := im.bitrate.Measure("WAN", counters)

	if bitrates.Rx != nil && !bitrates.Rx.Reset {
		im.rxMbits.Set(sanitizeMbits(bitrates.Rx.Value))
	}

	if bitrates.Tx != nil && !bitrates.Tx.Reset {
		im.txMbits.Set(sanitizeMbits(bitrates.Tx.Value))
	}

	return nil
}
