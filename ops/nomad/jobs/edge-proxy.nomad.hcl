job "edge-proxy" {
  datacenters = ["in-mum-1"]
  type        = "system"   # runs on every node — this IS the edge
  priority    = 90

  group "proxy" {
    network {
      # Host networking so we can bind 0.0.0.0:80/443 directly
      mode = "host"
      port "http"    { static = 80  }
      port "https"   { static = 443 }
      port "metrics" { static = 9090 }
    }

    service {
      name     = "edge-proxy-metrics"
      port     = "metrics"
      provider = "consul"

      check {
        type     = "http"
        path     = "/metrics"
        interval = "15s"
        timeout  = "3s"
      }
    }

    task "edge-proxy" {
      driver = "raw_exec"  # Phase 0: raw binary; Phase 1: firecracker driver

      config {
        command = "/opt/antariksh/bin/edge-proxy"
      }

      env {
        PROXY__HTTP_ADDR    = "0.0.0.0:80"
        PROXY__HTTPS_ADDR   = "0.0.0.0:443"
        PROXY__METRICS_ADDR = "0.0.0.0:9090"
        PROXY__NATS_URL     = "nats://127.0.0.1:4222"
        PROXY__VAULT_ADDR   = "http://127.0.0.1:8200"
        RUST_LOG            = "info"
      }

      vault {
        policies    = ["edge-proxy-tls"]
        change_mode = "signal"
        change_signal = "SIGUSR1"  # proxy hot-reloads certs on SIGUSR1
      }

      resources {
        cpu    = 2000  # 2 cores — this is the hot path
        memory = 1024
      }
    }
  }
}
