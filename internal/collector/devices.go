package collector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-exporter/pkg/bitrate"
	exporterLivebox "github.com/Tomy2e/livebox-exporter/pkg/livebox"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
)

type Devices struct {
	client                       *livebox.Client
	deviceRates                  sync.Map
	wifiDeviceRates              sync.Map
	deviceActive                 *prometheus.Desc
	deviceRxMbits, deviceTxMbits *prometheus.Desc
}

type rates struct {
	Tx, Rx float64
}

func NewDevices(client *livebox.Client, interfaces []*exporterLivebox.Interface) *Devices {
	d := &Devices{
		client: client,
		deviceActive: prometheus.NewDesc(
			"livebox_device_active",
			"Status of the device.",
			[]string{"name", "type", "mac"},
			nil,
		),
		deviceRxMbits: prometheus.NewDesc(
			"livebox_device_rx_mbits",
			"Received Mbits per second by device.",
			[]string{"name", "type", "mac", "source"},
			nil,
		),
		deviceTxMbits: prometheus.NewDesc(
			"livebox_device_tx_mbits",
			"Transmitted Mbits per second by device.",
			[]string{"name", "type", "mac", "source"},
			nil,
		),
	}

	go d.startEventsObserver()
	go d.startStationStatsPoller(interfaces)

	return d
}

func (d *Devices) startEventsObserver() {
	br := bitrate.New(0)
	events := d.client.Events(context.TODO(), []string{"Devices.Device"})

	for evt := range events {
		if evt.Error != nil {
			log.Printf("WARN: event error: %s", evt.Error)
			continue
		}
		if evt.Event.Object.Reason != "Statistics" {
			continue
		}

		for mac, attr := range evt.Event.Object.Attributes {
			var ds struct {
				RxBytes uint64
				TxBytes uint64
			}

			if err := mapstructure.Decode(attr, &ds); err != nil {
				continue
			}

			bitrates := br.Measure(mac, &bitrate.Counters{
				Tx: ds.TxBytes,
				Rx: ds.RxBytes,
			})

			if bitrates.Rx != nil && bitrates.Tx != nil {
				d.deviceRates.Store(mac, &rates{
					Tx: bitrates.Tx.Value,
					Rx: bitrates.Rx.Value,
				})
			}

		}
	}
}

func (d *Devices) startStationStatsPoller(interfaces []*exporterLivebox.Interface) {
	br := bitrate.New(0)

	for {
		for _, itf := range interfaces {
			// Skip non-wifi interfaces.
			if !itf.IsWLAN() {
				continue
			}

			var stats struct {
				Status []struct {
					MACAddress string `json:"MACAddress"`
					RxBytes    uint64 `json:"RxBytes"`
					TxBytes    uint64 `json:"TxBytes"`
				} `json:"status"`
			}

			if err := d.client.Request(context.TODO(), request.New(
				fmt.Sprintf("NeMo.Intf.%s", itf.Name),
				"getStationStats",
				nil,
			), &stats); err != nil {
				log.Printf("WARN: getStationStats error: %s", err)
				continue
			}

			for _, stationStats := range stats.Status {
				bitrates := br.Measure(stationStats.MACAddress, &bitrate.Counters{
					// Tx and Rx are swapped here.
					Tx: stationStats.RxBytes,
					Rx: stationStats.TxBytes,
				})

				// Load existing rates or initialize.
				r, ok := d.wifiDeviceRates.Load(stationStats.MACAddress)
				if !ok {
					r = &rates{}
				}

				if bitrates.Rx != nil && !bitrates.Rx.Reset {
					r.(*rates).Rx = bitrates.Rx.Value
				}

				if bitrates.Tx != nil && !bitrates.Tx.Reset {
					r.(*rates).Tx = bitrates.Tx.Value
				}

				// Persist rates.
				d.wifiDeviceRates.Store(stationStats.MACAddress, r)
			}
		}

		time.Sleep(5 * time.Second)
	}
}

// Describe currently does nothing.
func (d *Devices) Describe(_ chan<- *prometheus.Desc) {}

// Collect collects all Devices metrics.
func (d *Devices) Collect(c chan<- prometheus.Metric) {
	defer warnOnSlowCollect(d, time.Now())

	var devices struct {
		Status []struct {
			Key        string `json:"Key"`
			Name       string `json:"Name"`
			DeviceType string `json:"DeviceType"`
			Active     bool   `json:"Active"`
		} `json:"status"`
	}

	if err := d.client.Request(
		context.TODO(),
		request.New("Devices", "get", request.Parameters{"expression": ".DeviceType!=\"\" and .DeviceType!=\"SAH HGW\""}),
		&devices,
	); err != nil {
		log.Printf("WARN: Devices collector: failed to get interfaces: %s", err)
		return
	}

	for _, device := range devices.Status {
		// Quick check to skip devices without a MAC address.
		if !strings.Contains(device.Key, ":") {
			continue
		}

		var active float64
		if device.Active {
			active = 1
		}

		c <- prometheus.MustNewConstMetric(
			d.deviceActive,
			prometheus.GaugeValue,
			active,
			device.Name,
			device.DeviceType,
			device.Key,
		)

		// Skip sending other metrics if device is not active.
		if !device.Active {
			continue
		}

		// Try to get wifi rates first as they're more accurate.
		source := "stationStats"
		r, ok := d.wifiDeviceRates.Load(device.Key)
		if !ok {
			// Try to get rates obtained through events (less accurate).
			r, ok = d.deviceRates.Load(device.Key)
			if !ok {
				// Skip if no rates found.
				continue
			}
			source = "events"
		}

		c <- prometheus.MustNewConstMetric(
			d.deviceRxMbits,
			prometheus.GaugeValue,
			float64(r.(*rates).Rx),
			device.Name,
			device.DeviceType,
			device.Key,
			source,
		)
		c <- prometheus.MustNewConstMetric(
			d.deviceTxMbits,
			prometheus.GaugeValue,
			float64(r.(*rates).Tx),
			device.Name,
			device.DeviceType,
			device.Key,
			source,
		)
	}
}
