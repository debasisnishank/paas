package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegionsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/regions", nil)
	rec := httptest.NewRecorder()

	regionsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var resp regionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(resp.Regions) == 0 {
		t.Fatal("expected at least one region")
	}

	// Mumbai must be present and available — it is the only live Phase-0 region.
	var mum *struct{ available bool }
	for _, r := range resp.Regions {
		if r.Slug == "in-mum-1" {
			ok := r.Status == "available"
			mum = &struct{ available bool }{available: ok}
			if r.JurisdictionProfile.CountryCode != "IN" {
				t.Errorf("in-mum-1 country = %q, want IN", r.JurisdictionProfile.CountryCode)
			}
			if r.JurisdictionProfile.PaymentGateway != "razorpay" {
				t.Errorf("in-mum-1 gateway = %q, want razorpay", r.JurisdictionProfile.PaymentGateway)
			}
		}
	}
	if mum == nil {
		t.Fatal("in-mum-1 not found in catalog")
	}
	if !mum.available {
		t.Error("in-mum-1 status != available")
	}
}
