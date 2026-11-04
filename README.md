# elastic-shard-exporter

Prometheus exporter for Elasticsearch shard state. It tells you whether shards are
relocating right now and the highest replica count across your indices, so you can
alert on long relocations and on indices that run without replicas.

A single static Go binary; ships with a systemd unit, an installer and a Dockerfile.

## Metrics

| Metric | Labels | Value |
|---|---|---|
| `es_shard_relocation` | `status` = `active` or `inactive` | `1`; `active` while `_cluster/health` reports relocating shards |
| `es_shard_replica` | `count` = highest `number_of_replicas` among all indices | `1` |
| `es_shard_exporter_scrape_error` | | `1` if the last scrape failed, else `0` |
| `es_shard_exporter_scrape_duration_seconds` | | Duration of the last scrape |

Example output while two shards are moving:

```
# HELP es_shard_relocation Elasticsearch shard relocation status
es_shard_relocation{status="active"} 1
# HELP es_shard_replica Elasticsearch shard replica count
es_shard_replica{count="2"} 1
es_shard_exporter_scrape_error 0
es_shard_exporter_scrape_duration_seconds 0.001
```

The exporter calls `GET /_cluster/health` and `GET /_settings` on every scrape.

## Example alerts

```yaml
groups:
  - name: elasticsearch-shards
    rules:
      - alert: ElasticsearchShardsRelocatingTooLong
        expr: es_shard_relocation{status="active"} == 1
        for: 30m
        annotations:
          summary: "Shards on {{ $labels.instance }} have been relocating for 30 minutes"
      - alert: ElasticsearchNoReplicas
        expr: es_shard_replica{count="0"} == 1
        for: 10m
        annotations:
          summary: "No index on {{ $labels.instance }} has replicas"
      - alert: ElasticsearchShardExporterFailing
        expr: es_shard_exporter_scrape_error == 1
        for: 5m
```

## Configuration

| Flag | Environment | Default | Meaning |
|---|---|---|---|
| `--es-url` | `ES_URL` | (required) | Elasticsearch URL, for example `https://es.example.internal:9200` |
| `--es-user` | `ES_USER` | | Basic auth user |
| `--es-pass` | `ES_PASS` | | Basic auth password |
| `--ssl-enable` | | `false` | Use a custom TLS client configuration (needed for `--ssl-skip-verify`) |
| `--ssl-skip-verify` | | `false` | With `--ssl-enable`: do not verify the server certificate |
| `--listen-address` | | `:9061` | Address of the HTTP server |
| `--metrics-path` | | `/metrics` | Path of the metrics endpoint |
| `--version` | | | Print the version and exit |

Environment variables win over the matching flags. Besides `/metrics`, the exporter
serves `/health` (returns `OK`) and a small landing page on `/`.

## Run

From source (Go 1.21 or later):

```bash
git clone https://github.com/mdemirn/elastic-shard-exporter.git
cd elastic-shard-exporter
make build
ES_URL=http://localhost:9200 ./elasticsearch-shard-exporter
curl -s localhost:9061/metrics | grep es_shard
```

With Docker:

```bash
docker build -t elasticsearch-shard-exporter .
docker run -d -p 9061:9061 -e ES_URL=http://es.example.internal:9200 \
  -e ES_USER=monitor -e ES_PASS=change-me elasticsearch-shard-exporter
```

As a systemd service (builds on the host, so Go must be installed there):

```bash
sudo ./install.sh install
sudoedit /etc/default/elasticsearch-shard-exporter   # set ES_URL, ES_USER, ES_PASS
sudo systemctl restart elasticsearch-shard-exporter
```

The installer creates a `prometheus` system user, installs the binary to
`/usr/local/bin`, the unit to `/etc/systemd/system` and the settings file to
`/etc/default/elasticsearch-shard-exporter` (mode 0600), then enables and starts the
service. `sudo ./install.sh uninstall` removes the service and the binary and keeps the
settings file. The shipped unit starts the exporter with `--ssl-enable
--ssl-skip-verify`; remove `--ssl-skip-verify` from
`/etc/systemd/system/elasticsearch-shard-exporter.service` if your cluster's
certificate is trusted on the host.

## Prometheus scrape configuration

```yaml
scrape_configs:
  - job_name: elasticsearch-shards
    static_configs:
      - targets: ["es-exporter.example.internal:9061"]
```

## Make targets

`build`, `build-linux`, `build-all` (Linux, macOS amd64/arm64, Windows), `test`, `run`,
`docker`, `install`, `uninstall`, `deps`, `fmt`, `lint`, `clean`.
