package poller

import (
	"context"
	"fmt"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-exporter/pkg/bitrate"
	exporterLivebox "github.com/Tomy2e/livebox-exporter/pkg/livebox"
	"github.com/prometheus/client_golang/prometheus"
)

// InterfaceHomeLanMbitsMinDelay set the minimum delay between each poll.
// Polling must only be done once every 30 seconds as Livebox updates data
// only every 30 seconds.
const InterfaceHomeLanMbitsMinDelay = 30 * time.Second

var _ Poller = &InterfaceHomeLanMbits{}

// InterfaceHomeLanMbits is an experimental poller to get the current bandwidth
// usage on the Livebox interfaces.
type InterfaceHomeLanMbits struct {
	client           livebox.Client
	interfaces       []*exporterLivebox.Interface
	bitrate          *bitrate.Bitrate
	txMbits, rxMbits *prometheus.GaugeVec
}

// NewInterfaceHomeLanMbits returns a new InterfaceMbits poller.
func NewInterfaceHomeLanMbits(client livebox.Client, interfaces []*exporterLivebox.Interface) *InterfaceHomeLanMbits {
	return &InterfaceHomeLanMbits{
		client:     client,
		interfaces: interfaces,
		bitrate:    bitrate.New(InterfaceHomeLanMbitsMinDelay),
		txMbits: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_homelan_tx_mbits",
			Help: "Transmitted Mbits per second.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
		rxMbits: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_homelan_rx_mbits",
			Help: "Received Mbits per second.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
	}
}

// Collectors returns all metrics.
func (im *InterfaceHomeLanMbits) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		im.txMbits,
		im.rxMbits,
	}
}

// Poll polls the current bandwidth usage.
func (im *InterfaceHomeLanMbits) Poll(ctx context.Context) error {
	for _, itf := range im.interfaces {
		// Enforce InterfaceHomeLanMbitsMinDelay.
		if !im.bitrate.ShouldMeasure(itf.Name) {
			continue
		}

		var stats struct {
			Status struct {
				BytesReceived uint64 `json:"BytesReceived"`
				BytesSent     uint64 `json:"BytesSent"`
			} `json:"status"`
		}

		if err := im.client.Request(ctx, request.New(
			fmt.Sprintf("HomeLan.Interface.%s.Stats", itf.Name),
			"get",
			nil,
		), &stats); err != nil {
			return err
		}

		counters := &bitrate.Counters{
			Tx: stats.Status.BytesSent,
			Rx: stats.Status.BytesReceived,
		}

		if itf.IsWAN() {
			counters.Swap()
		}

		bitrates := im.bitrate.Measure(itf.Name, counters)

		if bitrates.Rx != nil && !bitrates.Rx.Reset {
			im.rxMbits.
				With(prometheus.Labels{"interface": itf.Name}).
				Set(sanitizeMbits(bitrates.Rx.Value))
		}

		if bitrates.Tx != nil && !bitrates.Tx.Reset {
			im.txMbits.
				With(prometheus.Labels{"interface": itf.Name}).
				Set(sanitizeMbits(bitrates.Tx.Value))
		}
	}

	return nil
}
