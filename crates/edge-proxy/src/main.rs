//! Antariksh Edge Proxy
//!
//! Hot path responsibilities (in order):
//!  1. Anycast TLS termination (rustls; certs distributed from Vault)
//!  2. SNI → tenant routing table lookup
//!  3. Autostart-on-request: if upstream microVM is suspended, wake it
//!     (snapshot-resume via fc-driver) and hold the connection
//!  4. Upstream load-balance across healthy VM replicas (consistent hash)
//!  5. Load-shedding + per-tenant concurrency limits (tower middleware)
//!  6. Retries and replay for idempotent requests
//!  7. Emit per-request telemetry to NATS "platform.metering.req"
//!
//! This binary is the single most load-bearing component in the platform.
//! Every platform incident will originate here or reveal itself here first.

use std::sync::Arc;

use anyhow::Result;
use tracing::info;

mod config;
mod proxy;
mod router;
mod tls;
mod wakeup;
mod metrics;
mod shed;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .json()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "info".into()),
        )
        .init();

    let cfg = config::Config::load()?;
    info!(
        http_addr  = %cfg.http_addr,
        https_addr = %cfg.https_addr,
        "edge-proxy starting"
    );

    // TODO: load TLS certificates from Vault → HTTPS listener (tls.rs)
    // TODO: start autostart wakeup pool — fc-driver HTTP client (wakeup.rs)
    // TODO: serve metrics on :9090/metrics for VictoriaMetrics scrape (metrics.rs)
    // TODO: populate the routing table from the control plane (NATS deploy.live)

    // Phase 0: plaintext HTTP listener + reverse proxy. PROXY__STATIC_ROUTES
    // pre-seeds routes; the admin API (admin_addr) lets the control plane add or
    // remove routes at runtime (host → microVM address) as deploys come and go.
    let table = Arc::new(router::Router::new());
    if let Ok(spec) = std::env::var("PROXY__STATIC_ROUTES") {
        for (host, backend) in router::parse_routes(&spec) {
            info!(%host, replicas = backend.replicas.len(), "seeding static route");
            table.upsert(host, backend);
        }
    }

    let admin_table = table.clone();
    let admin_addr = cfg.admin_addr.clone();
    tokio::spawn(async move {
        if let Err(e) = proxy::serve_admin(&admin_addr, admin_table).await {
            tracing::error!(error = %e, "admin API stopped");
        }
    });

    proxy::serve(&cfg.http_addr, table).await?;

    Ok(())
}
