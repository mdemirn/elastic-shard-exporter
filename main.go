package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"elasticsearch-shard-exporter/collector"
)

var (
	Version   = "1.0.0"
	BuildTime = "unknown"
)

type Config struct {
	ListenAddr    string
	MetricsPath   string
	ESUrl         string
	ESUser        string
	ESPass        string
	SSLEnable     bool
	SSLSkipVerify bool
	ShowVersion   bool
}

func main() {
	config := parseFlags()

	if config.ShowVersion {
		fmt.Printf("elasticsearch-shard-exporter version %s (built: %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	shardCollector, err := collector.NewShardCollector(collector.CollectorConfig{
		ESURL:         config.ESUrl,
		ESUser:        config.ESUser,
		ESPass:        config.ESPass,
		SSLEnable:     config.SSLEnable,
		SSLSkipVerify: config.SSLSkipVerify,
	})
	if err != nil {
		log.Fatalf("Failed to create collector: %v", err)
	}

	prometheus.MustRegister(shardCollector)

	http.Handle(config.MetricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Elasticsearch Shard Exporter</title></head>
<body>
<h1>Elasticsearch Shard Exporter</h1>
<p><a href="` + config.MetricsPath + `">Metrics</a></p>
<p>Version: ` + Version + `</p>
</body>
</html>`))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting Elasticsearch Shard Exporter v%s", Version)
	log.Printf("Elasticsearch URL: %s", config.ESUrl)
	log.Printf("Listening on %s", config.ListenAddr)
	log.Printf("Metrics available at http://%s%s", config.ListenAddr, config.MetricsPath)

	if err := http.ListenAndServe(config.ListenAddr, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func parseFlags() Config {
	config := Config{}

	flag.StringVar(&config.ListenAddr, "listen-address", ":9061", "Address to listen on for HTTP requests")
	flag.StringVar(&config.MetricsPath, "metrics-path", "/metrics", "Path under which to expose metrics")
	flag.StringVar(&config.ESUrl, "es-url", "", "Elasticsearch URL (required)")
	flag.StringVar(&config.ESUser, "es-user", "", "Elasticsearch username for authentication")
	flag.StringVar(&config.ESPass, "es-pass", "", "Elasticsearch password for authentication")
	flag.BoolVar(&config.SSLEnable, "ssl-enable", false, "Enable SSL/TLS for Elasticsearch connection")
	flag.BoolVar(&config.SSLSkipVerify, "ssl-skip-verify", false, "Skip SSL certificate verification")
	flag.BoolVar(&config.ShowVersion, "version", false, "Show version information")

	flag.Parse()

	if envURL := os.Getenv("ES_URL"); envURL != "" {
		config.ESUrl = envURL
	}
	if envUser := os.Getenv("ES_USER"); envUser != "" {
		config.ESUser = envUser
	}
	if envPass := os.Getenv("ES_PASS"); envPass != "" {
		config.ESPass = envPass
	}

	return config
}

func validateConfig(config Config) error {
	if config.ESUrl == "" {
		return fmt.Errorf("--es-url is required (or set ES_URL environment variable)")
	}
	return nil
}
