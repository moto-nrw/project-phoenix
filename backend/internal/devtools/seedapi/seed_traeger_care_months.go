package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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
	Fingerprint  string `json:"fingerprint"`
	// ExcludedCount and Invoices show the review of the open run (#5916).
	ExcludedCount int `json:"excluded_count"`
	Invoices      []struct {
		ContractID int64 `json:"contract_id"`
	} `json:"invoices"`
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
	prices, err := seedCarePriceLists(rt.Client, auth, from, seedFreeMonth(first, current))
	if err != nil {
		return err
	}
	rules, evidences, err := seedCareReductions(rt.Client, auth, from)
	if err != nil {
		return err
	}
	released, err := seedCareRuns(rt.Client, auth, first, current)
	if err != nil {
		return err
	}
	// The months after the current one go into a Jahresbescheid (#6219).
	notices, err := seedCareNotices(rt.Client, auth, current)
	if err != nil {
		return err
	}
	fmt.Printf("  Träger monthly contributions: %d prices, %d rules, %d evidences, %d invoices and %d fee notices issued\n",
		prices, rules, evidences, released, notices)
	return nil
}

// seedCareRuns releases the run of the first active month and leaves the
// run of the current month open for its review (#5916).
func seedCareRuns(client *Client, auth AuthRef, first, current calendar.Date) (int, error) {
	released := 0
	var err error
	if !first.IsZero() {
		if released, err = seedCareRun(client, auth, first, true); err != nil {
			return 0, err
		}
	}
	if first != current {
		if _, err := seedCareRun(client, auth, current, false); err != nil {
			return 0, err
		}
	}
	return released, nil
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

// seedFreeMonth is the free month of the first school: August like
// Ispringen, unless the seed bills August (its first or current month);
// then the month before. The runs of the seed must issue invoices, so the
// invoices, their reductions and cost bearer shares are on every dev
// machine.
func seedFreeMonth(first, current calendar.Date) int {
	billed := func(month int) bool {
		return (!first.IsZero() && int(first.Month()) == month) || int(current.Month()) == month
	}
	month := 8
	for billed(month) {
		month = (month+10)%12 + 1
	}
	return month
}

// seedCareScale prices a module by days per week: the step grows by less
// than a weekday's price per day; the meal of a lunch module comes on top.
func seedCareScale(module seedCareModule) []map[string]any {
	daily := seedCareModulePrice(module) / 4
	lunch := strings.Contains(strings.ToLower(module.ModuleName), "mittag")
	steps := make([]map[string]any, 0, 5)
	for days := 1; days <= 5; days++ {
		cents := daily*int64(days) - int64(days-1)*daily/10
		steps = append(steps, map[string]any{
			"care_offering_id": module.CareOfferingID, "module_name": module.ModuleName,
			"days_per_week": days, "monthly_cents": cents, "meal_extra": lunch,
		})
	}
	return steps
}

// seedCarePriceLists sets the prices of every school from month on, for the
// weekdays Monday to Friday of each care offering of the Träger. A school
// that already has prices keeps them.
func seedCarePriceLists(client *Client, auth AuthRef, month calendar.Date, freeMonth int) (int, error) {
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
	schools, bySchool := seedCarePrices(modules, priced)
	count := 0
	for _, schoolID := range schools {
		// The first school bills nothing in one month, like Ispringen in
		// August (#6196).
		free := []int{}
		if schoolID == schools[0] {
			free = []int{freeMonth}
		}
		if _, err := client.PutWithAuth(auth, "/traeger/care-tariffs/", map[string]any{
			"school_id": schoolID, "valid_from": month.String(), "free_months": free, "prices": bySchool[schoolID],
		}); err != nil {
			return 0, fmt.Errorf("set the prices of school %d: %w", schoolID, err)
		}
		count += len(bySchool[schoolID])
	}
	return count, nil
}

// seedCarePrices builds the prices of every school without prices, by
// school ID. Every second school prices by days per week (#6196), so both
// price forms are on every dev machine; further children pay three quarters.
func seedCarePrices(modules []seedCareModule, priced map[int64]bool) ([]int64, map[int64][]map[string]any) {
	order := []int64{}
	for _, module := range modules {
		if !priced[module.SchoolID] && !slices.Contains(order, module.SchoolID) {
			order = append(order, module.SchoolID)
		}
	}
	slices.Sort(order)
	bySchool := map[int64][]map[string]any{}
	for _, module := range modules {
		switch index := slices.Index(order, module.SchoolID); {
		case index < 0:
		case index%2 == 1:
			bySchool[module.SchoolID] = append(bySchool[module.SchoolID], seedCareScale(module)...)
		default:
			bySchool[module.SchoolID] = append(bySchool[module.SchoolID], seedCareWeekdays(module)...)
		}
	}
	return order, bySchool
}

// seedCareWeekdays prices Monday to Friday of a module.
func seedCareWeekdays(module seedCareModule) []map[string]any {
	cents := seedCareModulePrice(module)
	prices := make([]map[string]any, 0, 5)
	for _, weekday := range []string{"mon", "tue", "wed", "thu", "fri"} {
		prices = append(prices, map[string]any{
			"care_offering_id": module.CareOfferingID, "module_name": module.ModuleName,
			"weekday": weekday, "monthly_cents": cents, "sibling_cents": cents * 3 / 4,
		})
	}
	return prices
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

// seedCareRun takes the open run of a month or starts one, and releases it
// when asked and there is something to bill. It returns the number of
// issued invoices.
func seedCareRun(client *Client, auth AuthRef, month calendar.Date, release bool) (int, error) {
	runID, err := seedStartCareRun(client, auth, month)
	if err != nil {
		return 0, err
	}
	if err := seedBillingTerms(client, auth, runID, month); err != nil {
		return 0, err
	}
	raw, err := client.GetWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d", runID))
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
	if !release {
		return 0, seedCareReview(client, auth, month, run)
	}
	if run.PreviewCount == 0 {
		return 0, nil
	}
	if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/release", runID), map[string]any{
		"document_count": run.PreviewCount, "total_cents": run.PreviewCents, "fingerprint": run.Fingerprint,
	}); err != nil {
		if seedConflictCode(err, seedCareRunReleased) {
			return 0, nil
		}
		return 0, fmt.Errorf("release the monthly run: %w", err)
	}
	return run.PreviewCount, nil
}

// The registered codes of a run another seed of the same Träger started or
// released a moment earlier: the demo queue prepares several schools of one
// Träger at once.
const (
	seedCareRunOpen     = "parent_billing.care_run_open"
	seedCareRunReleased = "parent_billing.billing_run_released"
)

// seedStartCareRun takes the open run of a month or starts one. When another
// seed of the same Träger started it in between, it takes that run.
func seedStartCareRun(client *Client, auth AuthRef, month calendar.Date) (int64, error) {
	runID, open, err := seedOpenCareRun(client, auth, month)
	if err != nil || open {
		return runID, err
	}
	raw, err := client.PostWithAuth(auth, "/traeger/care-billing-runs/", map[string]any{"billing_month": month.String()})
	if seedConflictCode(err, seedCareRunOpen) {
		runID, open, err = seedOpenCareRun(client, auth, month)
		if err == nil && !open {
			err = fmt.Errorf("the monthly run of %s is neither open nor startable", month)
		}
		return runID, err
	}
	if err != nil {
		return 0, fmt.Errorf("start the monthly run of %s: %w", month, err)
	}
	if runID, err = decodeSeedEntityID(raw); err != nil {
		return 0, fmt.Errorf("read the monthly run: %w", err)
	}
	return runID, nil
}

// seedCareReview gives the open run of the current month the review the
// SJR checks before the release (#5916): the last day of the month as its
// invoice date and one invoice left out with a reason, when it has two.
func seedCareReview(client *Client, auth AuthRef, month calendar.Date, run seedCareRunDetail) error {
	invoiceDate := calendar.NewDate(month.Year(), month.Month()+1, 1).AddDays(-1)
	if _, err := client.PutWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/invoice-date", run.ID),
		map[string]any{"invoice_date": invoiceDate.String()}); err != nil {
		return fmt.Errorf("set the invoice date of the monthly run: %w", err)
	}
	if run.ExcludedCount > 0 || len(run.Invoices) < 2 {
		return nil
	}
	if _, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/care-billing-runs/%d/exclusions", run.ID), map[string]any{
		"contract_id": run.Invoices[len(run.Invoices)-1].ContractID, "reason": "Kündigung zum Monatsende wird noch geprüft",
	}); err != nil {
		return fmt.Errorf("exclude an invoice of the monthly run: %w", err)
	}
	return nil
}

// seedOpenCareRun finds the run of the month that is still a preview; a
// month has at most one.
func seedOpenCareRun(client *Client, auth AuthRef, month calendar.Date) (int64, bool, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/care-billing-runs/")
	if err != nil {
		return 0, false, fmt.Errorf("read the monthly runs: %w", err)
	}
	var envelope struct {
		Data struct {
			Runs []struct {
				ID           int64         `json:"id"`
				BillingMonth calendar.Date `json:"billing_month"`
				Status       string        `json:"status"`
			} `json:"runs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, false, fmt.Errorf("read the monthly runs: %w", err)
	}
	for _, run := range envelope.Data.Runs {
		if run.BillingMonth == month && run.Status == "calculated" {
			return run.ID, true, nil
		}
	}
	return 0, false, nil
}
