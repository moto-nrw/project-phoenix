package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
)

// demoPersonnelNumberStart is the first invented Personalnummer. The demo
// numbers count up from it; nothing real is behind them.
const demoPersonnelNumberStart = 70001

// seedPayrollNumbersStep gives every staff member of the school a
// Personalnummer (#5032). The payroll status counts all staff, the visitor,
// the admin and the external caregivers included, so the step reads the whole
// directory instead of the fixed demo staff. It runs after the last step that
// adds staff to the school and before the profile check, then proves through
// the payroll status that the Abrechnung page and the DATEV export are
// complete. Each write goes through the school's API and leaves its row in
// the audit of personnel number changes.
type seedPayrollNumbersStep struct{}

func (seedPayrollNumbersStep) Name() string { return "Seeding payroll personnel numbers" }

func (seedPayrollNumbersStep) Run(_ context.Context, rt *Runtime) error {
	staffIDs, err := listSeedStaffIDs(rt)
	if err != nil {
		return err
	}
	if len(staffIDs) == 0 {
		return fmt.Errorf("personnel numbers need staff")
	}
	used := make(map[string]bool, len(staffIDs))
	var missing []int64
	for _, id := range staffIDs {
		number, err := readSeedPersonnelNumber(rt, id)
		if err != nil {
			return err
		}
		if number == "" {
			missing = append(missing, id)
			continue
		}
		used[number] = true
	}
	next := demoPersonnelNumberStart
	for _, id := range missing {
		for used[strconv.Itoa(next)] {
			next++
		}
		number := strconv.Itoa(next)
		used[number] = true
		path := fmt.Sprintf("/api/staff/%d/payroll-number", id)
		if _, err := rt.Client.PutWithAuth(rt.TenantAuth, path, map[string]any{
			"personnel_number": number,
			"note":             "Personalnummer aus der Lohnabrechnung übernommen",
		}); err != nil {
			return fmt.Errorf("set personnel number of staff %d: %w", id, err)
		}
	}
	if err := verifySeedPayrollStatus(rt); err != nil {
		return err
	}
	fmt.Printf("  %d personnel numbers set\n", len(missing))
	return nil
}

func listSeedStaffIDs(rt *Runtime) ([]int64, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, "/api/staff/")
	if err != nil {
		return nil, fmt.Errorf("list staff for personnel numbers: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID any `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode staff for personnel numbers: %w", err)
	}
	ids := make([]int64, 0, len(envelope.Data))
	for _, staff := range envelope.Data {
		id, err := parseSeedID(staff.ID)
		if err != nil || id == 0 {
			return nil, fmt.Errorf("staff for personnel numbers has no id")
		}
		ids = append(ids, id)
	}
	// The directory sorts by name; the numbers follow the order of creation.
	slices.Sort(ids)
	return ids, nil
}

func readSeedPersonnelNumber(rt *Runtime, staffID int64) (string, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, fmt.Sprintf("/api/staff/%d/payroll-number", staffID))
	if err != nil {
		return "", fmt.Errorf("read personnel number of staff %d: %w", staffID, err)
	}
	var envelope struct {
		Data struct {
			PersonnelNumber *string `json:"personnel_number"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("decode personnel number of staff %d: %w", staffID, err)
	}
	if envelope.Data.PersonnelNumber == nil {
		return "", nil
	}
	return *envelope.Data.PersonnelNumber, nil
}

func verifySeedPayrollStatus(rt *Runtime) error {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, "/api/settings/payroll-status")
	if err != nil {
		return fmt.Errorf("read payroll status: %w", err)
	}
	var envelope struct {
		Data struct {
			LodasHeaderComplete         bool `json:"lodas_header_complete"`
			ConfiguredCategories        int  `json:"configured_categories"`
			TotalCategories             int  `json:"total_categories"`
			StaffTotal                  int  `json:"staff_total"`
			StaffWithoutPersonnelNumber int  `json:"staff_without_personnel_number"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode payroll status: %w", err)
	}
	status := envelope.Data
	switch {
	case status.TotalCategories == 0 || status.ConfiguredCategories != status.TotalCategories:
		return fmt.Errorf("payroll status: %d of %d wage types configured", status.ConfiguredCategories, status.TotalCategories)
	case !status.LodasHeaderComplete:
		return fmt.Errorf("payroll status: DATEV client incomplete")
	case status.StaffTotal == 0 || status.StaffWithoutPersonnelNumber != 0:
		return fmt.Errorf("payroll status: %d of %d staff without personnel number", status.StaffWithoutPersonnelNumber, status.StaffTotal)
	}
	return nil
}
