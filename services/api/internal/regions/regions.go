// Package regions provides the platform region catalog.
//
// Phase 0: the catalog is an in-memory seed. Mumbai (in-mum-1) is the only
// region with live capacity; the others are published as "planned" so the
// jurisdiction-profile model (residency, payment gateway, tax) is exercised
// end to end before any of those regions are actually stood up.
//
// Adding a country = adding an Entry here, never a code branch — see
// CLAUDE.md "config-driven jurisdiction profiles".
package regions

import "github.com/threemates/antariksh/services/api/internal/domain"

// Status reflects whether a region can currently accept workloads.
type Status string

const (
	StatusAvailable Status = "available" // live, accepting deploys
	StatusPlanned   Status = "planned"   // on the roadmap, no capacity yet
)

// Entry is a region plus its rollout status, as served by GET /v1/regions.
// It embeds domain.Region so the wire shape stays aligned with the domain model.
type Entry struct {
	domain.Region
	Status Status `json:"status"`
}

// catalog is the canonical seed. Order here is the display order.
var catalog = []Entry{
	{
		Region: domain.Region{
			ID:          "in-mum-1",
			Slug:        "in-mum-1",
			DisplayName: "Mumbai, India",
			JurisdictionProfile: domain.JurisdictionProfile{
				CountryCode:       "IN",
				ResidencyRule:     domain.ResidencySoft,
				TransferMechanism: "consent",
				BreachAuthority:   "DPB",
				BreachWindowHrs:   72,
				PaymentGateway:    "razorpay",
				Currency:          "INR",
				TaxModel:          "gst",
				DataClassPinning: []domain.DataClassPin{
					{Class: domain.DataClassPayment, Residency: domain.ResidencyHard},
				},
			},
		},
		Status: StatusAvailable,
	},
	{
		Region: domain.Region{
			ID:          "in-hyd-1",
			Slug:        "in-hyd-1",
			DisplayName: "Hyderabad, India",
			JurisdictionProfile: domain.JurisdictionProfile{
				CountryCode:       "IN",
				ResidencyRule:     domain.ResidencySoft,
				TransferMechanism: "consent",
				BreachAuthority:   "DPB",
				BreachWindowHrs:   72,
				PaymentGateway:    "razorpay",
				Currency:          "INR",
				TaxModel:          "gst",
				DataClassPinning: []domain.DataClassPin{
					{Class: domain.DataClassPayment, Residency: domain.ResidencyHard},
				},
			},
		},
		Status: StatusPlanned,
	},
	{
		Region: domain.Region{
			ID:          "ae-dxb-1",
			Slug:        "ae-dxb-1",
			DisplayName: "Dubai, UAE",
			JurisdictionProfile: domain.JurisdictionProfile{
				CountryCode:       "AE",
				ResidencyRule:     domain.ResidencySoft,
				TransferMechanism: "adequacy",
				BreachAuthority:   "DIFC",
				BreachWindowHrs:   72,
				PaymentGateway:    "paytabs",
				Currency:          "AED",
				TaxModel:          "vat_uae",
			},
		},
		Status: StatusPlanned,
	},
	{
		Region: domain.Region{
			ID:          "sa-ruh-1",
			Slug:        "sa-ruh-1",
			DisplayName: "Riyadh, Saudi Arabia",
			JurisdictionProfile: domain.JurisdictionProfile{
				CountryCode:       "SA",
				ResidencyRule:     domain.ResidencyHard,
				TransferMechanism: "authorization",
				BreachAuthority:   "SDAIA",
				BreachWindowHrs:   72,
				PaymentGateway:    "paytabs",
				Currency:          "SAR",
				TaxModel:          "vat_ksa_zatca",
				DataClassPinning: []domain.DataClassPin{
					{Class: domain.DataClassPersonal, Residency: domain.ResidencyHard},
					{Class: domain.DataClassSensitive, Residency: domain.ResidencyHard},
					{Class: domain.DataClassPayment, Residency: domain.ResidencyHard},
				},
			},
		},
		Status: StatusPlanned,
	},
}

// Catalog returns a copy of the region catalog in display order.
func Catalog() []Entry {
	out := make([]Entry, len(catalog))
	copy(out, catalog)
	return out
}
