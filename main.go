package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-exporter/internal/poller"
	exporterLivebox "github.com/Tomy2e/livebox-exporter/pkg/livebox"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/exp/slices"
)

const defaultPollingFrequency = 30

const (
	ExperimentalMetricsInterfaceHomeLan = "livebox_interface_homelan"
	ExperimentalMetricsInterfaceNetDev  = "livebox_interface_netdev"
	ExperimentalMetricsWAN              = "livebox_wan"
)

var experimentalMetrics = []string{
	ExperimentalMetricsInterfaceHomeLan,
	ExperimentalMetricsInterfaceNetDev,
	ExperimentalMetricsWAN,
}

func parseExperimentalFlag(
	ctx context.Context,
	client livebox.Client,
	experimental string,
	pollingFrequency *uint,
) (pollers []poller.Poller) {
	var (
		interfaces []*exporterLivebox.Interface
		err        error

		enabled = make(map[string]bool)
	)

	for _, exp := range strings.Split(experimental, ",") {
		exp = strings.TrimSpace(exp)

		if !slices.Contains(experimentalMetrics, exp) {
			log.Printf("WARN: Unknown experimental metrics: %s", exp)
			continue
		}

		if enabled[exp] {
			continue
		}

		// Discover interfaces for experimental pollers that require interfaces.
		switch exp {
		case ExperimentalMetricsInterfaceHomeLan, ExperimentalMetricsInterfaceNetDev:
			if interfaces == nil {
				interfaces, err = exporterLivebox.DiscoverInterfaces(ctx, client)
				if err != nil {
					log.Fatalf("Failed to discover Livebox interfaces: %s\n", err)
				}
			}
		}

		switch exp {
		case ExperimentalMetricsInterfaceHomeLan:
			pollers = append(pollers, poller.NewInterfaceHomeLanMbits(client, interfaces))
		case ExperimentalMetricsInterfaceNetDev:
			pollers = append(pollers, poller.NewInterfaceNetDevMbits(client, interfaces))

			if *pollingFrequency > 5 {
				log.Printf(
					"WARN: The %s experimental metrics require a lower polling frequency, "+
						"setting polling frequency to 5 seconds\n",
					ExperimentalMetricsInterfaceNetDev,
				)
				*pollingFrequency = 5
			}
		case ExperimentalMetricsWAN:
			pollers = append(pollers, poller.NewWANMbits(client))
		}

		log.Printf("INFO: enabled experimental metrics: %s\n", exp)
		enabled[exp] = true
	}

	return
}

func main() {
	pollingFrequency := flag.Uint("polling-frequency", defaultPollingFrequency, "Polling frequency")
	listen := flag.String("listen", ":8080", "Listening address")
	experimental := flag.String("experimental", "", fmt.Sprintf(
		"Comma separated list of experimental metrics to enable (available metrics: %s)",
		strings.Join(experimentalMetrics, ","),
	))
	flag.Parse()

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Fatal("ADMIN_PASSWORD environment variable must be set")
	}

	var (
		ctx      = context.Background()
		registry = prometheus.NewRegistry()
		client   = livebox.NewClient(adminPassword)
		pollers  = poller.Pollers{
			poller.NewDevicesTotal(client),
			poller.NewInterfaceMbits(client),
		}
	)

	// Add experimental pollers.
	pollers = append(pollers, parseExperimentalFlag(ctx, client, *experimental, pollingFrequency)...)

	registry.MustRegister(
		append(
			pollers.Collectors(),
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)...,
	)

	go func() {
		for {
			if err := pollers.Poll(ctx); err != nil {
				if errors.Is(err, livebox.ErrInvalidPassword) {
					log.Fatal(err)
				}

				log.Printf("WARN: polling failed: %s\n", err)
			}

			time.Sleep(time.Duration(*pollingFrequency) * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.InstrumentMetricHandler(
		registry, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	))
	log.Printf("Listening on %s\n", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
