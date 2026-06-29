// Consul server configuration for Phase 0 (single DC, 3-node quorum)
datacenter = "in-mum-1"
data_dir   = "/opt/consul/data"
log_level  = "INFO"
server     = true
bootstrap_expect = 3

bind_addr   = "{{ GetInterfaceIP \"eth0\" }}"
client_addr = "0.0.0.0"

ui_config {
  enabled = true
}

acl {
  enabled                  = true
  default_policy           = "deny"
  enable_token_persistence = true
}

connect {
  enabled = true  // Consul Connect (service mesh) — used for mTLS between platform services
}

telemetry {
  prometheus_retention_time = "60s"
  disable_hostname          = true
}

encrypt = "REPLACE_WITH_CONSUL_GOSSIP_KEY"  // generate: consul keygen

// TLS for inter-agent comms (Phase 1: auto-generated via Vault PKI)
// tls {
//   defaults {
//     ca_file   = "/opt/consul/tls/ca.pem"
//     cert_file = "/opt/consul/tls/server.pem"
//     key_file  = "/opt/consul/tls/server-key.pem"
//   }
// }
