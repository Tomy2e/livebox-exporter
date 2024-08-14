package poller

import (
	"context"
	"fmt"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-exporter/pkg/bitrate"
	exporterLivebox "github.com/Tomy2e/livebox-exporter/pkg/livebox"
	"github.com/prometheus/client_golang/prometheus"
)

var _ Poller = &InterfaceNetDevMbits{}

// InterfaceNetDevMbits is an experimental poller to get the current bandwidth
// usage on the Livebox interfaces.
type InterfaceNetDevMbits struct {
	client           *livebox.Client
	interfaces       []*exporterLivebox.Interface
	bitrate          *bitrate.Bitrate
	txMbits, rxMbits *prometheus.GaugeVec
}

// NewInterfaceNetDevMbits returns a new InterfaceNetDevMbits poller.
func NewInterfaceNetDevMbits(client *livebox.Client, interfaces []*exporterLivebox.Interface) *InterfaceNetDevMbits {
	return &InterfaceNetDevMbits{
		client:     client,
		interfaces: interfaces,
		bitrate:    bitrate.New(0),
		txMbits: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_netdev_tx_mbits",
			Help: "Transmitted Mbits per second.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
		rxMbits: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_netdev_rx_mbits",
			Help: "Received Mbits per second.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
	}
}

// Collectors returns all metrics.
func (im *InterfaceNetDevMbits) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		im.txMbits,
		im.rxMbits,
	}
}

func (im *InterfaceNetDevMbits) getNetDevStats(ctx context.Context, interfaceName string) (uint64, uint64, error) {
	var stats struct {
		Status struct {
			RxBytes uint64
			TxBytes uint64
		} `json:"status"`
	}

	if err := im.client.Request(ctx, request.New(
		fmt.Sprintf("NeMo.Intf.%s", interfaceName),
		"getNetDevStats",
		nil,
	), &stats); err != nil {
		return 0, 0, err
	}

	return stats.Status.RxBytes, stats.Status.TxBytes, nil
}

func (im *InterfaceNetDevMbits) getSSIDStats(ctx context.Context, interfaceName string) (uint64, uint64, error) {
	var stats struct {
		Status struct {
			BytesReceived uint64
			BytesSent     uint64
		} `json:"status"`
	}

	if err := im.client.Request(ctx, request.New(
		fmt.Sprintf("NeMo.Intf.%s", interfaceName),
		"getSSIDStats",
		nil,
	), &stats); err != nil {
		return 0, 0, err
	}

	return stats.Status.BytesReceived, stats.Status.BytesSent, nil
}

// Poll polls the current bandwidth usage.
func (im *InterfaceNetDevMbits) Poll(ctx context.Context) error {
	for _, itf := range im.interfaces {
		var (
			counters = &bitrate.Counters{}
			err      error
		)

		if itf.IsWLAN() {
			counters.Rx, counters.Tx, err = im.getSSIDStats(ctx, itf.Name)
		} else {
			counters.Rx, counters.Tx, err = im.getNetDevStats(ctx, itf.Name)
		}
		if err != nil {
			return fmt.Errorf("failed to get stats for interface (WLAN=%t): %s: %w", itf.IsWLAN(), itf.Name, err)
		}

		if !itf.IsWAN() {
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
