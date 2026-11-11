package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerCatalog keeps a Träger phase with one Träger offering for two
// schools of the Träger and rolls it out (#4704, ADR 0195): each school gets
// its own phase and offering, with tariff and contract template. A later
// seed run whose school already holds a phase of that name records the
// school's refusal; the seed goes on.
func seedTraegerCatalog(client *Client, auth AuthRef, today calendar.Date, schools []int64) error {
	if len(schools) < 2 {
		return nil
	}
	year := today.Year()
	if today.Month() >= time.August {
		year++
	}
	raw, err := client.PostWithAuth(auth, "/traeger/catalog/phases", map[string]any{
		"name": fmt.Sprintf("Trägerphase Schuljahr %d/%02d", year, (year+1)%100), "kind": "school_year",
		"service_start_date": calendar.NewDate(year, time.August, 1).String(),
		"service_end_date":   calendar.NewDate(year+1, time.July, 31).String(),
		"school_ids":         schools[:2],
	})
	if err != nil {
		return fmt.Errorf("create the Träger phase: %w", err)
	}
	phaseID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the Träger phase: %w", err)
	}
	path := fmt.Sprintf("/traeger/catalog/phases/%d", phaseID)
	if _, err := client.PostWithAuth(auth, path+"/offerings", seedTraegerCatalogOffering()); err != nil {
		return fmt.Errorf("create the Träger offering: %w", err)
	}
	raw, err = client.PostWithAuth(auth, path+"/preview", map[string]any{})
	if err != nil {
		return fmt.Errorf("preview the Träger phase: %w", err)
	}
	var preview struct {
		Data struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &preview); err != nil || preview.Data.Fingerprint == "" {
		return fmt.Errorf("read the preview of the Träger phase: %s", raw)
	}
	if _, err := client.PostWithAuth(auth, path+"/rollout", map[string]any{"fingerprint": preview.Data.Fingerprint}); err != nil {
		return fmt.Errorf("roll out the Träger phase: %w", err)
	}
	fmt.Println("  Träger-Büro: Träger phase with one offering rolled out to two schools")
	return nil
}

// seedTraegerCatalogOffering is the early care of Ispringen's price list
// (Modul 1, 6,50 € per weekday, 5,00 € for every further child).
func seedTraegerCatalogOffering() map[string]any {
	days := []string{"mon", "tue", "wed", "thu", "fri"}
	pickup := map[string]string{}
	prices := make([]map[string]any, 0, len(days))
	for _, day := range days {
		pickup[day] = "08:30"
		prices = append(prices, map[string]any{"weekday": day, "monthly_cents": 650, "sibling_cents": 500})
	}
	return map[string]any{
		"fields": map[string]any{
			"name": "Frühbetreuung (Träger)", "description": "07:00 bis 08:30 Uhr", "days_of_week_mode": "fixed",
			"available_days": days, "counts_as_care": true, "selection_rule": "optional", "pickup_times": pickup,
		},
		"prices":            prices,
		"contract_template": map[string]any{"term_kind": "school_year", "notice_months": 1, "notice_to": "month_end"},
	}
}
