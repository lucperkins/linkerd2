package prometheus

import (
	"net/http"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// WrapTransport provides a function for wrapping an http.RoundTripper
type WrapTransport func(http.RoundTripper) http.RoundTripper

// RequestDurationBucketsSeconds represents latency buckets to record (seconds)
var RequestDurationBucketsSeconds = append(append(append(append(
	prometheus.LinearBuckets(0.01, 0.01, 5),
	prometheus.LinearBuckets(0.1, 0.1, 5)...),
	prometheus.LinearBuckets(1, 1, 5)...),
	prometheus.LinearBuckets(10, 10, 5)...),
)

// ResponseSizeBuckets represents response size buckets (bytes)
var ResponseSizeBuckets = append(append(append(append(
	prometheus.LinearBuckets(100, 100, 5),
	prometheus.LinearBuckets(1000, 1000, 5)...),
	prometheus.LinearBuckets(10000, 10000, 5)...),
	prometheus.LinearBuckets(1000000, 1000000, 5)...),
)

var (
	// server metrics
	counter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	duration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "A histogram of latencies for requests in seconds.",
			Buckets: RequestDurationBucketsSeconds,
		},
		[]string{"code", "method"},
	)

	responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: ResponseSizeBuckets,
		},
		[]string{"code", "method"},
	)

	// client metrics
	clientCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "client_api_requests_total",
			Help: "A counter for requests from the wrapped client.",
		},
		[]string{"client", "code", "method"},
	)

	clientDur = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "client_request_duration_seconds",
			Help:    "A histogram of request latencies.",
			Buckets: RequestDurationBucketsSeconds,
		},
		[]string{"client", "code", "method"},
	)

	clientInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "client_in_flight_requests",
			Help: "A gauge of in-flight requests for the wrapped client.",
		},
		[]string{"client"},
	)
)

func init() {
	prometheus.MustRegister(
		counter, duration, responseSize, // server metrics
		clientCounter, clientDur, clientInFlight, // client metrics
	)
}

// NewGrpcServer returns a grpc server pre-configured with prometheus interceptors
func NewGrpcServer() *grpc.Server {
	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)

	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(server)
	return server
}

// WithTelemetry instruments the HTTP server with prometheus
func WithTelemetry(handler http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerDuration(duration,
		promhttp.InstrumentHandlerResponseSize(responseSize,
			promhttp.InstrumentHandlerCounter(counter, handler)))
}

// ClientWithTelemetry instruments the HTTP client with prometheus
func ClientWithTelemetry(name string, wt WrapTransport) WrapTransport {
	dur := clientDur.MustCurryWith(prometheus.Labels{"client": name})
	count := clientCounter.MustCurryWith(prometheus.Labels{"client": name})
	flight := clientInFlight.With(prometheus.Labels{"client": name})

	return func(rt http.RoundTripper) http.RoundTripper {
		if wt != nil {
			rt = wt(rt)
		}

		return promhttp.InstrumentRoundTripperInFlight(flight,
			promhttp.InstrumentRoundTripperCounter(count,
				promhttp.InstrumentRoundTripperDuration(dur, rt),
			),
		)
	}
}
