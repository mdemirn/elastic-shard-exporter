package collector

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "trendyol_nosql"
)

type CollectorConfig struct {
	ESURL         string
	ESUser        string
	ESPass        string
	SSLEnable     bool
	SSLSkipVerify bool
}

type ClusterHealthResponse struct {
	ClusterName                 string  `json:"cluster_name"`
	Status                      string  `json:"status"`
	TimedOut                    bool    `json:"timed_out"`
	NumberOfNodes               int     `json:"number_of_nodes"`
	NumberOfDataNodes           int     `json:"number_of_data_nodes"`
	ActivePrimaryShards         int     `json:"active_primary_shards"`
	ActiveShards                int     `json:"active_shards"`
	RelocatingShards            int     `json:"relocating_shards"`
	InitializingShards          int     `json:"initializing_shards"`
	UnassignedShards            int     `json:"unassigned_shards"`
	DelayedUnassignedShards     int     `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks        int     `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch       int     `json:"number_of_in_flight_fetch"`
	TaskMaxWaitingInQueueMillis int     `json:"task_max_waiting_in_queue_millis"`
	ActiveShardsPercentAsNumber float64 `json:"active_shards_percent_as_number"`
}

type IndexSettingsResponse map[string]IndexSettings

type IndexSettings struct {
	Settings struct {
		Index struct {
			NumberOfReplicas string `json:"number_of_replicas"`
			NumberOfShards   string `json:"number_of_shards"`
		} `json:"index"`
	} `json:"settings"`
}

type ShardCollector struct {
	config CollectorConfig
	client *http.Client
	mutex  sync.RWMutex

	shardRelocationMetric *prometheus.Desc
	shardReplicaMetric    *prometheus.Desc
	scrapeErrorMetric     *prometheus.Desc
	scrapeDurationMetric  *prometheus.Desc
}

func NewShardCollector(config CollectorConfig) (*ShardCollector, error) {
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if config.SSLEnable {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: config.SSLSkipVerify,
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &ShardCollector{
		config: config,
		client: client,

		shardRelocationMetric: prometheus.NewDesc(
			"trendyol_nosql_shard_relocation",
			"Elasticsearch shard relocation status",
			[]string{"status"},
			nil,
		),

		shardReplicaMetric: prometheus.NewDesc(
			"trendyol_nosql_shard_replica",
			"Elasticsearch shard replica count",
			[]string{"count"},
			nil,
		),

		scrapeErrorMetric: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "exporter", "scrape_error"),
			"Scrape error status",
			nil,
			nil,
		),

		scrapeDurationMetric: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "exporter", "scrape_duration_seconds"),
			"Duration of the scrape in seconds",
			nil,
			nil,
		),
	}, nil
}

func (c *ShardCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.shardRelocationMetric
	ch <- c.shardReplicaMetric
	ch <- c.scrapeErrorMetric
	ch <- c.scrapeDurationMetric
}

func (c *ShardCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	start := time.Now()
	scrapeError := 0.0

	health, err := c.fetchClusterHealth()
	if err != nil {
		log.Printf("Error fetching cluster health: %v", err)
		scrapeError = 1.0
	} else {
		relocationStatus := "inactive"
		if health.RelocatingShards > 0 {
			relocationStatus = "active"
		}
		ch <- prometheus.MustNewConstMetric(
			c.shardRelocationMetric,
			prometheus.GaugeValue,
			1,
			relocationStatus,
		)
	}

	replicas, err := c.fetchMaxReplicaCount()
	if err != nil {
		log.Printf("Error fetching replica count: %v", err)
		scrapeError = 1.0
	} else {
		ch <- prometheus.MustNewConstMetric(
			c.shardReplicaMetric,
			prometheus.GaugeValue,
			1,
			fmt.Sprintf("%d", replicas),
		)
	}

	ch <- prometheus.MustNewConstMetric(
		c.scrapeErrorMetric,
		prometheus.GaugeValue,
		scrapeError,
	)

	ch <- prometheus.MustNewConstMetric(
		c.scrapeDurationMetric,
		prometheus.GaugeValue,
		time.Since(start).Seconds(),
	)
}

func (c *ShardCollector) fetchClusterHealth() (*ClusterHealthResponse, error) {
	url := fmt.Sprintf("%s/_cluster/health", c.config.ESURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.config.ESUser != "" && c.config.ESPass != "" {
		req.SetBasicAuth(c.config.ESUser, c.config.ESPass)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var health ClusterHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &health, nil
}

func (c *ShardCollector) fetchMaxReplicaCount() (int, error) {
	url := fmt.Sprintf("%s/_settings", c.config.ESURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if c.config.ESUser != "" && c.config.ESPass != "" {
		req.SetBasicAuth(c.config.ESUser, c.config.ESPass)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch index settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var settings IndexSettingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	maxReplicas := 0
	for _, indexSettings := range settings {
		var replicas int
		fmt.Sscanf(indexSettings.Settings.Index.NumberOfReplicas, "%d", &replicas)
		if replicas > maxReplicas {
			maxReplicas = replicas
		}
	}

	return maxReplicas, nil
}
