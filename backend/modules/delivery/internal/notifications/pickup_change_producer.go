package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Copy for the changed pickup time (#6855).
//
// No child name: Web Push renders Title and Body on a lock screen, possibly on
// a shared group tablet. The day and the new time are no personal data on
// their own; the deep link opens the child, where the permission-filtered
// page names it and marks the changed day.
const (
	pickupChangedTitle       = "Abholzeit geändert"
	pickupChangedBody        = "Eltern haben die Abholzeit für ein Kind aus Ihrer Gruppe geändert. Neu für %s: %s Uhr."
	pickupWithdrawnTitle     = "Abholzeit zurückgenommen"
	pickupWithdrawnBody      = "Die geänderte Abholzeit für ein Kind aus Ihrer Gruppe gilt nicht mehr. Für %s gilt wieder die normale Abholzeit."
	pickupChangedDayToday    = "heute"
	pickupChangedDayTomorrow = "morgen"
)

var germanWeekdayShort = [...]string{"So.", "Mo.", "Di.", "Mi.", "Do.", "Fr.", "Sa."}

// PickupChangeReport and PickupChangeNotifier are the contract's.
type (
	PickupChangeReport   = contract.PickupChangeReport
	PickupChangeNotifier = contract.PickupChangeNotifier
)

var _ PickupChangeNotifier = (*absenceNotifier)(nil)

func (n *absenceNotifier) todayDate() calendar.Date {
	if n.today == nil {
		return calendar.TodayDate()
	}
	return n.today()
}

// NotifyPickupChanged tells the staff responsible for the child that its
// pickup time of one day changed or no longer applies. It runs in the
// caller's transaction, so the durable push intent commits with the change.
func (n *absenceNotifier) NotifyPickupChanged(ctx context.Context, report PickupChangeReport) error {
	if n.notifier == nil || n.recipients == nil || report.TenantID <= 0 || report.StudentID <= 0 {
		return nil
	}
	today := n.todayDate()
	// A past day is history, not news.
	if report.Date == "" || report.Date.Before(today) {
		return nil
	}
	if !report.Withdrawn && report.PickupTime == "" {
		return nil
	}
	err := n.notifyPickupChange(ctx, report, today)
	if errors.Is(err, ErrDisabled) || errors.Is(err, ErrOutsideActiveWindow) {
		return nil
	}
	if err != nil {
		n.getLogger().Warn("pickup change notification failed",
			slog.Int64("tenant_id", report.TenantID),
			slog.Int64("student_id", report.StudentID),
			slog.String("error", err.Error()),
		)
	}
	return err
}

func (n *absenceNotifier) notifyPickupChange(ctx context.Context, report PickupChangeReport, today calendar.Date) error {
	scopes, err := n.recipients.Resolve(ctx, StaffRecipientRequest{
		StudentIDs:         []int64{report.StudentID},
		NotificationType:   TypeStudentPickupChanged,
		ExcludedAccountIDs: []int64{report.ActorAccountID},
	})
	if err != nil {
		return err
	}
	accountIDs := make([]int64, 0, len(scopes))
	for _, scope := range scopes {
		accountIDs = append(accountIDs, scope.AccountID)
	}
	if len(accountIDs) == 0 {
		return nil
	}
	title, body := pickupChangeCopy(report, today)
	return n.notifier.Notify(ctx, Event{
		Type: TypeStudentPickupChanged,
		IdempotencyKey: fmt.Sprintf("student-pickup-changed:%d:%d:%s:%t:%s:%d",
			report.TenantID, report.StudentID, report.Date, report.Withdrawn, report.PickupTime, report.Revision),
		RelatedType: "student_pickup_exception",
		RelatedID:   report.StudentID,
		Priority:    PriorityNormal,
		Title:       title,
		Body:        body,
		DeepLink:    fmt.Sprintf("/students/%d", report.StudentID),
		Audience: Audience{
			TenantID:        report.TenantID,
			Scope:           ScopeStaff,
			StaffAccountIDs: accountIDs,
		},
	})
}

// pickupChangeCopy names the day as "heute", "morgen" or "Fr. 13.11." and
// the new time, or that the regular time applies again.
func pickupChangeCopy(report PickupChangeReport, today calendar.Date) (string, string) {
	day := pickupChangeDay(report.Date, today)
	if report.Withdrawn {
		return pickupWithdrawnTitle, fmt.Sprintf(pickupWithdrawnBody, day)
	}
	return pickupChangedTitle, fmt.Sprintf(pickupChangedBody, day, report.PickupTime)
}

func pickupChangeDay(date, today calendar.Date) string {
	switch date {
	case today:
		return pickupChangedDayToday
	case today.AddDays(1):
		return pickupChangedDayTomorrow
	}
	return germanWeekdayShort[date.Weekday()] + " " + date.Format("02.01.")
}
