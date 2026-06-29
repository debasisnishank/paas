job "nats" {
  datacenters = ["in-mum-1"]
  type        = "service"
  priority    = 85

  group "nats" {
    count = 3  # JetStream raft quorum

    network {
      port "client"  { static = 4222 }
      port "cluster" { static = 6222 }
      port "monitor" { static = 8222 }
    }

    service {
      name     = "nats"
      port     = "client"
      provider = "consul"
      check {
        type     = "http"
        path     = "/healthz"
        port     = "monitor"
        interval = "10s"
        timeout  = "3s"
      }
    }

    task "nats" {
      driver = "docker"
      config {
        image   = "nats:2.10-alpine"
        ports   = ["client", "cluster", "monitor"]
        args    = ["-c", "/local/nats.conf"]
      }

      template {
        destination = "local/nats.conf"
        data        = <<EOT
server_name: {{ env "NOMAD_ALLOC_ID" }}
listen: 0.0.0.0:4222
http:  0.0.0.0:8222

jetstream {
  store_dir: /data/nats
  max_memory_store: 512MB
  max_file_store:   10GB
}

cluster {
  name: antariksh-nats
  listen: 0.0.0.0:6222
  routes: [
    nats://nats.service.consul:6222
  ]
}
EOT
      }

      resources {
        cpu    = 512
        memory = 512
      }

      volume_mount {
        volume      = "nats-data"
        destination = "/data/nats"
      }
    }

    volume "nats-data" {
      type   = "host"
      source = "nats-data"
    }
  }
}
