package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-exporter/internal/poller"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultPollingFrequency = 30

func main() {
	pollingFrequency := flag.Uint("polling-frequency", defaultPollingFrequency, "Polling frequency")
	listen := flag.String("listen", ":8080", "Listening address")
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
