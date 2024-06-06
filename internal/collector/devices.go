package collector

import (
	"context"
	"log"
	"sync"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
)

type Devices struct {
	client             *livebox.Client
	deviceCounters     sync.Map
	deviceActive       *prometheus.Desc
	deviceRx, deviceTx *prometheus.Desc
}

func NewDevices(client *livebox.Client) *Devices {
	d := &Devices{
		client: client,
		deviceActive: prometheus.NewDesc(
			"livebox_device_active",
			"Status of the device.",
			[]string{"name", "type", "mac"},
			nil,
		),
		deviceRx: prometheus.NewDesc(
			"livebox_device_rx_bytes_total",
			"Bytes received by device.",
			[]string{"name", "type", "mac"},
			nil,
		),
		deviceTx: prometheus.NewDesc(
			"livebox_device_tx_bytes_total",
			"Bytes transmitted by device.",
			[]string{"name", "type", "mac"},
			nil,
		),
	}

	go d.startEventObserver()

	return d
}

type deviceStatistics struct {
	RxBytes float64
	TxBytes float64
}

func (d *Devices) startEventObserver() {
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
			var ds deviceStatistics
			if err := mapstructure.Decode(attr, &ds); err != nil {
				continue
			}
			d.deviceCounters.Store(mac, ds)
		}
	}
}

// Describe currently does nothing.
func (d *Devices) Describe(_ chan<- *prometheus.Desc) {}

// Collect collects all Devices metrics.
func (d *Devices) Collect(c chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.TODO(), collectTimeout)
	defer cancel()

	var devices struct {
		Status []struct {
			Key        string `json:"Key"`
			Name       string `json:"Name"`
			DeviceType string `json:"DeviceType"`
			Active     bool   `json:"Active"`
		} `json:"status"`
	}

	if err := d.client.Request(
		ctx,
		request.New("Devices", "get", request.Parameters{"expression": ".DeviceType!=\"\" and .DeviceType!=\"SAH HGW\""}),
		&devices,
	); err != nil {
		log.Printf("WARN: Devices collector: failed to get interfaces: %s", err)
		return
	}

	for _, device := range devices.Status {
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

		if ds, ok := d.deviceCounters.Load(device.Key); ok {
			c <- prometheus.MustNewConstMetric(
				d.deviceRx,
				prometheus.GaugeValue,
				ds.(deviceStatistics).RxBytes,
				device.Name,
				device.DeviceType,
				device.Key,
			)
			c <- prometheus.MustNewConstMetric(
				d.deviceTx,
				prometheus.GaugeValue,
				ds.(deviceStatistics).TxBytes,
				device.Name,
				device.DeviceType,
				device.Key,
			)
		}
	}
}
