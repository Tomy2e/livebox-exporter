package collector

import (
	"context"
	"log"
	"sync"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = &DeviceInfo{}

// DeviceInfo implements a prometheus Collector that returns Livebox specific metrics.
type DeviceInfo struct {
	client                livebox.Client
	numberOfRebootsMetric *prometheus.Desc
	uptimeMetric          *prometheus.Desc
	memoryTotalMetric     *prometheus.Desc
	memoryUsageMetric     *prometheus.Desc
}

// NewDeviceInfo returns a new DeviceInfo collector using the specified client.
func NewDeviceInfo(client livebox.Client) *DeviceInfo {
	return &DeviceInfo{
		client: client,
		numberOfRebootsMetric: prometheus.NewDesc(
			"livebox_deviceinfo_reboots_total",
			"Number of Livebox reboots.",
			nil, nil,
		),
		uptimeMetric: prometheus.NewDesc(
			"livebox_deviceinfo_uptime_seconds_total",
			"Livebox current uptime.",
			nil, nil,
		),
		memoryTotalMetric: prometheus.NewDesc(
			"livebox_deviceinfo_memory_total_bytes",
			"Livebox system total memory.",
			nil, nil,
		),
		memoryUsageMetric: prometheus.NewDesc(
			"livebox_deviceinfo_memory_usage_bytes",
			"Livebox system used memory.",
			nil, nil,
		),
	}
}

// Describe currently does nothing.
func (d *DeviceInfo) Describe(c chan<- *prometheus.Desc) {}

func (d *DeviceInfo) deviceInfo(c chan<- prometheus.Metric) {
	var deviceInfo struct {
		Status struct {
			NumberOfReboots float64 `json:"NumberOfReboots"`
			UpTime          float64 `json:"UpTime"`
		} `json:"status"`
	}
	if err := d.client.Request(context.TODO(), request.New("DeviceInfo", "get", nil), &deviceInfo); err != nil {
		log.Printf("WARN: DeviceInfo collector failed: %s", err)
		return
	}

	c <- prometheus.MustNewConstMetric(d.numberOfRebootsMetric, prometheus.GaugeValue, deviceInfo.Status.NumberOfReboots)
	c <- prometheus.MustNewConstMetric(d.uptimeMetric, prometheus.GaugeValue, deviceInfo.Status.UpTime)
}

func (d *DeviceInfo) memoryStatus(c chan<- prometheus.Metric) {
	var memoryStatus struct {
		Status struct {
			Total float64 `json:"Total"`
			Free  float64 `json:"Free"`
		} `json:"status"`
	}
	if err := d.client.Request(context.TODO(), request.New("DeviceInfo.MemoryStatus", "get", nil), &memoryStatus); err != nil {
		log.Printf("WARN: MemoryStatus collector failed: %s", err)
		return
	}

	c <- prometheus.MustNewConstMetric(d.memoryTotalMetric, prometheus.GaugeValue, 1000*memoryStatus.Status.Total)
	c <- prometheus.MustNewConstMetric(d.memoryUsageMetric, prometheus.GaugeValue, 1000*(memoryStatus.Status.Total-memoryStatus.Status.Free))
}

// Collect collects all DeviceInfo metrics.
func (d *DeviceInfo) Collect(c chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		d.deviceInfo(c)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		d.memoryStatus(c)
		wg.Done()
	}()
	wg.Wait()
}
