package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// seedMealBillingStep gives the demo its Mensa billing (#4686) through the
// routes, the way the people work: the Träger bills the Mensa of every
// school per meal at 4,80 € from three months back, and the OGS-Leitung of
// the demo school releases the last month that has ended, with the counts
// it was shown. It runs before the monthly run, so the run reads the
// release.
type seedMealBillingStep struct{ seeder *Seeder }

func (seedMealBillingStep) Name() string { return "Seeding the Mensa billing" }

// seedMealPriceCents is the price per meal of the demo (Hardt, MK-001).
const seedMealPriceCents = 480

type seedMealMonthSummary struct {
	Month  string `json:"month"`
	Status string `json:"status"`
}

type seedMealMonth struct {
	Status      string `json:"status"`
	ChildCount  int    `json:"child_count"`
	MealCount   int    `json:"meal_count"`
	Fingerprint string `json:"fingerprint"`
}

func (s seedMealBillingStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" {
		return fmt.Errorf("the Mensa billing seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	today, err := rt.historyToday()
	if err != nil {
		return err
	}
	if err := seedMealHistory(ctx, rt, s.seeder); err != nil {
		return err
	}
	from := calendar.NewDate(today.Year(), today.Month()-3, 1)
	prices, err := seedMealPrices(rt.Client, auth, from)
	if err != nil {
		return err
	}
	rt.Client.BindAuth(rt.TenantAuth)
	released, err := seedMealRelease(rt.Client, calendar.NewDate(today.Year(), today.Month()-1, 1))
	if err != nil {
		return err
	}
	fmt.Printf("  Mensa billing: %d prices, released %s\n", prices, released)
	return nil
}

// seedMealPrices bills the Mensa of every school of the Träger per meal from
// month on. A school that already has a price keeps it.
func seedMealPrices(client *Client, auth AuthRef, month calendar.Date) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/care-meal-tariffs/")
	if err != nil {
		return 0, fmt.Errorf("read the Mensa prices: %w", err)
	}
	var envelope struct {
		Data struct {
			Schools []struct {
				ID int64 `json:"id"`
			} `json:"schools"`
			Tariffs []struct {
				SchoolID int64 `json:"school_id"`
			} `json:"tariffs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the Mensa prices: %w", err)
	}
	priced := map[int64]bool{}
	for _, tariff := range envelope.Data.Tariffs {
		priced[tariff.SchoolID] = true
	}
	saved := 0
	for _, school := range envelope.Data.Schools {
		if priced[school.ID] {
			continue
		}
		if _, err := client.PutWithAuth(auth, "/traeger/care-meal-tariffs/", map[string]any{
			"school_id": school.ID, "valid_from": month.String(), "mode": "per_meal", "amount_cents": seedMealPriceCents,
		}); err != nil {
			return saved, fmt.Errorf("set the Mensa price of school %d: %w", school.ID, err)
		}
		saved++
	}
	return saved, nil
}

// seedMealRelease releases the latest ended month of the demo school that
// is not released yet, with the counts the OGS-Leitung reads first.
func seedMealRelease(client *Client, month calendar.Date) (string, error) {
	months, err := seedMealMonths(client)
	if err != nil {
		return "", err
	}
	for _, summary := range months {
		if summary.Month != month.String() {
			continue
		}
		if summary.Status == "released" {
			return "nothing new", nil
		}
		if summary.Status == "open" {
			return summary.Month, seedReleaseMealMonth(client, summary.Month)
		}
	}
	return "nothing", nil
}

// seedMealMonths reads the Mensa months of the demo school, newest first. A
// school without the meal registration answers no list; it has no Mensa
// month to release.
func seedMealMonths(client *Client) ([]seedMealMonthSummary, error) {
	raw, err := client.Get("/api/meal-billing-months/")
	if err != nil {
		return nil, fmt.Errorf("read the Mensa months: %w", err)
	}
	var overview struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &overview); err != nil {
		return nil, fmt.Errorf("read the Mensa months: %w", err)
	}
	if len(overview.Data) == 0 || overview.Data[0] != '[' {
		return nil, nil
	}
	var months []seedMealMonthSummary
	if err := json.Unmarshal(overview.Data, &months); err != nil {
		return nil, fmt.Errorf("read the Mensa months: %w", err)
	}
	return months, nil
}

// seedReleaseMealMonth releases month with the counts the page shows.
func seedReleaseMealMonth(client *Client, month string) error {
	raw, err := client.Get("/api/meal-billing-months/" + month)
	if err != nil {
		return fmt.Errorf("read the Mensa month %s: %w", month, err)
	}
	var detail struct {
		Data seedMealMonth `json:"data"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		return fmt.Errorf("read the Mensa month %s: %w", month, err)
	}
	if _, err := client.Post("/api/meal-billing-months/"+month+"/release", map[string]any{
		"child_count": detail.Data.ChildCount, "meal_count": detail.Data.MealCount, "fingerprint": detail.Data.Fingerprint,
	}); err != nil {
		return fmt.Errorf("release the Mensa month %s: %w", month, err)
	}
	return nil
}
