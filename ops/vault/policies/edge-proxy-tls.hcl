// Vault policy: edge-proxy
// Reads TLS certificates for all tenant domains; issues ACME challenge tokens.

path "kv/data/tls/*" {
  capabilities = ["read", "list"]
}

path "pki/issue/edge-proxy-wildcard" {
  capabilities = ["update"]
}

path "auth/token/renew-self" {
  capabilities = ["update"]
}
