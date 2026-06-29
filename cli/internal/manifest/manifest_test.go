package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `
[app]
name = "my-api"
org = "acme"
project = "backend"
region = "in-mum-1"

[build]
builder = "nixpacks"

[compute]
tier = "shared"
vcpu = 1
memory = "512mb"
min_replicas = 1
max_replicas = 10

[compute.autoscale]
metric = "rps"
threshold = 50

[[service]]
name = "web"
internal_port = 8080
protocol = "http"

[[service.env]]
name = "LOG_LEVEL"
value = "info"

[[service.secret_refs]]
env_var = "DATABASE_URL"
vault_path = "kv/my-api/db-url"

[[database]]
name = "main"
engine = "postgres"
data_class = "personal"

[environments.production]
min_replicas = 2
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "platform.toml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValidManifest(t *testing.T) {
	m, err := Load(writeTemp(t, sample))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.App.Name != "my-api" || m.App.Org != "acme" {
		t.Errorf("app = %+v", m.App)
	}
	if len(m.Services) != 1 || m.Services[0].InternalPort != 8080 {
		t.Fatalf("services = %+v", m.Services)
	}
	if len(m.Services[0].Env) != 1 || m.Services[0].Env[0].Name != "LOG_LEVEL" {
		t.Errorf("env = %+v", m.Services[0].Env)
	}
	if len(m.Services[0].SecretRefs) != 1 || m.Services[0].SecretRefs[0].VaultPath != "kv/my-api/db-url" {
		t.Errorf("secret_refs = %+v", m.Services[0].SecretRefs)
	}
	if m.Compute.Autoscale.Threshold != 50 {
		t.Errorf("autoscale threshold = %d", m.Compute.Autoscale.Threshold)
	}
	if m.Environments["production"].MinReplicas != 2 {
		t.Errorf("production min_replicas = %d", m.Environments["production"].MinReplicas)
	}
}

func TestValidateRequiresFields(t *testing.T) {
	_, err := Load(writeTemp(t, `
[app]
org = "acme"
project = "backend"
[[service]]
name = "web"
`))
	if err == nil {
		t.Fatal("expected error for missing app.name")
	}
}

func TestServiceByName(t *testing.T) {
	m, err := Load(writeTemp(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	s, err := m.ServiceByName("")
	if err != nil || s.Name != "web" {
		t.Fatalf("default service = %v, %v", s, err)
	}
	if _, err := m.ServiceByName("nope"); err == nil {
		t.Error("expected error for unknown service")
	}
}

// If the repo's example manifest is reachable, it must parse and validate.
func TestExampleManifestParses(t *testing.T) {
	example := filepath.Join("..", "..", "..", "platform.toml.example")
	if _, err := os.Stat(example); err != nil {
		t.Skip("example manifest not present")
	}
	if _, err := Load(example); err != nil {
		t.Fatalf("example manifest failed to load: %v", err)
	}
}
