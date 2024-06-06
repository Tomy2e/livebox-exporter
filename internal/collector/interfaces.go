package collector

import (
	"context"
	"log"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-exporter/pkg/bitrate"
	"github.com/prometheus/client_golang/prometheus"
)

type Interfaces struct {
	client           *livebox.Client
	txMbits, rxMbits *prometheus.Desc
}

func NewInterfaces(client *livebox.Client) *Interfaces {
	return &Interfaces{
		client: client,
		txMbits: prometheus.NewDesc(
			"livebox_interface_tx_mbits",
			"Transmitted Mbits per second.",
			[]string{
				// Name of the interface.
				"interface",
			}, nil,
		),
		rxMbits: prometheus.NewDesc(
			"livebox_interface_rx_mbits",
			"Received Mbits per second.",
			[]string{
				// Name of the interface.
				"interface",
			}, nil,
		),
	}
}

// Describe currently does nothing.
func (i *Interfaces) Describe(_ chan<- *prometheus.Desc) {}

// Collect collects all Interfaces metrics.
func (i *Interfaces) Collect(c chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.TODO(), collectTimeout)
	defer cancel()

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
	if err := i.client.Request(
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
		log.Printf("WARN: failed to get interfaces: %s", err)
		return
	}

	for iface, traffic := range counters.Status {
		rxCounter := 0
		txCounter := 0

		if len(traffic.Traffic) > 0 {
			rxCounter = traffic.Traffic[0].RxCounter
			txCounter = traffic.Traffic[0].TxCounter
		}

		c <- prometheus.MustNewConstMetric(i.rxMbits, prometheus.GaugeValue, bitrate.BitsPer30SecsToMbits(rxCounter), iface)
		c <- prometheus.MustNewConstMetric(i.txMbits, prometheus.GaugeValue, bitrate.BitsPer30SecsToMbits(txCounter), iface)
	}
}
