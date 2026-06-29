use anyhow::Result;
use serde::Deserialize;

// Several fields below are parsed now but consumed in later phases (TLS,
// metrics, metering, autostart). Allow them to sit unread until then rather
// than dropping config the operator already sets.
#[allow(dead_code)]
#[derive(Debug, Deserialize)]
pub struct Config {
    /// Address to bind the plaintext HTTP redirect listener.
    #[serde(default = "default_http_addr")]
    pub http_addr: String,

    /// Address to bind the TLS listener.
    #[serde(default = "default_https_addr")]
    pub https_addr: String,

    /// Prometheus metrics exposition address.
    #[serde(default = "default_metrics_addr")]
    pub metrics_addr: String,

    /// Admin/route-management API bind address (control plane → routing table).
    #[serde(default = "default_admin_addr")]
    pub admin_addr: String,

    /// NATS JetStream URL for emitting metering events.
    #[serde(default = "default_nats_url")]
    pub nats_url: String,

    /// Vault address for certificate retrieval.
    #[serde(default = "default_vault_addr")]
    pub vault_addr: String,

    /// Upstream healthcheck interval in seconds.
    #[serde(default = "default_healthcheck_interval_secs")]
    pub healthcheck_interval_secs: u64,

    /// Per-tenant default concurrency limit (requests in-flight).
    #[serde(default = "default_concurrency_per_tenant")]
    pub concurrency_per_tenant: usize,

    /// Max queued autostart wakeup requests before load-shedding.
    #[serde(default = "default_wakeup_queue_depth")]
    pub wakeup_queue_depth: usize,
}

impl Config {
    pub fn load() -> Result<Self> {
        let cfg = config::Config::builder()
            .add_source(config::Environment::with_prefix("PROXY").separator("__"))
            .build()?
            .try_deserialize()?;
        Ok(cfg)
    }
}

fn default_http_addr()                  -> String { "0.0.0.0:80".into() }
fn default_https_addr()                 -> String { "0.0.0.0:443".into() }
fn default_metrics_addr()               -> String { "0.0.0.0:9090".into() }
fn default_admin_addr()                 -> String { "127.0.0.1:9901".into() }
fn default_nats_url()                   -> String { "nats://127.0.0.1:4222".into() }
fn default_vault_addr()                 -> String { "http://127.0.0.1:8200".into() }
fn default_healthcheck_interval_secs()  -> u64    { 5 }
fn default_concurrency_per_tenant()     -> usize  { 512 }
fn default_wakeup_queue_depth()         -> usize  { 128 }
