package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// region mirrors the GET /v1/regions wire shape (a subset of the API's
// domain.Region + status). Defined locally so the CLI does not depend on the
// api module.
type region struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	Jurisdiction struct {
		CountryCode    string `json:"country_code"`
		ResidencyRule  string `json:"residency_rule"`
		PaymentGateway string `json:"payment_gateway"`
		Currency       string `json:"currency"`
	} `json:"jurisdiction"`
}

type regionsResp struct {
	Regions []region `json:"regions"`
}

var regionsCmd = &cobra.Command{
	Use:   "regions",
	Short: "List available regions and their jurisdiction profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		var resp regionsResp
		if err := apiGet(cmd.Context(), "/v1/regions", &resp); err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "SLUG\tREGION\tCOUNTRY\tRESIDENCY\tGATEWAY\tCURRENCY\tSTATUS")
		for _, r := range resp.Regions {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Slug,
				r.DisplayName,
				r.Jurisdiction.CountryCode,
				r.Jurisdiction.ResidencyRule,
				r.Jurisdiction.PaymentGateway,
				r.Jurisdiction.Currency,
				r.Status,
			)
		}
		return w.Flush()
	},
}
