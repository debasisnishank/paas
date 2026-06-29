package nomadspec

import (
	"strings"
	"testing"
)

func TestGenerateContainsKeyFields(t *testing.T) {
	hcl, err := Generate(ServiceSpec{
		Name:         "web",
		Org:          "acme",
		Project:      "backend",
		Region:       "in-mum-1",
		Image:        "registry.local/acme/web@sha256:abc",
		VCPU:         2,
		MemoryMB:     512,
		InternalPort: 8080,
		Replicas:     3,
		Env:          map[string]string{"LOG_LEVEL": "info"},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	wants := []string{
		`job "acme-backend-web"`,
		`driver = "firecracker"`,
		`count = 3`,
		`to = 8080`,
		`vcpu   = 2`,
		`memory = 512`,
		`cpu    = 2000`,
		`image  = "registry.local/acme/web@sha256:abc"`,
		`LOG_LEVEL = "info"`,
		`path     = "/healthz"`,
	}
	for _, w := range wants {
		if !strings.Contains(hcl, w) {
			t.Errorf("generated HCL missing %q\n---\n%s", w, hcl)
		}
	}
}

func TestGenerateDefaults(t *testing.T) {
	hcl, err := Generate(ServiceSpec{
		Name: "api", Org: "o", Project: "p", Image: "img",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, w := range []string{"count = 1", "vcpu   = 1", "memory = 256", "to = 8080"} {
		if !strings.Contains(hcl, w) {
			t.Errorf("default missing %q", w)
		}
	}
}

func TestGenerateRequiresImage(t *testing.T) {
	if _, err := Generate(ServiceSpec{Name: "web", Org: "o", Project: "p"}); err == nil {
		t.Fatal("expected error when image is empty")
	}
}

func TestParseMemoryMB(t *testing.T) {
	cases := map[string]int{
		"512mb": 512, "512m": 512, "1gb": 1024, "2g": 2048, "256": 256, "1 gb": 1024,
	}
	for in, want := range cases {
		got, err := ParseMemoryMB(in)
		if err != nil || got != want {
			t.Errorf("ParseMemoryMB(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := ParseMemoryMB("not-a-size"); err == nil {
		t.Error("expected error for invalid memory value")
	}
	if _, err := ParseMemoryMB(""); err == nil {
		t.Error("expected error for empty value")
	}
}
