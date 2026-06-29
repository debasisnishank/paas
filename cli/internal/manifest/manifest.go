// Package manifest parses the platform.toml deploy manifest.
package manifest

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Manifest is the parsed platform.toml.
type Manifest struct {
	App          App                    `toml:"app"`
	Build        Build                  `toml:"build"`
	Compute      Compute                `toml:"compute"`
	Services     []Service              `toml:"service"`
	Databases    []Database             `toml:"database"`
	Domains      Domains                `toml:"domains"`
	Networking   Networking             `toml:"networking"`
	Environments map[string]Environment `toml:"environments"`
}

type App struct {
	Name    string `toml:"name"`
	Org     string `toml:"org"`
	Project string `toml:"project"`
	Region  string `toml:"region"`
}

type Build struct {
	Builder      string            `toml:"builder"`
	Dockerfile   string            `toml:"dockerfile"`
	BuildArgs    map[string]string `toml:"build_args"`
	BuildSecrets []string          `toml:"build_secrets"`
}

type Compute struct {
	Tier             string    `toml:"tier"`
	VCPU             int       `toml:"vcpu"`
	Memory           string    `toml:"memory"`
	ScaleToZeroAfter string    `toml:"scale_to_zero_after"`
	MinReplicas      int       `toml:"min_replicas"`
	MaxReplicas      int       `toml:"max_replicas"`
	Autoscale        Autoscale `toml:"autoscale"`
}

type Autoscale struct {
	Metric    string `toml:"metric"`
	Threshold int    `toml:"threshold"`
	Cooldown  string `toml:"cooldown"`
}

type Service struct {
	Name         string        `toml:"name"`
	Command      []string      `toml:"command"`
	InternalPort int           `toml:"internal_port"`
	Protocol     string        `toml:"protocol"`
	HealthChecks []HealthCheck `toml:"health_check"`
	Env          []EnvVar      `toml:"env"`
	SecretRefs   []SecretRef   `toml:"secret_refs"`
}

type HealthCheck struct {
	Path     string `toml:"path"`
	Interval string `toml:"interval"`
	Timeout  string `toml:"timeout"`
}

type EnvVar struct {
	Name  string `toml:"name"`
	Value string `toml:"value"`
}

type SecretRef struct {
	EnvVar    string `toml:"env_var"`
	VaultPath string `toml:"vault_path"`
}

type Database struct {
	Name                string `toml:"name"`
	Engine              string `toml:"engine"`
	Version             string `toml:"version"`
	Region              string `toml:"region"`
	DataClass           string `toml:"data_class"`
	AutoBranchOnPreview bool   `toml:"auto_branch_on_preview"`
}

type Domains struct {
	Custom []string `toml:"custom"`
}

type Networking struct {
	Private bool `toml:"private"`
}

type Environment struct {
	MinReplicas      int    `toml:"min_replicas"`
	ScaleToZeroAfter string `toml:"scale_to_zero_after"`
}

// Load reads and validates the manifest at path.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := toml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// Validate checks the required fields are present.
func (m *Manifest) Validate() error {
	switch {
	case m.App.Name == "":
		return errors.New("app.name is required")
	case m.App.Org == "":
		return errors.New("app.org is required")
	case m.App.Project == "":
		return errors.New("app.project is required")
	case len(m.Services) == 0:
		return errors.New("at least one [[service]] is required")
	}
	for i, s := range m.Services {
		if s.Name == "" {
			return fmt.Errorf("service[%d].name is required", i)
		}
	}
	return nil
}

// Service returns the named service, or the first service if name is empty.
func (m *Manifest) ServiceByName(name string) (*Service, error) {
	if name == "" {
		return &m.Services[0], nil
	}
	for i := range m.Services {
		if m.Services[i].Name == name {
			return &m.Services[i], nil
		}
	}
	return nil, fmt.Errorf("service %q not found in manifest", name)
}
