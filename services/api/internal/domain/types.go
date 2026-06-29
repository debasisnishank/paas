// Package domain defines the core platform domain model.
//
// Hierarchy: Org → Team → Project → Service → Deployment
// Each entity carries a JurisdictionProfile ref for data-placement decisions.
package domain

import (
	"time"
)

// ID types for compile-time safety.
type (
	OrgID        string
	TeamID       string
	ProjectID    string
	ServiceID    string
	DeploymentID string
	RegionID     string
	EnvID        string
)

// Org is the top-level billing and identity boundary.
type Org struct {
	ID          OrgID     `json:"id"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"display_name"`
	PlanTier    PlanTier  `json:"plan_tier"`
	CreatedAt   time.Time `json:"created_at"`
}

// PlanTier maps to compute tiers defined in the blueprint.
type PlanTier string

const (
	TierShared     PlanTier = "shared"     // bin-packed microVMs, burstable
	TierDedicated  PlanTier = "dedicated"  // pinned reserved hosts
	TierEnterprise PlanTier = "enterprise" // whole-node / whole-rack, isolated DC
)

// Project groups services and environments under a team.
type Project struct {
	ID        ProjectID  `json:"id"`
	OrgID     OrgID      `json:"org_id"`
	TeamID    TeamID     `json:"team_id"`
	Slug      string     `json:"slug"`
	CreatedAt time.Time  `json:"created_at"`
}

// Environment is a named deploy target (production, staging, pr-<n>).
type Environment struct {
	ID        EnvID     `json:"id"`
	ProjectID ProjectID `json:"project_id"`
	Name      string    `json:"name"`        // "production" | "staging" | "pr-123"
	IsPreview bool      `json:"is_preview"`  // ephemeral per-PR environment
	RegionID  RegionID  `json:"region_id"`
}

// Service is a deployable unit within a project.
type Service struct {
	ID          ServiceID   `json:"id"`
	ProjectID   ProjectID   `json:"project_id"`
	Name        string      `json:"name"`
	Type        ServiceType `json:"type"`
	BuildConfig BuildConfig `json:"build_config"`
}

// ServiceType enumerates what can be deployed.
type ServiceType string

const (
	ServiceTypeApp      ServiceType = "app"
	ServiceTypeWorker   ServiceType = "worker"
	ServiceTypeCron     ServiceType = "cron"
	ServiceTypeDatabase ServiceType = "database" // serverless PG instance
)

// BuildConfig captures how a service is built.
type BuildConfig struct {
	Builder    Builder  `json:"builder"`           // nixpacks | buildpack | dockerfile
	Dockerfile string   `json:"dockerfile,omitempty"`
	BuildArgs  []KV     `json:"build_args,omitempty"`
	Secrets    []string `json:"build_secret_refs,omitempty"` // Vault paths
}

// Builder selects the build strategy.
type Builder string

const (
	BuilderNixpacks   Builder = "nixpacks"
	BuilderBuildpack  Builder = "buildpack"
	BuilderDockerfile Builder = "dockerfile"
)

// Deployment is an immutable snapshot of a service state in an environment.
type Deployment struct {
	ID          DeploymentID `json:"id"`
	ServiceID   ServiceID    `json:"service_id"`
	EnvID       EnvID        `json:"env_id"`
	Image       string       `json:"image"` // OCI digest ref
	Status      DeployStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	FinishedAt  *time.Time   `json:"finished_at,omitempty"`
}

// DeployStatus tracks lifecycle.
type DeployStatus string

const (
	DeployPending   DeployStatus = "pending"
	DeployBuilding  DeployStatus = "building"
	DeployDeploying DeployStatus = "deploying"
	DeployLive      DeployStatus = "live"
	DeployFailed    DeployStatus = "failed"
	DeployStopped   DeployStatus = "stopped"
)

// Region is the abstract placement unit — cloud or bare-metal DC.
type Region struct {
	ID                 RegionID           `json:"id"`
	Slug               string             `json:"slug"` // e.g. "in-mum-1", "ae-dxb-1"
	DisplayName        string             `json:"display_name"`
	JurisdictionProfile JurisdictionProfile `json:"jurisdiction"`
}

// JurisdictionProfile encodes data-residency and compliance rules for a region.
// Adding a new country = adding a profile, not a code fork.
type JurisdictionProfile struct {
	CountryCode      string           `json:"country_code"`       // ISO 3166-1 alpha-2
	ResidencyRule    ResidencyRule    `json:"residency_rule"`
	TransferMechanism string          `json:"transfer_mechanism"` // adequacy | SCC | consent | authorization
	BreachAuthority  string          `json:"breach_authority"`   // "DPB" | "SDAIA" | "DIFC" …
	BreachWindowHrs  int             `json:"breach_window_hrs"`  // 72 | 6 (CERT-In)
	PaymentGateway   string          `json:"payment_gateway"`    // "razorpay" | "paytabs" | "paystack"
	Currency         string          `json:"currency"`           // ISO 4217
	TaxModel         string          `json:"tax_model"`          // "gst" | "vat_uae" | "vat_ksa_zatca"
	DataClassPinning []DataClassPin  `json:"data_class_pinning"`
}

// ResidencyRule determines where data may physically live.
type ResidencyRule string

const (
	ResidencyNone ResidencyRule = "none" // no restriction
	ResidencySoft ResidencyRule = "soft" // local copy; SCC-style transfers allowed
	ResidencyHard ResidencyRule = "hard" // must stay in-country (KSA, payment data)
)

// DataClassPin maps a data class to its residency requirement.
type DataClassPin struct {
	Class    DataClass     `json:"class"`
	Residency ResidencyRule `json:"residency"`
}

// DataClass tags records for placement decisions.
type DataClass string

const (
	DataClassPersonal   DataClass = "personal"
	DataClassSensitive  DataClass = "sensitive"
	DataClassPayment    DataClass = "payment"
	DataClassTelemetry  DataClass = "telemetry" // non-regulated; flows freely
)

// KV is a generic key-value pair.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
