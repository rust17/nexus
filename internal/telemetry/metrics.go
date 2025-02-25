package telemetry

import (
	"net/http"
	"time"

	otelmetric "go.opentelemetry.io/otel/metric"
)

func (t *Telemetry) RegisterMetrics() error {
	requestCounter, err := t.meter.Int64Counter(
		"nexus.requests.total",
		otelmetric.WithDescription("Total number of requests"),
		otelmetric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	latencyHistogram, err := t.meter.Int64Histogram(
		"nexus.request.latency",
		otelmetric.WithDescription("Request latency distribution"),
		otelmetric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Request counter
		requestCounter.Add(r.Context(), 1)

		// Delay histogram
		defer func() {
			latency := time.Since(start).Milliseconds()
			latencyHistogram.Record(r.Context(), latency)
		}()

		// ...process request...
	})

	return nil
}
