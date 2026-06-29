// Vault policy: antariksh-api service
// Grants read access to its own secrets; no lateral movement.

path "kv/data/antariksh/api" {
  capabilities = ["read"]
}

path "kv/data/antariksh/api/*" {
  capabilities = ["read"]
}

// Allow renewal of the service's own token
path "auth/token/renew-self" {
  capabilities = ["update"]
}

// PKI: issue short-lived certs for mTLS (SPIFFE-style)
path "pki/issue/antariksh-services" {
  capabilities = ["update"]
}
