job "control-plane-api" {
  datacenters = ["in-mum-1"]
  type        = "service"
  priority    = 80

  update {
    max_parallel      = 1
    min_healthy_time  = "15s"
    healthy_deadline  = "3m"
    auto_revert       = true
    canary            = 1
  }

  group "api" {
    count = 2

    network {
      port "http" { to = 8080 }
    }

    service {
      name     = "antariksh-api"
      port     = "http"
      provider = "consul"

      check {
        type     = "http"
        path     = "/healthz"
        interval = "10s"
        timeout  = "3s"
      }
    }

    task "api" {
      driver = "docker"  # Phase 0: docker driver; Phase 1: firecracker driver

      config {
        image   = "registry.internal.antariksh.in/antariksh/api:${DEPLOY_TAG}"
        ports   = ["http"]
        volumes = []
      }

      env {
        PORT         = "${NOMAD_PORT_http}"
        NATS_URL     = "nats://nats.service.consul:4222"
        DATABASE_URL = "postgresql://api:${DB_PASS}@pgbouncer.service.consul:5432/antariksh"
        TEMPORAL_HOST = "temporal.service.consul:7233"
        VAULT_ADDR   = "http://vault.service.consul:8200"
      }

      template {
        data        = "{{ with secret \"kv/data/antariksh/api\" }}{{ .Data.data.DB_PASS }}{{ end }}"
        destination = "secrets/db.env"
        env         = true
      }

      vault {
        policies = ["antariksh-api"]
        change_mode = "restart"
      }

      resources {
        cpu    = 512   # MHz
        memory = 512   # MB
      }
    }
  }
}
