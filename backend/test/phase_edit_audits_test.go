package test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestPhaseEditAudits runs the three read-only audits of #6505 against one
// contradictory and one consistent child per class, plus the legitimate
// deviations each audit must not report. Dates lie in 2000 or 2099, so the
// audits' Berlin "today" never reaches them.
func TestPhaseEditAudits(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	school := newPhaseAuditSchool(t, db)
	bookingLed := newPhaseAuditSchool(t, db)
	other := newPhaseAuditSchool(t, db)

	// An explicit false is no booking-led care either.
	school.exec(`INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'enrollment.bookings_authoritative', 'false'::jsonb)`, school.tenantID)
	activations := school.seedPendingActivations()
	switches := school.seedClassSwitches()
	care := bookingLed.seedBookingLedCare()
	// The same contradictory state in a school without booking-led care is
	// no finding, and nothing leaks between schools.
	other.student("1a", "active", "2000-01-01", "")

	assert.Equal(t, activations, phaseAuditFindings(t, db, "phase_edit_stale_pending_activations", "request_child_id", school.tenantID))
	assert.Equal(t, switches, phaseAuditFindings(t, db, "phase_edit_stale_class_switches", "request_child_id", school.tenantID))
	assert.Equal(t, care, phaseAuditFindings(t, db, "booking_led_care_without_source_or_task", "student_id", bookingLed.tenantID))
	assert.Empty(t, phaseAuditFindings(t, db, "booking_led_care_without_source_or_task", "student_id", school.tenantID))
	assert.Empty(t, phaseAuditFindings(t, db, "booking_led_care_without_source_or_task", "student_id", other.tenantID))
}

// seedPendingActivations arranges class a: the phase starts on 2099-08-10,
// its pending children were stamped on the old start 2099-08-03.
func (s phaseAuditSchool) seedPendingActivations() map[int64]string {
	phase := s.phase("2099-08-10", "2100-07-31")
	holiday := s.phase("2099-07-20", "2099-08-09")
	stale := s.student("1a", "pending", "2099-08-03", "2100-07-31")
	current := s.student("1a", "pending", "2099-08-10", "2100-07-31")
	manual := s.student("1a", "pending", "2099-09-01", "2100-07-31")
	twoPhases := s.student("1a", "pending", "2099-07-20", "2100-07-31")
	activated := s.student("1a", "active", "2099-08-03", "2100-07-31")
	staleChild := s.scheduledChild(phase, stale, "2099-08-03")
	s.scheduledChild(phase, current, "2099-08-10")
	manualChild := s.scheduledChild(phase, manual, "2099-08-03")
	twoPhaseChild := s.scheduledChild(phase, twoPhases, "2099-08-03")
	s.scheduledChild(holiday, twoPhases, "2099-07-20")
	s.scheduledChild(phase, activated, "2099-08-03")
	return map[int64]string{
		staleChild: "unambiguous", manualChild: "ambiguous_window_changed", twoPhaseChild: "ambiguous_several_approvals",
	}
}

// seedClassSwitches arranges class b: open plans of approved renewals of a
// phase that starts on 2099-08-10.
func (s phaseAuditSchool) seedClassSwitches() map[int64]string {
	phase := s.phase("2099-08-10", "2100-07-31")
	stale := s.immediateChild(phase, s.student("2a", "active", "2000-01-01", ""))
	current := s.immediateChild(phase, s.student("2a", "active", "2000-01-01", ""))
	changed := s.immediateChild(phase, s.student("2b", "active", "2000-01-01", ""))
	weekly := s.immediateChild(phase, s.student("2a", "active", "2000-01-01", ""))
	s.exec(`UPDATE enrollment.request_children SET class_switch_from = '2a', class_switch_to = '3a',
		class_switch_on = CASE WHEN id = ? THEN '2099-08-10'::date ELSE '2099-08-03'::date END,
		weekly_plan_on = '2099-08-10' WHERE tenant_id = ? AND id IN (?)`,
		current, s.tenantID, bun.List([]int64{stale, current, changed}))
	s.exec(`UPDATE enrollment.request_children SET weekly_plan_on = '2099-08-03' WHERE tenant_id = ? AND id = ?`, s.tenantID, weekly)
	return map[int64]string{stale: "unambiguous", changed: "no_effect", weekly: "weekly_plan_only"}
}

// seedBookingLedCare arranges class c in a booking-led school: children in
// care since 2000-01-01 with different care sources.
func (s phaseAuditSchool) seedBookingLedCare() map[int64]string {
	s.exec(`INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'enrollment.bookings_authoritative', 'true'::jsonb)`, s.tenantID)
	phase := s.phase("2000-01-01", "2100-07-31")
	care := s.offering(phase, true)
	extra := s.offering(phase, false)
	withoutBooking := s.student("1a", "active", "2000-01-01", "")
	booked := s.student("1a", "active", "2000-01-01", "")
	ended := s.student("1a", "active", "2000-01-01", "")
	later := s.student("1a", "active", "2000-01-01", "")
	extraOnly := s.student("1a", "active", "2000-01-01", "")
	withTask := s.student("1a", "active", "2000-01-01", "")
	// Activated ahead of its start: not yet in care, no finding.
	s.student("1a", "active", "2099-08-01", "")
	s.booking(s.immediateChild(phase, booked), care, nil, nil)
	s.booking(s.immediateChild(phase, ended), care, new("2000-01-03"), new("2000-02-01"))
	s.booking(s.immediateChild(phase, later), care, new("2099-08-01"), nil)
	s.booking(s.immediateChild(phase, extraOnly), extra, nil, nil)
	s.exec(`INSERT INTO users.care_withdrawal_completions
		(tenant_id, student_id, first_bookingless_day, trigger, withdrawal_confirmed_role, withdrawal_confirmed_at)
		VALUES (?, ?, '2000-02-01', 'booking_expired', 'system', NOW())`, s.tenantID, withTask)
	return map[int64]string{
		withoutBooking: "no_booking_at_all", ended: "booking_ended", later: "booking_starts_later", extraOnly: "no_booking_at_all",
	}
}

type phaseAuditSchool struct {
	t        *testing.T
	db       *bun.DB
	ctx      context.Context
	tenantID int64
}

func newPhaseAuditSchool(t *testing.T, db *bun.DB) phaseAuditSchool {
	t.Helper()
	scope := NewTenantScope(t, db)
	return phaseAuditSchool{t: t, db: db, ctx: scope.Context(), tenantID: scope.TenantID}
}

func (s phaseAuditSchool) exec(query string, args ...any) {
	s.t.Helper()
	_, err := s.db.NewRaw(query, args...).Exec(s.ctx)
	require.NoError(s.t, err)
}

func (s phaseAuditSchool) id(query string, args ...any) int64 {
	s.t.Helper()
	var id int64
	require.NoError(s.t, s.db.NewRaw(query, args...).Scan(s.ctx, &id))
	return id
}

func (s phaseAuditSchool) phase(start, end string) int64 {
	s.t.Helper()
	return s.id(`INSERT INTO enrollment.phases (tenant_id, name, kind, service_start_date, service_end_date, care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, ?, 'school_year', ?, ?, 'waitlist', 'optional', true) RETURNING id`,
		s.tenantID, fmt.Sprintf("Audit %d", uniqueFixtureSuffix()), start, end)
}

// student creates a child with the given membership status and window; an
// empty bound stays open.
func (s phaseAuditSchool) student(class, status, from, until string) int64 {
	s.t.Helper()
	student := CreateTestStudentForTenant(s.t, s.db, s.tenantID, "Audit", fmt.Sprintf("Kind %d", uniqueFixtureSuffix()), class)
	s.exec(`UPDATE users.student_school_memberships SET status = ?, enrolled_from = NULLIF(?, '')::date, enrolled_until = NULLIF(?, '')::date
		WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`, status, from, until, s.tenantID, student.ID)
	return student.ID
}

func (s phaseAuditSchool) child(phaseID, studentID int64, mode string, activateOn *string) int64 {
	s.t.Helper()
	suffix := uniqueFixtureSuffix()
	requestID := s.id(`INSERT INTO enrollment.requests (tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token)
		VALUES (?, ?, 'Anna', 'Audit', ?, ?) RETURNING id`,
		s.tenantID, phaseID, fmt.Sprintf("phase-audit-%d@example.test", suffix), fmt.Sprintf("phase-audit-%d", suffix))
	return s.id(`INSERT INTO enrollment.request_children (tenant_id, request_id, first_name, last_name, date_of_birth, status, activation_mode, activate_on, created_student_id)
		VALUES (?, ?, 'Audit', 'Kind', '2018-04-15', 'approved', ?, ?::date, ?) RETURNING id`,
		s.tenantID, requestID, mode, activateOn, studentID)
}

func (s phaseAuditSchool) scheduledChild(phaseID, studentID int64, activateOn string) int64 {
	return s.child(phaseID, studentID, "scheduled", &activateOn)
}

func (s phaseAuditSchool) immediateChild(phaseID, studentID int64) int64 {
	return s.child(phaseID, studentID, "immediate", nil)
}

func (s phaseAuditSchool) offering(phaseID int64, countsAsCare bool) int64 {
	s.t.Helper()
	return s.id(`INSERT INTO enrollment.care_offerings (tenant_id, phase_id, name, days_of_week_mode, available_days, counts_as_care)
		VALUES (?, ?, ?, 'fixed', '["mon","tue","wed","thu","fri"]'::jsonb, ?) RETURNING id`,
		s.tenantID, phaseID, fmt.Sprintf("Angebot %d", uniqueFixtureSuffix()), countsAsCare)
}

func (s phaseAuditSchool) booking(requestChildID, offeringID int64, from, until *string) {
	s.t.Helper()
	s.exec(`INSERT INTO enrollment.care_offering_bookings (tenant_id, request_child_id, care_offering_id, valid_from, valid_until)
		VALUES (?, ?, ?, ?::date, ?::date)`, s.tenantID, requestChildID, offeringID, from, until)
}

// phaseAuditFindings runs one audit file and maps the key column of the
// tenant's rows to their finding.
func phaseAuditFindings(t *testing.T, db *bun.DB, audit, key string, tenantID int64) map[int64]string {
	t.Helper()
	query, err := os.ReadFile("../database/audits/" + audit + ".sql")
	require.NoError(t, err)
	var rows []map[string]any
	require.NoError(t, db.NewRaw(string(query)).Scan(context.Background(), &rows))
	findings := map[int64]string{}
	for _, row := range rows {
		if row["school_id"].(int64) == tenantID {
			findings[row[key].(int64)] = row["finding"].(string)
		}
	}
	return findings
}
