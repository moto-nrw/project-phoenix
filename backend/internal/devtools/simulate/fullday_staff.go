package simulate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// codeAlreadyCheckedIn is the registered error code of a check-in for a staff
// member whose work block is already open, e.g. through a supervision start.
const codeAlreadyCheckedIn = "workforce.already_checked_in"

// shiftLead is how early before the planned start a staff member stamps in.
// It stays inside the deviation tolerance, so no reason is asked for.
const shiftLead = 5 * time.Minute

// plannedShift is the part of a planned shift the clock-in needs.
type plannedShift struct {
	StaffID   int64  `json:"staff_id"`
	StartTime string `json:"start_time"`
	Cancelled bool   `json:"cancelled"`
}

type staffClockInAction struct{}

func (staffClockInAction) Name() string { return "clock in staff" }

// Run stamps in every staff member whose planned shift starts by now and who
// is not on approved leave today, with the staff member's own login. The
// seed plans today's shifts; who is already at work is left alone.
func (staffClockInAction) Run(_ context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 3b: Staff clock in at the start of their shift...")
	accounts := staffAccounts(rt.State.Accounts)
	if len(accounts) == 0 {
		fmt.Println("  Skipped: no staff logins in this profile")
		return nil
	}
	now := rt.Options.now().In(simulationBerlin)
	today := now.Format(isoDate)
	shifts, err := todaysShifts(rt, today)
	if err != nil {
		return err
	}
	for _, shift := range shifts {
		account, ok := accounts[shift.StaffID]
		if !ok || shift.Cancelled || rt.ClockedInStaff[shift.StaffID] || !shiftDue(shift, now) {
			continue
		}
		clocked, err := rt.clockIn(account)
		if err != nil {
			return err
		}
		rt.ClockedInStaff[shift.StaffID] = true
		if clocked {
			rt.Counts.StaffClockedIn++
		} else {
			rt.Counts.StaffAbsent++
		}
	}
	fmt.Printf("  %d staff clocked in, %d on leave\n", rt.Counts.StaffClockedIn, rt.Counts.StaffAbsent)
	return nil
}

func todaysShifts(rt *Runtime, today string) ([]plannedShift, error) {
	raw, err := rt.Client.Get(fmt.Sprintf("/api/staff-shifts?from=%s&to=%s", today, today))
	if err != nil {
		return nil, fmt.Errorf("list today's shifts: %w", err)
	}
	var envelope struct {
		Data []plannedShift `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode today's shifts: %w", err)
	}
	return envelope.Data, nil
}

// staffAccounts maps the staff ID of every seeded login to its credentials.
func staffAccounts(accounts SeedStateAccounts) map[int64]AccountCredentials {
	byStaff := make(map[int64]AccountCredentials)
	for _, account := range append(append([]AccountCredentials(nil), accounts.Admin...), accounts.Betreuer...) {
		if account.StaffID != 0 && account.Email != "" {
			byStaff[account.StaffID] = account
		}
	}
	return byStaff
}

// shiftDue reports whether the shift starts at most shiftLead after now.
func shiftDue(shift plannedShift, now time.Time) bool {
	start, err := time.ParseInLocation("15:04", shift.StartTime[:min(5, len(shift.StartTime))], simulationBerlin)
	if err != nil {
		return false
	}
	startAt := time.Date(now.Year(), now.Month(), now.Day(), start.Hour(), start.Minute(), 0, 0, simulationBerlin)
	return !startAt.After(now.Add(shiftLead))
}

// clockIn stamps one staff member in on a client of their own. It returns
// false without stamping when the staff member is on approved leave today.
func (rt *Runtime) clockIn(account AccountCredentials) (bool, error) {
	onLeave, err := rt.isOnLeave(account.StaffID)
	if err != nil || onLeave {
		return false, err
	}
	client, err := rt.staffClient(account)
	if err != nil {
		return false, err
	}
	_, err = client.Post("/api/time-tracking/check-in", map[string]any{"status": "present"})
	var apiErr interface{ HTTPErrorCode() string }
	if errors.As(err, &apiErr) && apiErr.HTTPErrorCode() == codeAlreadyCheckedIn {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("clock in staff %d: %w", account.StaffID, err)
	}
	return true, nil
}

// staffClient signs a staff member in on a new client. Every call signs in
// afresh: the clock may have moved past the last access token.
func (rt *Runtime) staffClient(account AccountCredentials) (Client, error) {
	client, err := buildClient(rt.Options.Client, rt.State.BaseURL, rt.Options.Verbose)
	if err != nil {
		return nil, err
	}
	if err := client.Login(account.Email, account.Password, rt.State.Bootstrap.TenantSlug); err != nil {
		return nil, fmt.Errorf("login staff %d: %w", account.StaffID, err)
	}
	return client, nil
}

// isOnLeave reports whether the staff member has an approved absence today
// (sick, vacation and the like). Staff without a seeded login count as
// present. The answer is remembered for the run.
func (rt *Runtime) isOnLeave(staffID int64) (bool, error) {
	if onLeave, checked := rt.StaffOnLeave[staffID]; checked {
		return onLeave, nil
	}
	account, ok := staffAccounts(rt.State.Accounts)[staffID]
	if !ok {
		return false, nil
	}
	client, err := rt.staffClient(account)
	if err != nil {
		return false, err
	}
	today := rt.Options.now().In(simulationBerlin).Format(isoDate)
	raw, err := client.Get(fmt.Sprintf("/api/time-tracking/absences?from=%s&to=%s&status=approved", today, today))
	if err != nil {
		return false, fmt.Errorf("list today's absences of staff %d: %w", staffID, err)
	}
	var absences struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &absences); err != nil {
		return false, fmt.Errorf("decode today's absences of staff %d: %w", staffID, err)
	}
	rt.StaffOnLeave[staffID] = len(absences.Data) > 0
	return rt.StaffOnLeave[staffID], nil
}

// availableCaregivers lists the caregivers who are not on leave today, in
// seed order.
func (rt *Runtime) availableCaregivers() ([]AccountCredentials, error) {
	available := make([]AccountCredentials, 0, len(rt.State.Accounts.Betreuer))
	for _, caregiver := range rt.State.Accounts.Betreuer {
		onLeave, err := rt.isOnLeave(caregiver.StaffID)
		if err != nil {
			return nil, err
		}
		if !onLeave {
			available = append(available, caregiver)
		}
	}
	return available, nil
}

// freeCaregiver returns the first caregiver at work who supervises no running
// session, 0 when there is none.
func (rt *Runtime) freeCaregiver() (int64, error) {
	available, err := rt.availableCaregivers()
	if err != nil {
		return 0, err
	}
	for _, caregiver := range available {
		if caregiver.StaffID != 0 && !slices.Contains(rt.SupervisorStaffIDs, caregiver.StaffID) {
			return caregiver.StaffID, nil
		}
	}
	return 0, nil
}
