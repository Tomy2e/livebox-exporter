package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Tomy2e/livebox-api-client"
	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const defaultPollingFrequency = 30

var (
	rxMbits = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "livebox_interface_rx_mbits",
		Help: "Received Mbits per second.",
	}, []string{
		// Name of the interface.
		"interface",
	})
	txMbits = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "livebox_interface_tx_mbits",
		Help: "Transmitted Mbits per second.",
	}, []string{
		// Name of the interface.
		"interface",
	})
)

func bitsPer30SecsToMbitsPerSec(v int) float64 {
	return float64(v) / 30000000
}

func main() {
	pollingFrequency := flag.Uint("polling-frequency", defaultPollingFrequency, "Polling frequency")
	listen := flag.String("listen", ":8080", "Listening address")
	flag.Parse()

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Fatal("ADMIN_PASSWORD environment variable must be set")
	}

	ctx := context.Background()
	client := livebox.NewClient(adminPassword)

	go func() {
		for {
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
			if err := client.Request(
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
				log.Fatalf("Request to Livebox API failed: %v", err)
			}

			for iface, traffic := range counters.Status {
				rxCounter := 0
				txCounter := 0

				if len(traffic.Traffic) > 0 {
					rxCounter = traffic.Traffic[0].RxCounter
					txCounter = traffic.Traffic[0].TxCounter
				}

				rxMbits.
					With(prometheus.Labels{"interface": iface}).
					Set(bitsPer30SecsToMbitsPerSec(rxCounter))
				txMbits.
					With(prometheus.Labels{"interface": iface}).
					Set(bitsPer30SecsToMbitsPerSec(txCounter))
			}

			time.Sleep(time.Duration(*pollingFrequency) * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Listening on %s\n", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
