// Observability stack: VictoriaMetrics + Loki + Tempo + Grafana
// In Phase 0 these are co-scheduled in a single group for simplicity.
// Phase 3: split into dedicated jobs, add ClickHouse for audit/billing analytics.
job "observability" {
  datacenters = ["in-mum-1"]
  type        = "service"
  priority    = 70

  group "victoria" {
    count = 1
    network {
      port "vmhttp"  { static = 8428 }
    }
    service {
      name     = "victoriametrics"
      port     = "vmhttp"
      provider = "consul"
      check { type = "http"; path = "/health"; interval = "15s"; timeout = "5s" }
    }
    task "vm" {
      driver = "docker"
      config {
        image = "victoriametrics/victoria-metrics:v1.105.0"
        ports = ["vmhttp"]
        args  = [
          "-storageDataPath=/data/vm",
          "-retentionPeriod=12",       # months
          "-httpListenAddr=:8428",
        ]
      }
      resources { cpu = 1000; memory = 1024 }
      volume_mount { volume = "vm-data"; destination = "/data/vm" }
    }
    volume "vm-data" { type = "host"; source = "vm-data" }
  }

  group "loki" {
    count = 1
    network {
      port "lokihttp" { static = 3100 }
    }
    service {
      name     = "loki"
      port     = "lokihttp"
      provider = "consul"
      check { type = "http"; path = "/ready"; interval = "15s"; timeout = "5s" }
    }
    task "loki" {
      driver = "docker"
      config {
        image = "grafana/loki:3.3.0"
        ports = ["lokihttp"]
        args  = ["-config.file=/local/loki.yaml"]
      }
      template {
        destination = "local/loki.yaml"
        data        = <<EOT
auth_enabled: false
server:
  http_listen_port: 3100
common:
  storage:
    filesystem:
      chunks_directory: /data/loki/chunks
      rules_directory:  /data/loki/rules
schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h
EOT
      }
      resources { cpu = 512; memory = 512 }
      volume_mount { volume = "loki-data"; destination = "/data/loki" }
    }
    volume "loki-data" { type = "host"; source = "loki-data" }
  }

  group "grafana" {
    count = 1
    network {
      port "grafanahttp" { static = 3000 }
    }
    service {
      name     = "grafana"
      port     = "grafanahttp"
      provider = "consul"
      check { type = "http"; path = "/api/health"; interval = "15s"; timeout = "5s" }
    }
    task "grafana" {
      driver = "docker"
      config {
        image = "grafana/grafana:11.4.0"
        ports = ["grafanahttp"]
      }
      env {
        GF_SECURITY_ADMIN_PASSWORD = "changeme-in-vault"
        GF_DATABASE_TYPE           = "postgres"
        GF_DATABASE_HOST           = "pgbouncer.service.consul:5432"
      }
      resources { cpu = 256; memory = 256 }
    }
  }
}
