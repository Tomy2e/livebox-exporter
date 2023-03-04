package poller

import (
	"context"
	"fmt"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/prometheus/client_golang/prometheus"
)

var _ Poller = &DevicesTotal{}

// DevicesTotal allows to poll the total number of active devices.
type DevicesTotal struct {
	client       livebox.Client
	devicesTotal *prometheus.GaugeVec
}

// NewDevicesTotal returns a new DevicesTotal poller.
func NewDevicesTotal(client livebox.Client) *DevicesTotal {
	return &DevicesTotal{
		client: client,
		devicesTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "livebox_devices_total",
			Help: "The total number of active devices",
		}, []string{
			// Type of the device (dongle, ethernet, printer or wifi).
			"type",
		}),
	}
}

// Collectors returns all metrics.
func (dt *DevicesTotal) Collectors() []prometheus.Collector {
	return []prometheus.Collector{dt.devicesTotal}
}

// Poll polls the current number of active devices.
func (dt *DevicesTotal) Poll(ctx context.Context) error {
	var devices struct {
		Status map[string][]struct{} `json:"status"`
	}

	if err := dt.client.Request(ctx, request.New("Devices", "get", map[string]interface{}{
		"expression": map[string]string{
			"ethernet": "not interface and not self and eth and .Active==true",
			"wifi":     "not interface and not self and wifi and .Active==true",
			"printer":  "printer and .Active==true",
			"dongle":   "usb && wwan and .Active==true",
		},
	}), &devices); err != nil {
		return fmt.Errorf("failed to get active devices: %w", err)
	}

	for t, d := range devices.Status {
		dt.devicesTotal.
			With(prometheus.Labels{"type": t}).
			Set(float64(len(d)))
	}

	return nil
}
