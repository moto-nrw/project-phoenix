package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerCareMonthsStep gives the demo Träger its monthly contribution
// (#4684) through the Träger routes, the way the Abrechnung works: the number
// range of the monthly care, a price list for every booked module of each
// school, a released run of the first month with an active contract and the
// preview of the month after it.
type seedTraegerCareMonthsStep struct{}

func (seedTraegerCareMonthsStep) Name() string { return "Seeding the Träger monthly contributions" }

// seedCareNumberPattern is the number range of the monthly invoices.
const seedCareNumberPattern = "MB-{JJJJ}-{NNNN}"

type seedCareModule struct {
	SchoolID       int64  `json:"school_id"`
	CareOfferingID int64  `json:"care_offering_id"`
	ModuleName     string `json:"module_name"`
}

type seedCareRunDetail struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	PreviewCount int    `json:"preview_count"`
	PreviewCents int64  `json:"preview_cents"`
}

func (seedTraegerCareMonthsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" {
		return fmt.Errorf("the Träger monthly contribution seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/number-ranges/care", map[string]any{"pattern": seedCareNumberPattern}); err != nil {
		return fmt.Errorf("keep the number range of the monthly care: %w", err)
	}
	current := calendar.TodayDate()
	current = calendar.NewDate(current.Year(), current.Month(), 1)
	// The Worker concludes the contracts of Weg b on its own tick (#4680);
	// a contract already active bills its first month, the others bill once
	// it is concluded.
	first, err := seedFirstActiveMonth(rt.Client, auth)
	if err != nil {
		return err
	}
	from := current
	if !first.IsZero() && first.Before(current) {
		from = first
	}
	prices, err := seedCarePriceLists(rt.Client, auth, from)
	if err != nil {
		return err
	}
	released := 0
	if !first.IsZero() {
		if released, err = seedCareRun(rt.Client, auth, first, true); err != nil {
			return err
		}
	}
	if first != current {
		if _, err := seedCareRun(rt.Client, auth, current, false); err != nil {
			return err
		}
	}
	fmt.Printf("  Träger monthly contributions: %d prices, %d invoices issued\n", prices, released)
	return nil
}

// seedFirstActiveMonth is the month of the earliest effective date of an
// active contract; zero without one.
func seedFirstActiveMonth(client *Client, auth AuthRef) (calendar.Date, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contracts/?status=aktiv")
	if err != nil {
		return "", fmt.Errorf("read the active contracts: %w", err)
	}
	var envelope struct {
		Data struct {
			Contracts []struct {
				EffectiveFrom calendar.Date `json:"effective_from"`
			} `json:"contracts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("read the active contracts: %w", err)
	}
	var first calendar.Date
	for _, contract := range envelope.Data.Contracts {
		if first.IsZero() || contract.EffectiveFrom.Before(first) {
			first = contract.EffectiveFrom
		}
	}
	if first.IsZero() {
		return "", nil
	}
	return calendar.NewDate(first.Year(), first.Month(), 1), nil
}

// seedCareModulePrice is the monthly price of one booked weekday: early care
// costs less than the short care, and that less than the full day.
func seedCareModulePrice(module seedCareModule) int64 {
	name := strings.ToLower(module.ModuleName)
	switch {
	case strings.Contains(name, "früh"):
		return 1500
	case strings.Contains(name, "14"), strings.Contains(name, "kurz"), strings.Contains(name, "mittag"):
		return 3200
	default:
		return 4550
	}
}

// seedCarePriceLists sets the prices of every school from month on, for the
// weekdays Monday to Friday of each care offering of the Träger. A school
// that already has prices keeps them.
func seedCarePriceLists(client *Client, auth AuthRef, month calendar.Date) (int, error) {
	modules, err := seedCareOfferings(client, auth)
	if err != nil {
		return 0, err
	}
	raw, err := client.GetWithAuth(auth, "/traeger/care-tariffs/")
	if err != nil {
		return 0, fmt.Errorf("read the price list: %w", err)
	}
	var envelope struct {
		Data struct {
			Tariffs []struct {
				SchoolID int64 `json:"school_id"`
			} `json:"tariffs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the price list: %w", err)
	}
	priced := map[int64]bool{}
	for _, tariff := range envelope.Data.Tariffs {
		priced[tariff.SchoolID] = true
	}
	bySchool := map[int64][]map[string]any{}
	for _, module := range modules {
		if priced[module.SchoolID] {
			continue
		}
		for _, weekday := range []string{"mon", "tue", "wed", "thu", "fri"} {
			bySchool[module.SchoolID] = append(bySchool[module.SchoolID], map[string]any{
				"care_offering_id": module.CareOfferingID, "module_name": module.ModuleName,
				"weekday": weekday, "monthly_cents": seedCareModulePrice(module),
			})
		}
	}
	schools := make([]int64, 0, len(bySchool))
	for schoolID := range bySchool {
		schools = append(schools, schoolID)
	}
	sort.Slice(schools, func(i, j int) bool { return schools[i] < schools[j] })
	count := 0
	for _, schoolID := range schools {
		if _, err := client.PutWithAuth(auth, "/traeger/care-tariffs/", map[string]any{
			"school_id": schoolID, "valid_from": month.String(), "prices": bySchool[schoolID],
		}); err != nil {
			return 0, fmt.Errorf("set the prices of school %d: %w", schoolID, err)
		}
		count += len(bySchool[schoolID])
	}
	return count, nil
}

// seedCareOfferings lists the care offerings of the Träger's schools from
// the contract templates.
func seedCareOfferings(client *Client, auth AuthRef) ([]seedCareModule, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contract-templates/")
	if err != nil {
		return nil, fmt.Errorf("read the care offerings: %w", err)
	}
	var envelope struct {
		Data []struct {
			SchoolID     int64  `json:"school_id"`
			OfferingID   int64  `json:"offering_id"`
			OfferingName string `json:"offering_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("read the care offerings: %w", err)
	}
	modules := make([]seedCareModule, 0, len(envelope.Data))
	for _, offering := range envelope.Data {
		modules = append(modules, seedCareModule{SchoolID: offering.SchoolID, CareOfferingID: offering.OfferingID, ModuleName: offering.OfferingName})
	}
	return modules, nil
}

// seedCareRun starts the run of a month, or takes the open one, and
// releases it when asked and there is something to bill. It returns the
// number of issued invoices.
func seedCareRun(client *Client, auth AuthRef, month calendar.Date, release bool) (int, error) {
	raw, err := client.PostWithAuth(auth, "/traeger/care-billing-runs/", map[string]any{"billing_month": month.String()})
	if err != nil {
		if strings.Contains(err.Error(), "parent_billing.care_run_open") {
			return 0, nil
		}
		return 0, fmt.Errorf("start the monthly run of %s: %w", month, err)
	}
	runID, err := decodeSeedEntityID(raw)
	if err != nil {
		return 0, fmt.Errorf("read the monthly run: %w", err)
	}
	raw, err = client.GetWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d", runID))
	if err != nil {
		return 0, fmt.Errorf("check the monthly run: %w", err)
	}
	var envelope struct {
		Data seedCareRunDetail `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("check the monthly run: %w", err)
	}
	run := envelope.Data
	if !release || run.PreviewCount == 0 {
		return 0, nil
	}
	if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/release", runID), map[string]any{
		"document_count": run.PreviewCount, "total_cents": run.PreviewCents,
	}); err != nil {
		return 0, fmt.Errorf("release the monthly run: %w", err)
	}
	return run.PreviewCount, nil
}
