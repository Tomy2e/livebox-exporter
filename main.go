package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-exporter/internal/collector"
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
	client *livebox.Client,
	experimental string,
	pollingFrequency *uint,
) (pollers []poller.Poller) {
	if experimental == "" {
		return nil
	}

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

func getHTTPClient() (*http.Client, error) {
	liveboxCACertPath := os.Getenv("LIVEBOX_CACERT")

	if liveboxCACertPath == "" {
		return http.DefaultClient, nil
	}

	// Get the SystemCertPool, continue with an empty pool on error.
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	certs, err := ioutil.ReadFile(liveboxCACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read livebox CA cert: %w", err)
	}

	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		return nil, errors.New("no livebox CA cert was successfully added")
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: rootCAs,
			},
		},
	}, nil
}

func isFatalError(err error) bool {
	if errors.Is(err, livebox.ErrInvalidPassword) {
		return true
	}

	var certError *tls.CertificateVerificationError
	if errors.As(err, &certError) {
		return true
	}

	return false
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

	liveboxAddress := os.Getenv("LIVEBOX_ADDRESS")
	if liveboxAddress == "" {
		liveboxAddress = livebox.DefaultAddress
	}

	httpClient, err := getHTTPClient()
	if err != nil {
		log.Fatal(err)
	}

	client, err := livebox.NewClient(
		adminPassword,
		livebox.WithAddress(liveboxAddress),
		livebox.WithHTTPClient(httpClient),
	)
	if err != nil {
		log.Fatalf("Failed to create Livebox client: %v", err)
	}

	var (
		ctx      = context.Background()
		registry = prometheus.NewRegistry()
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

	registry.MustRegister(collector.NewDeviceInfo(client))

	go func() {
		for {
			if err := pollers.Poll(ctx); err != nil {
				if isFatalError(err) {
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
