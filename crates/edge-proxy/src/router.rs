//! Host → backend routing table with consistent-hash replica selection.
//!
//! Phase 0: an in-memory table. The control plane will later populate this
//! (via NATS `platform.deploy.live` / a routing-table feed). SNI-based lookup
//! and health-aware replica sets build on top of this same structure.

use std::collections::hash_map::DefaultHasher;
use std::collections::HashMap;
use std::hash::{Hash, Hasher};
use std::sync::RwLock;

/// A routable backend: one or more healthy replica authorities (`host:port`).
#[derive(Clone, Debug, Default)]
pub struct Backend {
    pub replicas: Vec<String>,
}

impl Backend {
    /// Build a backend from a list of `host:port` replica authorities.
    pub fn new(replicas: impl IntoIterator<Item = String>) -> Self {
        Self {
            replicas: replicas.into_iter().collect(),
        }
    }

    /// Select a replica by consistent hash of `key` (e.g. the client IP), so a
    /// given client sticks to the same replica while it stays healthy. Returns
    /// `None` when there are no replicas.
    pub fn select(&self, key: &str) -> Option<&str> {
        if self.replicas.is_empty() {
            return None;
        }
        let mut h = DefaultHasher::new();
        key.hash(&mut h);
        let idx = (h.finish() % self.replicas.len() as u64) as usize;
        Some(&self.replicas[idx])
    }
}

/// Host-keyed routing table with interior mutability, so the control plane can
/// update routes at runtime (via the admin API) while the proxy serves traffic.
/// Lookups are port-insensitive.
#[derive(Default)]
pub struct Router {
    routes: RwLock<HashMap<String, Backend>>,
}

impl Router {
    pub fn new() -> Self {
        Self::default()
    }

    /// Insert or replace the backend for `host`.
    pub fn upsert(&self, host: impl Into<String>, backend: Backend) {
        self.routes.write().unwrap().insert(host.into(), backend);
    }

    /// Remove the route for `host`; returns whether it existed.
    pub fn remove(&self, host: &str) -> bool {
        self.routes.write().unwrap().remove(host).is_some()
    }

    /// Look up the backend for `host` (cloned, so no lock is held across an
    /// await), ignoring any `:port` suffix.
    pub fn lookup(&self, host: &str) -> Option<Backend> {
        let host = host.split(':').next().unwrap_or(host);
        self.routes.read().unwrap().get(host).cloned()
    }

    /// The set of routed hosts, for introspection.
    pub fn hosts(&self) -> Vec<String> {
        let mut hs: Vec<String> = self.routes.read().unwrap().keys().cloned().collect();
        hs.sort();
        hs
    }
}

/// Parse a `PROXY__STATIC_ROUTES` spec into `(host, backend)` pairs.
///
/// Format: comma-separated entries `host=replica1|replica2`, where replicas are
/// pipe-separated `host:port` authorities. Malformed or empty entries are
/// skipped. Used to pre-seed the table before the control-plane feed exists.
pub fn parse_routes(spec: &str) -> Vec<(String, Backend)> {
    spec.split(',')
        .filter_map(|entry| {
            let (host, reps) = entry.split_once('=')?;
            let host = host.trim();
            if host.is_empty() {
                return None;
            }
            let replicas: Vec<String> = reps
                .split('|')
                .map(str::trim)
                .filter(|s| !s.is_empty())
                .map(str::to_owned)
                .collect();
            if replicas.is_empty() {
                return None;
            }
            Some((host.to_owned(), Backend::new(replicas)))
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn lookup_strips_port() {
        let r = Router::new();
        r.upsert("app.example.com", Backend::new(["10.0.0.1:8080".to_string()]));
        assert!(r.lookup("app.example.com").is_some());
        assert!(r.lookup("app.example.com:443").is_some());
        assert!(r.lookup("other.example.com").is_none());
    }

    #[test]
    fn upsert_remove_roundtrip() {
        let r = Router::new();
        r.upsert("a.local", Backend::new(["10.0.0.1:80".to_string()]));
        assert_eq!(r.hosts(), vec!["a.local".to_string()]);
        assert!(r.remove("a.local"));
        assert!(!r.remove("a.local"));
        assert!(r.lookup("a.local").is_none());
    }

    #[test]
    fn select_is_stable_and_in_range() {
        let b = Backend::new(["a:1".to_string(), "b:2".to_string(), "c:3".to_string()]);
        let first = b.select("1.2.3.4").unwrap().to_string();
        // deterministic for the same key
        assert_eq!(b.select("1.2.3.4").unwrap(), first);
        // always one of the replicas
        assert!(b.replicas.iter().any(|r| r == &first));
    }

    #[test]
    fn select_empty_backend_is_none() {
        let b = Backend::default();
        assert!(b.select("anything").is_none());
    }

    #[test]
    fn parse_routes_handles_multi_replica_and_skips_garbage() {
        let routes = parse_routes("app.local=127.0.0.1:9000|127.0.0.1:9001,bad-entry,=noreplicas,api.local=10.0.0.5:80");
        assert_eq!(routes.len(), 2);
        let r = Router::new();
        for (host, backend) in routes {
            r.upsert(host, backend);
        }
        assert_eq!(r.lookup("app.local").unwrap().replicas.len(), 2);
        assert_eq!(r.lookup("api.local").unwrap().replicas.len(), 1);
    }
}
