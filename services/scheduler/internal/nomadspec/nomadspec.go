// Package nomadspec generates Nomad job specs (HCL) from a service spec.
//
// This is the translation layer the DeployWorkflow will use: platform.toml
// service spec → Nomad job HCL targeting the custom `firecracker` task driver
// (crates/fc-driver). It is pure string generation with no Nomad dependency, so
// it is fully unit-testable without a cluster.
package nomadspec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// ServiceSpec is the minimal input needed to render a Firecracker job.
type ServiceSpec struct {
	Name         string
	Org          string
	Project      string
	Region       string
	Image        string // OCI digest ref
	VCPU         int
	MemoryMB     int
	InternalPort int
	Replicas     int
	HealthPath   string
	Env          map[string]string
}

// JobName is the deterministic Nomad job id for this service.
func (s ServiceSpec) JobName() string {
	return fmt.Sprintf("%s-%s-%s", s.Org, s.Project, s.Name)
}

func (s *ServiceSpec) applyDefaults() {
	if s.VCPU <= 0 {
		s.VCPU = 1
	}
	if s.MemoryMB <= 0 {
		s.MemoryMB = 256
	}
	if s.Replicas <= 0 {
		s.Replicas = 1
	}
	if s.InternalPort <= 0 {
		s.InternalPort = 8080
	}
	if s.HealthPath == "" {
		s.HealthPath = "/healthz"
	}
}

func (s ServiceSpec) validate() error {
	switch {
	case s.Name == "":
		return errors.New("service name is required")
	case s.Org == "":
		return errors.New("org is required")
	case s.Project == "":
		return errors.New("project is required")
	case s.Image == "":
		return errors.New("image is required")
	}
	return nil
}

var jobTemplate = template.Must(template.New("job").Parse(`job "{{ .JobName }}" {
  region      = "{{ .Region }}"
  datacenters = ["{{ .Region }}"]
  type        = "service"

  update {
    max_parallel = 1
    auto_revert  = true
    health_check = "checks"
  }

  group "{{ .Name }}" {
    count = {{ .Replicas }}

    network {
      mode = "bridge"
      port "http" {
        to = {{ .InternalPort }}
      }
    }

    task "{{ .Name }}" {
      driver = "firecracker"

      config {
        image  = "{{ .Image }}"
        vcpu   = {{ .VCPU }}
        memory = {{ .MemoryMB }}
      }
{{- if .Env }}

      env {
{{- range $k, $v := .Env }}
        {{ $k }} = "{{ $v }}"
{{- end }}
      }
{{- end }}

      resources {
        cpu    = {{ .CPUMHz }}
        memory = {{ .MemoryMB }}
      }

      service {
        name = "{{ .JobName }}"
        port = "http"

        check {
          type     = "http"
          path     = "{{ .HealthPath }}"
          interval = "10s"
          timeout  = "3s"
        }
      }
    }
  }
}
`))

// Generate renders the Nomad job HCL for spec.
func Generate(spec ServiceSpec) (string, error) {
	spec.applyDefaults()
	if err := spec.validate(); err != nil {
		return "", err
	}

	// Nomad's resources.cpu is in MHz; rough heuristic of 1 vCPU ≈ 1000 MHz.
	data := struct {
		ServiceSpec
		CPUMHz int
	}{ServiceSpec: spec, CPUMHz: spec.VCPU * 1000}

	var b strings.Builder
	if err := jobTemplate.Execute(&b, data); err != nil {
		return "", fmt.Errorf("render job spec: %w", err)
	}
	return b.String(), nil
}

// ParseMemoryMB converts a manifest memory string ("512mb", "1gb", "256m",
// "2g") into megabytes. A bare number is treated as megabytes.
func ParseMemoryMB(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, errors.New("empty memory value")
	}
	mult := 1
	switch {
	case strings.HasSuffix(s, "gb"):
		mult, s = 1024, strings.TrimSuffix(s, "gb")
	case strings.HasSuffix(s, "g"):
		mult, s = 1024, strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "mb"):
		s = strings.TrimSuffix(s, "mb")
	case strings.HasSuffix(s, "m"):
		s = strings.TrimSuffix(s, "m")
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %q", s)
	}
	return n * mult, nil
}
