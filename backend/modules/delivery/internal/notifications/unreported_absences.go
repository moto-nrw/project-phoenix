package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

// Copy of the notice about children missing without a report (#4791). It
// lands on a lock screen, so it names a number and never a child or a class.
const (
	unreportedAbsenceTitle   = "Fehlt ohne Meldung"
	unreportedAbsenceBodyOne = "1 Kind fehlt ohne Meldung"
	unreportedAbsenceBody    = "%d Kinder fehlen ohne Meldung"
	// unreportedAbsenceDeepLink opens the school-wide day overview.
	unreportedAbsenceDeepLink = "/school"
	// permissionClassDayAbsenceReportWrite restates the permission
	// registry's name, which this package may not import: the notice goes to
	// whoever may act on it (#4791). TestUnreportedAbsenceNoticeFollowsTheRight
	// pins it to the registry.
	permissionClassDayAbsenceReportWrite = "class_day:absence_report_write"
)

// UnreportedAbsenceReport and UnreportedAbsenceNotifier are the contract's.
type (
	UnreportedAbsenceReport   = contract.UnreportedAbsenceReport
	UnreportedAbsenceNotifier = contract.UnreportedAbsenceNotifier
)

var _ UnreportedAbsenceNotifier = (*absenceNotifier)(nil)

// NotifyUnreportedAbsences addresses the Schulverwaltung that did not switch
// the type off, minus the account whose mark caused it. The same children on
// the same day are one notice: the key carries the set.
// It runs in the caller's transaction, so the notice commits with the
// morning mark that caused it. Without an office it sends nothing.
func (n *absenceNotifier) NotifyUnreportedAbsences(ctx context.Context, report UnreportedAbsenceReport) error {
	studentIDs := uniqueInt64s(report.StudentIDs)
	if report.TenantID <= 0 || len(studentIDs) == 0 || n.school == nil || n.school.Office == nil {
		return nil
	}
	office, err := n.school.Office.SchoolOfficeAccountIDs(ctx)
	if err != nil {
		return fmt.Errorf("list school office accounts: %w", err)
	}
	candidates := slices.DeleteFunc(uniqueInt64s(office), func(id int64) bool { return id == report.ActorAccountID })
	if len(candidates) == 0 {
		return nil
	}
	recipients, err := n.school.Consent.FilterNotOptedOut(ctx, TypeSchoolUnreportedAbsence, candidates)
	if err != nil {
		return fmt.Errorf("filter school office consent: %w", err)
	}
	if len(recipients) == 0 {
		return nil
	}
	slices.Sort(recipients)
	err = n.notifier.Notify(ctx, unreportedAbsenceEvent(report, studentIDs, recipients))
	if errors.Is(err, ErrDisabled) || errors.Is(err, ErrOutsideActiveWindow) {
		return nil
	}
	if err != nil {
		n.getLogger().Warn("unreported absence notification failed",
			slog.Int64("tenant_id", report.TenantID),
			slog.Int("student_count", len(studentIDs)),
			slog.String("error", err.Error()),
		)
	}
	return err
}

func unreportedAbsenceEvent(report UnreportedAbsenceReport, studentIDs, recipients []int64) Event {
	body := unreportedAbsenceBodyOne
	if len(studentIDs) != 1 {
		body = fmt.Sprintf(unreportedAbsenceBody, len(studentIDs))
	}
	slices.Sort(studentIDs)
	keys := make([]string, 0, len(studentIDs))
	for _, id := range studentIDs {
		keys = append(keys, strconv.FormatInt(id, 10))
	}
	return Event{
		Type:           TypeSchoolUnreportedAbsence,
		IdempotencyKey: "school-unreported-" + report.Date.String() + "-" + strings.Join(keys, "."),
		RelatedType:    "school_day",
		Priority:       PriorityNormal,
		Title:          unreportedAbsenceTitle,
		Body:           body,
		SchoolDeepLink: unreportedAbsenceDeepLink,
		Audience: Audience{
			TenantID:        report.TenantID,
			Scope:           ScopeStaff,
			StaffAccountIDs: recipients,
		},
	}
}
