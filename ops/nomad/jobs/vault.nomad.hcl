// Vault runs outside Nomad in production (it's the secrets backend Nomad relies on).
// This spec is for a dev/staging single-node Vault — NOT for production use.
// Production: dedicated HA Vault cluster with Raft, auto-unseal via cloud KMS.
job "vault-dev" {
  datacenters = ["in-mum-1"]
  type        = "service"
  priority    = 95

  group "vault" {
    count = 1

    network {
      port "api"     { static = 8200 }
      port "cluster" { static = 8201 }
    }

    service {
      name     = "vault"
      port     = "api"
      provider = "consul"
      check {
        type     = "http"
        path     = "/v1/sys/health"
        interval = "15s"
        timeout  = "5s"
      }
    }

    task "vault" {
      driver = "docker"
      config {
        image   = "hashicorp/vault:1.17"
        ports   = ["api", "cluster"]
        cap_add = ["IPC_LOCK"]
        args    = ["server", "-config=/local/vault.hcl"]
      }

      template {
        destination = "local/vault.hcl"
        data        = <<EOT
storage "raft" {
  path    = "/data/vault"
  node_id = "vault-0"
}
listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true   # TLS terminated at edge proxy in dev; enable in prod
}
api_addr     = "http://vault.service.consul:8200"
cluster_addr = "http://vault.service.consul:8201"
ui           = true
EOT
      }

      resources {
        cpu    = 512
        memory = 512
      }

      volume_mount {
        volume      = "vault-data"
        destination = "/data/vault"
      }
    }

    volume "vault-data" {
      type   = "host"
      source = "vault-data"
    }
  }
}
