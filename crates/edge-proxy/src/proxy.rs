//! Plaintext HTTP listener + reverse proxy (Phase 0).
//!
//! Accepts HTTP/1.1 connections, resolves the target backend by `Host` header
//! via the [`Router`], picks a replica by consistent hash of the client IP, and
//! forwards the request. TLS termination, autostart-on-request, load-shedding,
//! and per-request metering are layered on in later phases — see `main.rs`.

use std::net::SocketAddr;
use std::sync::Arc;

use anyhow::{Context, Result};
use http_body_util::{combinators::BoxBody, BodyExt, Full};
use hyper::body::{Bytes, Incoming};
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper::{Method, Request, Response, StatusCode, Uri};
use hyper_util::client::legacy::connect::HttpConnector;
use hyper_util::client::legacy::Client;
use hyper_util::rt::{TokioExecutor, TokioIo};
use serde::Deserialize;
use tokio::net::TcpListener;
use tracing::{error, info, warn};

use crate::router::{Backend, Router};

/// Response body type used throughout the proxy: either a small in-proxy
/// message or a forwarded upstream body, both boxed to a single type.
type ProxyBody = BoxBody<Bytes, hyper::Error>;
type ProxyClient = Client<HttpConnector, Incoming>;

/// Bind `http_addr` and serve until error.
pub async fn serve(http_addr: &str, router: Arc<Router>) -> Result<()> {
    let addr: SocketAddr = http_addr
        .parse()
        .with_context(|| format!("invalid http_addr: {http_addr}"))?;
    let listener = TcpListener::bind(addr)
        .await
        .with_context(|| format!("bind {addr}"))?;
    serve_listener(listener, router).await
}

/// Serve an already-bound listener. Split out so tests can drive an ephemeral
/// port.
pub async fn serve_listener(listener: TcpListener, router: Arc<Router>) -> Result<()> {
    let local = listener.local_addr().context("local_addr")?;
    info!(addr = %local, "edge-proxy HTTP listener bound");

    let client: ProxyClient = Client::builder(TokioExecutor::new()).build_http();

    loop {
        let (stream, peer) = listener.accept().await.context("accept")?;
        let io = TokioIo::new(stream);
        let router = router.clone();
        let client = client.clone();
        tokio::spawn(async move {
            let svc = service_fn(move |req| handle(req, router.clone(), client.clone(), peer));
            if let Err(e) = http1::Builder::new().serve_connection(io, svc).await {
                warn!(error = %e, "connection error");
            }
        });
    }
}

async fn handle(
    req: Request<Incoming>,
    router: Arc<Router>,
    client: ProxyClient,
    peer: SocketAddr,
) -> Result<Response<ProxyBody>, hyper::Error> {
    // The proxy's own liveness endpoint, independent of any backend.
    if req.uri().path() == "/healthz" {
        return Ok(text(StatusCode::OK, "ok"));
    }

    let host = req
        .headers()
        .get(hyper::header::HOST)
        .and_then(|h| h.to_str().ok())
        .map(str::to_string)
        .or_else(|| req.uri().host().map(str::to_string));

    let Some(host) = host else {
        return Ok(text(StatusCode::BAD_REQUEST, "missing Host header"));
    };

    let Some(replica) = router
        .lookup(&host)
        .and_then(|b| b.select(&peer.ip().to_string()).map(str::to_string))
    else {
        return Ok(text(StatusCode::NOT_FOUND, "no route for host"));
    };

    match forward(req, &replica, &client).await {
        Ok(resp) => Ok(resp),
        Err(e) => {
            error!(error = %e, replica, "upstream forward failed");
            Ok(text(StatusCode::BAD_GATEWAY, "upstream error"))
        }
    }
}

/// Rewrite the request URI to target `replica` and forward it upstream.
async fn forward(
    mut req: Request<Incoming>,
    replica: &str,
    client: &ProxyClient,
) -> Result<Response<ProxyBody>> {
    let path_and_query = req
        .uri()
        .path_and_query()
        .map(|pq| pq.as_str())
        .unwrap_or("/");
    let upstream: Uri = format!("http://{replica}{path_and_query}")
        .parse()
        .context("build upstream uri")?;
    *req.uri_mut() = upstream;

    let resp = client.request(req).await.context("upstream request")?;
    Ok(resp.map(|b| b.boxed()))
}

/// Admin API request body for `POST /routes`.
#[derive(Deserialize)]
struct RouteReq {
    host: String,
    replicas: Vec<String>,
}

/// Bind the admin API listener (route management) and serve until error.
pub async fn serve_admin(admin_addr: &str, router: Arc<Router>) -> Result<()> {
    let addr: SocketAddr = admin_addr
        .parse()
        .with_context(|| format!("invalid admin_addr: {admin_addr}"))?;
    let listener = TcpListener::bind(addr)
        .await
        .with_context(|| format!("bind admin {addr}"))?;
    serve_admin_listener(listener, router).await
}

async fn serve_admin_listener(listener: TcpListener, router: Arc<Router>) -> Result<()> {
    info!(addr = %listener.local_addr().context("local_addr")?, "edge-proxy admin API bound");
    loop {
        let (stream, _) = listener.accept().await.context("accept")?;
        let io = TokioIo::new(stream);
        let router = router.clone();
        tokio::spawn(async move {
            let svc = service_fn(move |req| admin_handle(req, router.clone()));
            if let Err(e) = http1::Builder::new().serve_connection(io, svc).await {
                warn!(error = %e, "admin connection error");
            }
        });
    }
}

/// The control-plane route-management API:
///   GET    /routes          → {"hosts":[...]}
///   POST   /routes          → {"host":"app.local","replicas":["10.0.0.2:80"]}
///   DELETE /routes/<host>   → remove a route
async fn admin_handle(
    req: Request<Incoming>,
    router: Arc<Router>,
) -> Result<Response<ProxyBody>, hyper::Error> {
    let method = req.method().clone();
    let path = req.uri().path().to_string();

    match (&method, path.as_str()) {
        (&Method::GET, "/healthz") => Ok(json(StatusCode::OK, r#"{"status":"ok"}"#.into())),
        (&Method::GET, "/routes") => {
            let arr = serde_json::to_string(&router.hosts()).unwrap_or_else(|_| "[]".into());
            Ok(json(StatusCode::OK, format!(r#"{{"hosts":{arr}}}"#)))
        }
        (&Method::POST, "/routes") => {
            let bytes = req.into_body().collect().await?.to_bytes();
            match serde_json::from_slice::<RouteReq>(&bytes) {
                Ok(r) if !r.host.is_empty() && !r.replicas.is_empty() => {
                    info!(host = %r.host, replicas = r.replicas.len(), "route upserted");
                    router.upsert(r.host, Backend::new(r.replicas));
                    Ok(json(StatusCode::OK, r#"{"status":"ok"}"#.into()))
                }
                Ok(_) => Ok(json(
                    StatusCode::BAD_REQUEST,
                    r#"{"error":"host and replicas are required"}"#.into(),
                )),
                Err(_) => Ok(json(StatusCode::BAD_REQUEST, r#"{"error":"invalid json"}"#.into())),
            }
        }
        (&Method::DELETE, p) if p.starts_with("/routes/") => {
            let host = p.trim_start_matches("/routes/");
            let removed = router.remove(host);
            let status = if removed { StatusCode::OK } else { StatusCode::NOT_FOUND };
            Ok(json(status, format!(r#"{{"removed":{removed}}}"#)))
        }
        _ => Ok(json(StatusCode::NOT_FOUND, r#"{"error":"not found"}"#.into())),
    }
}

/// Build a JSON response.
fn json(status: StatusCode, body: String) -> Response<ProxyBody> {
    let b = Full::new(Bytes::from(body))
        .map_err(|never| match never {})
        .boxed();
    Response::builder()
        .status(status)
        .header(hyper::header::CONTENT_TYPE, "application/json")
        .body(b)
        .expect("static response is always valid")
}

/// Build a small plaintext response with the given status.
fn text(status: StatusCode, msg: &str) -> Response<ProxyBody> {
    let body = Full::new(Bytes::from(msg.to_owned()))
        .map_err(|never| match never {})
        .boxed();
    Response::builder()
        .status(status)
        .header(hyper::header::CONTENT_TYPE, "text/plain")
        .body(body)
        .expect("static response is always valid")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::router::Backend;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};

    /// Start the proxy on an ephemeral port and return its address.
    async fn start_proxy(router: Router) -> SocketAddr {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(serve_listener(listener, Arc::new(router)));
        addr
    }

    /// A minimal HTTP/1.1 upstream that replies `upstream-ok` to any request.
    async fn dummy_upstream() -> SocketAddr {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            loop {
                let (mut sock, _) = listener.accept().await.unwrap();
                tokio::spawn(async move {
                    let mut buf = [0u8; 1024];
                    let _ = sock.read(&mut buf).await;
                    let body = "upstream-ok";
                    let resp = format!(
                        "HTTP/1.1 200 OK\r\ncontent-length: {}\r\nconnection: close\r\n\r\n{}",
                        body.len(),
                        body
                    );
                    let _ = sock.write_all(resp.as_bytes()).await;
                });
            }
        });
        addr
    }

    #[tokio::test]
    async fn healthz_returns_ok() {
        let addr = start_proxy(Router::new()).await;
        let resp = reqwest::get(format!("http://{addr}/healthz")).await.unwrap();
        assert_eq!(resp.status(), 200);
        assert_eq!(resp.text().await.unwrap(), "ok");
    }

    #[tokio::test]
    async fn unknown_host_returns_404() {
        let addr = start_proxy(Router::new()).await;
        let resp = reqwest::get(format!("http://{addr}/anything"))
            .await
            .unwrap();
        assert_eq!(resp.status(), 404);
    }

    #[tokio::test]
    async fn routed_request_is_forwarded() {
        let upstream = dummy_upstream().await;
        let router = Router::new();
        // reqwest sends Host = 127.0.0.1:<proxy port>; lookup strips the port.
        router.upsert("127.0.0.1", Backend::new([upstream.to_string()]));
        let addr = start_proxy(router).await;

        let resp = reqwest::get(format!("http://{addr}/hello")).await.unwrap();
        assert_eq!(resp.status(), 200);
        assert_eq!(resp.text().await.unwrap(), "upstream-ok");
    }

    #[tokio::test]
    async fn admin_api_adds_and_removes_routes() {
        let upstream = dummy_upstream().await;
        let router = Arc::new(Router::new());

        let plistener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let paddr = plistener.local_addr().unwrap();
        tokio::spawn(serve_listener(plistener, router.clone()));

        let alistener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let aaddr = alistener.local_addr().unwrap();
        tokio::spawn(serve_admin_listener(alistener, router.clone()));

        let http = reqwest::Client::new();

        // No route yet → proxy 404.
        let r = http.get(format!("http://{paddr}/")).send().await.unwrap();
        assert_eq!(r.status(), 404);

        // Add a route via the admin API.
        let body = format!(r#"{{"host":"127.0.0.1","replicas":["{upstream}"]}}"#);
        let r = http
            .post(format!("http://{aaddr}/routes"))
            .body(body)
            .send()
            .await
            .unwrap();
        assert_eq!(r.status(), 200);

        // Now the proxy forwards to the upstream.
        let r = http.get(format!("http://{paddr}/")).send().await.unwrap();
        assert_eq!(r.status(), 200);
        assert_eq!(r.text().await.unwrap(), "upstream-ok");

        // GET /routes lists it.
        let listed = http
            .get(format!("http://{aaddr}/routes"))
            .send()
            .await
            .unwrap()
            .text()
            .await
            .unwrap();
        assert!(listed.contains("127.0.0.1"), "routes list: {listed}");

        // DELETE removes it → proxy 404 again.
        let r = http
            .delete(format!("http://{aaddr}/routes/127.0.0.1"))
            .send()
            .await
            .unwrap();
        assert_eq!(r.status(), 200);
        let r = http.get(format!("http://{paddr}/")).send().await.unwrap();
        assert_eq!(r.status(), 404);
    }
}
