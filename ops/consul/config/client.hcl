// Consul client — runs on every compute node alongside the Nomad client
datacenter = "in-mum-1"
data_dir   = "/opt/consul/data"
log_level  = "INFO"
server     = false

bind_addr   = "{{ GetInterfaceIP \"eth0\" }}"
client_addr = "127.0.0.1"

retry_join = ["consul.in-mum-1.internal"]  // DNS name of your Consul servers

acl {
  enabled                  = true
  default_policy           = "deny"
  enable_token_persistence = true
  // agent token injected at bootstrap via Vault
}

connect { enabled = true }

telemetry {
  prometheus_retention_time = "60s"
  disable_hostname          = true
}

encrypt = "REPLACE_WITH_CONSUL_GOSSIP_KEY"
