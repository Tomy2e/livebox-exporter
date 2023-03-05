package poller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
)

var _ Poller = &InterfaceMbits{}

// InterfaceMbits allows to poll the current bandwidth usage on the Livebox
// interfaces.
type InterfaceMbits struct {
	client                       livebox.Client
	txMbits, rxMbits             *prometheus.GaugeVec
	txMbitsNetDev, rxMbitsNetDev *prometheus.GaugeVec
	bytesSent, bytesReceived     *prometheus.CounterVec
	interfaces                   map[string]netInterface
	interfacesNetDev             map[string]netInterface
}

type netInterface struct {
	Flags          string
	LastTx, LastRx int64
	LastPoll       time.Time
}

func (ni *netInterface) IsWAN() bool {
	return strings.Contains(ni.Flags, "wan")
}

func (ni *netInterface) IsWLAN() bool {
	return strings.Contains(ni.Flags, "wlanvap")
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
		txMbitsNetDev: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_netdev_tx_mbits",
			Help: "Transmitted Mbits per second, calculated from netdevstats.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
		rxMbitsNetDev: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_interface_netdev_rx_mbits",
			Help: "Received Mbits per second, calculated from netdevstats.",
		}, []string{
			// Name of the interface.
			"interface",
		}),
		bytesSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "livebox_interface_bytes_sent_total",
			Help: "Bytes sent on the interface",
		}, []string{
			"interface",
		}),
		bytesReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "livebox_interface_bytes_received_total",
			Help: "Bytes received on the interface",
		}, []string{
			"interface",
		}),
		interfaces:       make(map[string]netInterface),
		interfacesNetDev: make(map[string]netInterface),
	}
}

// Collectors returns all metrics.
func (im *InterfaceMbits) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		im.txMbits,
		im.rxMbits,
		im.txMbitsNetDev,
		im.rxMbitsNetDev,
		im.bytesSent,
		im.bytesReceived,
	}
}

func bitsPer30SecsToMbitsPerSec(v int) float64 {
	return float64(v) / 30000000
}

func (im *InterfaceMbits) discoverInterfaces(ctx context.Context) error {
	var mibs struct {
		Status struct {
			//GPON map[string]struct{} `json:"gpon"`
			Base map[string]struct {
				Flags string `json:"flags"`
			} `json:"base"`
		} `json:"status"`
	}

	if err := im.client.Request(
		ctx,
		request.New("NeMo.Intf.data", "getMIBs", map[string]interface{}{
			"traverse": "all",
			"flag":     "statmon && !vlan",
		}),
		&mibs,
	); err != nil {
		return fmt.Errorf("failed to discover interface: %w", err)
	}

	if len(mibs.Status.Base) == 0 {
		return errors.New("wan interface not found")
	}

	for itf, val := range mibs.Status.Base {
		im.interfaces[itf] = netInterface{Flags: val.Flags}
		im.interfacesNetDev[itf] = netInterface{Flags: val.Flags}
	}

	return nil
}

func bytesPerSecToMbits(bytes float64) float64 {
	return bytes * 8 / 1000000
}

// Poll polls the current bandwidth usage.
func (im *InterfaceMbits) Poll(ctx context.Context) error {
	if len(im.interfaces) == 0 {
		if err := im.discoverInterfaces(ctx); err != nil {
			return err
		}
	}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return im.pollInterfaces(ctx)
	})

	eg.Go(func() error {
		return im.pollInterfacesNetDev(ctx)
	})

	return eg.Wait()
}

func (im *InterfaceMbits) pollInterfaces(ctx context.Context) error {
	for itf, val := range im.interfaces {
		elapsed := time.Now().Sub(val.LastPoll)
		if elapsed.Seconds() < 30 {
			// Polling must only be done once every 30 seconds Livebox updates data
			// only every 30 seconds.
			continue
		}

		var stats struct {
			Status struct {
				BytesReceived int64 `json:"BytesReceived"`
				BytesSent     int64 `json:"BytesSent"`
			} `json:"status"`
		}

		if err := im.client.Request(ctx, request.New(
			fmt.Sprintf("HomeLan.Interface.%s.Stats", itf),
			"get",
			nil,
		), &stats); err != nil {
			return err
		}

		rxMetric := im.rxMbits
		txMetric := im.txMbits
		brMetric := im.bytesReceived
		bsMetric := im.bytesSent

		if !val.IsWAN() {
			rxMetric = im.txMbits
			txMetric = im.rxMbits
			brMetric = im.bytesSent
			bsMetric = im.bytesReceived
		}

		if !val.LastPoll.IsZero() {
			if elapsed.Seconds() > 0 {
				if stats.Status.BytesReceived >= val.LastRx {
					diff := float64(stats.Status.BytesReceived - val.LastRx)
					rxMetric.
						With(prometheus.Labels{"interface": itf}).
						Set(bytesPerSecToMbits(diff / (elapsed.Seconds())))
					brMetric.With(prometheus.Labels{"interface": itf}).Add(diff)
				} else {
					// Counter was reset?
					brMetric.Reset()
					brMetric.With(prometheus.Labels{"interface": itf}).Add(float64(stats.Status.BytesReceived))
				}

				if stats.Status.BytesSent >= val.LastTx {
					diff := float64(stats.Status.BytesSent - val.LastTx)
					txMetric.
						With(prometheus.Labels{"interface": itf}).
						Set(bytesPerSecToMbits(diff / (elapsed.Seconds())))
					bsMetric.With(prometheus.Labels{"interface": itf}).Add(diff)

				} else {
					// Counter was reset?
					bsMetric.Reset()
					bsMetric.With(prometheus.Labels{"interface": itf}).Add(float64(stats.Status.BytesSent))
				}
			}
		} else {
			// Initialize bytes
			bsMetric.With(prometheus.Labels{"interface": itf}).Add(float64(stats.Status.BytesSent))
			brMetric.With(prometheus.Labels{"interface": itf}).Add(float64(stats.Status.BytesReceived))
		}

		val.LastTx = stats.Status.BytesSent
		val.LastRx = stats.Status.BytesReceived

		val.LastPoll = time.Now()
		im.interfaces[itf] = val
	}

	return nil
}

func (im *InterfaceMbits) pollInterfacesNetDev(ctx context.Context) error {
	for itf, val := range im.interfacesNetDev {
		var stats struct {
			Status struct {
				RxBytes int64 `json:"RxBytes"`
				TxBytes int64 `json:"TxBytes"`
			} `json:"status"`
		}

		if err := im.client.Request(ctx, request.New(
			fmt.Sprintf("NeMo.Intf.%s", itf),
			"getNetDevStats",
			nil,
		), &stats); err != nil {
			return err
		}

		rxMetric := im.rxMbitsNetDev
		txMetric := im.txMbitsNetDev

		if !val.IsWAN() {
			rxMetric = im.txMbitsNetDev
			txMetric = im.rxMbitsNetDev
		}

		if !val.LastPoll.IsZero() {
			elapsed := time.Now().Sub(val.LastPoll)
			if elapsed.Seconds() > 0 {
				if stats.Status.RxBytes >= val.LastRx {
					rxMetric.
						With(prometheus.Labels{"interface": itf}).
						Set(8 * float64(stats.Status.RxBytes-val.LastRx) / (elapsed.Seconds() * 1000000))
				}

				if stats.Status.TxBytes >= val.LastTx {
					txMetric.
						With(prometheus.Labels{"interface": itf}).
						Set(8 * float64(stats.Status.TxBytes-val.LastTx) / (elapsed.Seconds() * 1000000))
				}

			}
		}

		val.LastRx = stats.Status.RxBytes
		val.LastTx = stats.Status.TxBytes
		val.LastPoll = time.Now()
		im.interfacesNetDev[itf] = val
	}

	return nil
}
