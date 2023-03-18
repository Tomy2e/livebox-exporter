package poller

import (
	"context"
	"fmt"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-exporter/pkg/bitrate"
	"github.com/prometheus/client_golang/prometheus"
)

var _ Poller = &InterfaceMbits{}

// InterfaceMbits allows to poll the current bandwidth usage on the Livebox
// interfaces.
type InterfaceMbits struct {
	client           livebox.Client
	txMbits, rxMbits *prometheus.GaugeVec
}

// NewInterfaceMbits returns a new InterfaceMbits poller.
func NewInterfaceMbits(client livebox.Client) *InterfaceMbits {
	return &InterfaceMbits{
		client: client,
		txMbits: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_tx_mbits",
			Help: "Transmitted Mbits per second.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
		rxMbits: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_rx_mbits",
			Help: "Received Mbits per second.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
	}
}

// Collectors returns all metrics.
func (im *InterfaceMbits) Collectors() []prometheus.Collector {
	return []prometheus.Collector{im.txMbits, im.rxMbits}
}

// Poll polls the current bandwidth usage.
func (im *InterfaceMbits) Poll(ctx context.Context) error {
	var counters struct {
		Status map[string]struct {
			Traffic []struct {
				Timestamp int `json:"Timestamp"`
				RxCounter int `json:"Rx_Counter"`
				TxCounter int `json:"Tx_Counter"`
			} `json:"Traffic"`
		} `json:"status"`
	}

	// Request latest rx/tx counters.
	if err := im.client.Request(
		ctx,
		request.New(
			"HomeLan",
			"getResults",
			map[string]interface{}{
				"Seconds":          0,
				"NumberOfReadings": 1,
			},
		),
		&counters,
	); err != nil {
		return fmt.Errorf("failed to get interfaces: %w", err)
	}

	for iface, traffic := range counters.Status {
		rxCounter := 0
		txCounter := 0

		if len(traffic.Traffic) > 0 {
			rxCounter = traffic.Traffic[0].RxCounter
			txCounter = traffic.Traffic[0].TxCounter
		}

		im.rxMbits.
			With(prometheus.Labels{"interface": iface}).
			Set(bitrate.BitsPer30SecsToMbits(rxCounter))
		im.txMbits.
			With(prometheus.Labels{"interface": iface}).
			Set(bitrate.BitsPer30SecsToMbits(txCounter))
	}

	return nil
}
