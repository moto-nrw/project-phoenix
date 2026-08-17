package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Copy for the sick/excused notification.
//
// No child name, no class, no group name: Web Push renders Title and Body on a
// lock screen and keeps them in the notification centre, possibly on a shared
// group tablet. The deep link leads into the authenticated app, where the
// permission-filtered view shows who it is.
const (
	absenceReportedTitle           = "Krankmeldung"
	absenceReportedBodyParent      = "Eine Familie hat ein Kind aus Ihrer Gruppe für heute krankgemeldet."
	absenceReportedBodyStaff       = "Für ein Kind aus Ihrer Gruppe wurde heute eine Krankmeldung eingetragen."
	absenceReportedBodySickMany    = "%d Kinder wurden für heute krankgemeldet."
	absenceReportedBodyExcused     = "Für ein Kind aus Ihrer Gruppe wurde heute eine Entschuldigung eingetragen."
	absenceReportedBodyExcusedMany = "%d Kinder wurden für heute entschuldigt."
)

// absenceAggregateThreshold is the point at which one entry covering many
// children collapses into a single count instead of one notification each.
const absenceAggregateThreshold = 3

// AbsenceReport describes one submitted absence, whatever path recorded it.
type AbsenceReport struct {
	TenantID int64
	// StudentIDs are the children covered by this one submission.
	StudentIDs []int64
	// Status is active.StudentStatusDaySick or ...Excused. Anything else is
	// ignored: a class trip is not news, and a cleared status is not either.
	Status string
	// Dates the submission covers. The notification only fires when today is
	// among them — a note for next Tuesday is planning data, not an interrupt.
	Dates []timezone.Date
	// FromParent picks the wording. A family reporting is a different event
	// from a colleague entering it.
	FromParent bool
	// ActorAccountID is excluded from the recipients: nobody needs a push about
	// their own keystroke.
	ActorAccountID int64
	// ExcludedAccountIDs covers other people who originated the same report,
	// such as a guardian whose request was later approved by staff.
	ExcludedAccountIDs []int64
}

// AbsenceNotifier turns a recorded absence into a notification for the people
// responsible for that child.
type AbsenceNotifier interface {
	// NotifyAbsenceReported is fire-and-forget by contract: it is called from
	// after-commit hooks, and a failure must never roll back the absence that
	// was already recorded. Errors are logged, not returned.
	NotifyAbsenceReported(ctx context.Context, report AbsenceReport)
}

type absenceNotifier struct {
	notifier   Service
	recipients StaffRecipientResolver
	db         *bun.DB
	logger     *slog.Logger
}

// NewAbsenceNotifier builds the sick/excused producer.
func NewAbsenceNotifier(
	notifier Service,
	recipients StaffRecipientResolver,
	db *bun.DB,
	logger *slog.Logger,
) AbsenceNotifier {
	return &absenceNotifier{
		notifier:   notifier,
		recipients: recipients,
		db:         db,
		logger:     logger,
	}
}

func (n *absenceNotifier) getLogger() *slog.Logger {
	if n.logger == nil {
		return slog.Default()
	}
	return n.logger
}

func (n *absenceNotifier) NotifyAbsenceReported(ctx context.Context, report AbsenceReport) {
	if report.TenantID <= 0 {
		return
	}

	var err error
	if n.db == nil {
		err = n.notify(ctx, report)
	} else {
		// Every caller invokes the producer after the write transaction has
		// committed. Open a new tenant transaction so all recipient, consent and
		// delivery reads carry the PostgreSQL RLS context they require.
		err = tenant.WithTenantTx(ctx, n.db, report.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			return n.notify(txCtx, report)
		})
	}
	if err != nil {
		if errors.Is(err, ErrDisabled) || errors.Is(err, ErrOutsideActiveWindow) {
			return
		}
		n.getLogger().Warn("absence notification failed",
			slog.Int64("tenant_id", report.TenantID),
			slog.Int("student_count", len(report.StudentIDs)),
			slog.String("error", err.Error()),
		)
	}
}

func (n *absenceNotifier) notify(ctx context.Context, report AbsenceReport) error {
	if n.notifier == nil || n.recipients == nil {
		return nil
	}
	if report.TenantID <= 0 || len(report.StudentIDs) == 0 {
		return nil
	}
	switch report.Status {
	case activeModel.StudentStatusDaySick, activeModel.StudentStatusDayExcused:
	default:
		// A class trip is not news, and a cleared status even less so.
		return nil
	}
	if !containsToday(report.Dates) {
		return nil
	}

	deliveries, err := n.resolveDeliveries(ctx, report)
	if err != nil {
		return err
	}
	if len(deliveries) == 0 {
		return nil
	}

	events := make([]Event, 0, len(deliveries))
	for _, delivery := range deliveries {
		scopedReport := report
		scopedReport.StudentIDs = delivery.studentIDs
		events = append(events, Event{
			Type:     TypeStudentAbsenceReported,
			Priority: PriorityNormal,
			Title:    absenceReportedTitle,
			Body:     absenceBody(scopedReport),
			DeepLink: absenceDeepLink(scopedReport),
			Audience: Audience{
				TenantID:        report.TenantID,
				Scope:           ScopeStaff,
				StaffAccountIDs: delivery.accountIDs,
			},
		})
	}
	if batch, ok := n.notifier.(BatchNotifier); ok {
		return batch.NotifyBatch(ctx, events)
	}
	for _, event := range events {
		if err := n.notifier.Notify(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type absenceDelivery struct {
	studentIDs []int64
	accountIDs []int64
}

// resolveDeliveries builds each account's visible student subset from the
// supervision relation, adds the full submission for effective admins, then
// applies consent. Accounts with identical visibility share one event.
func (n *absenceNotifier) resolveDeliveries(ctx context.Context, report AbsenceReport) ([]absenceDelivery, error) {
	excluded := make([]int64, 0, len(report.ExcludedAccountIDs)+1)
	excluded = append(excluded, report.ExcludedAccountIDs...)
	excluded = append(excluded, report.ActorAccountID)
	scopes, err := n.recipients.Resolve(ctx, StaffRecipientRequest{
		StudentIDs:         report.StudentIDs,
		NotificationType:   TypeStudentAbsenceReported,
		ExcludedAccountIDs: excluded,
	})
	if err != nil {
		return nil, err
	}

	byScope := make(map[string]*absenceDelivery, len(scopes))
	keys := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		key := fmt.Sprint(scope.StudentIDs)
		delivery := byScope[key]
		if delivery == nil {
			delivery = &absenceDelivery{studentIDs: scope.StudentIDs}
			byScope[key] = delivery
			keys = append(keys, key)
		}
		delivery.accountIDs = append(delivery.accountIDs, scope.AccountID)
	}
	sort.Strings(keys)

	deliveries := make([]absenceDelivery, 0, len(keys))
	for _, key := range keys {
		delivery := byScope[key]
		sort.Slice(delivery.accountIDs, func(i, j int) bool {
			return delivery.accountIDs[i] < delivery.accountIDs[j]
		})
		deliveries = append(deliveries, *delivery)
	}
	return deliveries, nil
}

func absenceBody(report AbsenceReport) string {
	if len(report.StudentIDs) > 1 {
		if report.Status == activeModel.StudentStatusDayExcused {
			return fmt.Sprintf(absenceReportedBodyExcusedMany, len(report.StudentIDs))
		}
		return fmt.Sprintf(absenceReportedBodySickMany, len(report.StudentIDs))
	}
	if report.Status == activeModel.StudentStatusDayExcused {
		return absenceReportedBodyExcused
	}
	if report.FromParent {
		return absenceReportedBodyParent
	}
	return absenceReportedBodyStaff
}

// absenceDeepLink points at the one child when there is exactly one, and at the
// list otherwise. The ID is opaque and the page is permission-filtered.
func absenceDeepLink(report AbsenceReport) string {
	if len(report.StudentIDs) == 1 {
		return fmt.Sprintf("/students/%d", report.StudentIDs[0])
	}
	if len(report.StudentIDs) > absenceAggregateThreshold {
		return ""
	}
	return "/students"
}

func containsToday(dates []timezone.Date) bool {
	today := timezone.TodayDate()
	for _, d := range dates {
		if d == today {
			return true
		}
	}
	return false
}
