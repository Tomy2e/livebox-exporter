package collector

import (
	"context"
	"log"
	"slices"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	exporterLivebox "github.com/Tomy2e/livebox-exporter/pkg/livebox"
	"github.com/prometheus/client_golang/prometheus"
)

const gponInterfaceName = "veip0"

// ONT implements a prometheus Collector that returns ONT specific metrics.
type ONT struct {
	client *livebox.Client

	enabled bool

	temperatureMetric        *prometheus.Desc
	downstreamCurrRateMetric *prometheus.Desc
	upstreamCurrRateMetric   *prometheus.Desc
}

// NewONT returns a new ONT collector using the specified client.
func NewONT(client *livebox.Client, interfaces []*exporterLivebox.Interface) *ONT {
	return &ONT{
		client: client,
		// Do not enable this collector if veip0 interface is not found.
		enabled: slices.ContainsFunc(interfaces, func(itf *exporterLivebox.Interface) bool { return itf.Name == gponInterfaceName }),
		temperatureMetric: prometheus.NewDesc(
			"livebox_ont_temperature_celsius",
			"Current ONT temperature.",
			nil, nil,
		),
		downstreamCurrRateMetric: prometheus.NewDesc(
			"livebox_ont_downstream_current_rate_bytes",
			"Current ONT downstream rate.",
			nil, nil,
		),
		upstreamCurrRateMetric: prometheus.NewDesc(
			"livebox_ont_upstream_current_rate_bytes",
			"Current ONT upstream rate.",
			nil, nil,
		),
	}
}

// Describe currently does nothing.
func (d *ONT) Describe(_ chan<- *prometheus.Desc) {}

// Collect collects all ONT metrics.
func (d *ONT) Collect(c chan<- prometheus.Metric) {
	defer warnOnSlowCollect(d, time.Now())

	// Skip if GPON interface does not exist
	if !d.enabled {
		return
	}

	var ont struct {
		Status struct {
			Temperature        float64 `json:"Temperature"`
			DownstreamCurrRate float64 `json:"DownstreamCurrRate"`
			UpstreamCurrRate   float64 `json:"UpstreamCurrRate"`
		} `json:"status"`
	}

	if err := d.client.Request(context.TODO(), request.New("NeMo.Intf.veip0", "get", nil), &ont); err != nil {
		log.Printf("WARN: ONT collector: failed to get gpon interface: %s", err)
		return
	}

	c <- prometheus.MustNewConstMetric(d.temperatureMetric, prometheus.GaugeValue, ont.Status.Temperature)
	c <- prometheus.MustNewConstMetric(d.downstreamCurrRateMetric, prometheus.GaugeValue, 1000*ont.Status.DownstreamCurrRate)
	c <- prometheus.MustNewConstMetric(d.upstreamCurrRateMetric, prometheus.GaugeValue, 1000*ont.Status.UpstreamCurrRate)
}
