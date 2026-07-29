package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
)

// Copy for the sick/excused notification.
//
// No child name, no class, no group name: Web Push renders Title and Body on a
// lock screen and keeps them in the notification centre, possibly on a shared
// group tablet. The deep link leads into the authenticated app, where the
// permission-filtered view shows who it is.
const (
	absenceReportedTitle       = "Krankmeldung"
	absenceReportedBodyParent  = "Eine Familie hat ein Kind aus Ihrer Gruppe für heute krankgemeldet."
	absenceReportedBodyStaff   = "Für ein Kind aus Ihrer Gruppe wurde heute eine Krankmeldung eingetragen."
	absenceReportedBodyMany    = "%d Kinder wurden für heute krankgemeldet."
	absenceReportedBodyExcused = "Für ein Kind aus Ihrer Gruppe wurde heute eine Entschuldigung eingetragen."
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
	notifier    Service
	preferences PreferenceService
	students    userModel.StudentRepository
	groups      educationModel.GroupRepository
	staff       userModel.StaffRepository
	accounts    authAccountReader
	logger      *slog.Logger
}

// authAccountReader is the slice of the account repository this producer needs.
type authAccountReader interface {
	ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
}

// NewAbsenceNotifier builds the sick/excused producer.
func NewAbsenceNotifier(
	notifier Service,
	preferences PreferenceService,
	students userModel.StudentRepository,
	groups educationModel.GroupRepository,
	staff userModel.StaffRepository,
	accounts authAccountReader,
	logger *slog.Logger,
) AbsenceNotifier {
	return &absenceNotifier{
		notifier:    notifier,
		preferences: preferences,
		students:    students,
		groups:      groups,
		staff:       staff,
		accounts:    accounts,
		logger:      logger,
	}
}

func (n *absenceNotifier) getLogger() *slog.Logger {
	if n.logger == nil {
		return slog.Default()
	}
	return n.logger
}

func (n *absenceNotifier) NotifyAbsenceReported(ctx context.Context, report AbsenceReport) {
	if err := n.notify(ctx, report); err != nil {
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
	if n.notifier == nil || n.preferences == nil {
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

	recipients, err := n.resolveRecipients(ctx, report)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return nil
	}

	event := Event{
		Type:     TypeStudentAbsenceReported,
		Priority: PriorityNormal,
		Title:    absenceReportedTitle,
		Body:     absenceBody(report),
		DeepLink: absenceDeepLink(report),
		Audience: Audience{
			TenantID:        report.TenantID,
			Scope:           ScopeStaff,
			StaffAccountIDs: recipients,
		},
	}
	return n.notifier.Notify(ctx, event)
}

// resolveRecipients builds the candidate set from the relation and then applies
// consent. The order matters: consent narrows a relation-derived set, it never
// stands in for one.
func (n *absenceNotifier) resolveRecipients(ctx context.Context, report AbsenceReport) ([]int64, error) {
	candidates := make(map[int64]struct{})

	groupIDs, err := n.groupIDsOfStudents(ctx, report.StudentIDs)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) > 0 {
		pairs, perr := n.groups.ListStaffIDsByEducationGroupIDs(ctx, groupIDs, timezone.TodayDate())
		if perr != nil {
			return nil, fmt.Errorf("resolve supervising staff: %w", perr)
		}
		staffIDs := make([]int64, 0, len(pairs))
		for _, pair := range pairs {
			staffIDs = append(staffIDs, pair.StaffID)
		}
		accountsByStaff, aerr := n.staff.ListAccountIDsByStaffIDs(ctx, staffIDs)
		if aerr != nil {
			return nil, fmt.Errorf("resolve staff accounts: %w", aerr)
		}
		for _, accountID := range accountsByStaff {
			candidates[accountID] = struct{}{}
		}
	}

	// The office owns attendance bookkeeping and parent contact, so it hears
	// about an absence regardless of which group the child is in. This is also
	// what covers a child with no group at all.
	adminIDs, err := n.accounts.ListEffectiveAdminAccountIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve admin accounts: %w", err)
	}
	for _, accountID := range adminIDs {
		candidates[accountID] = struct{}{}
	}

	delete(candidates, report.ActorAccountID)
	if len(candidates) == 0 {
		return nil, nil
	}

	candidateIDs := make([]int64, 0, len(candidates))
	for accountID := range candidates {
		candidateIDs = append(candidateIDs, accountID)
	}

	optedIn, err := n.preferences.FilterOptedIn(ctx, TypeStudentAbsenceReported, candidateIDs)
	if err != nil {
		return nil, err
	}
	return optedIn, nil
}

// groupIDsOfStudents collects the education groups of the reported children.
func (n *absenceNotifier) groupIDsOfStudents(ctx context.Context, studentIDs []int64) ([]int64, error) {
	if n.students == nil {
		return nil, nil
	}
	students, err := n.students.FindReadScopeByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("load reported students: %w", err)
	}
	seen := make(map[int64]struct{}, len(students))
	groupIDs := make([]int64, 0, len(students))
	for _, student := range students {
		if student == nil || student.GroupID == nil {
			continue
		}
		if _, dup := seen[*student.GroupID]; dup {
			continue
		}
		seen[*student.GroupID] = struct{}{}
		groupIDs = append(groupIDs, *student.GroupID)
	}
	return groupIDs, nil
}

func absenceBody(report AbsenceReport) string {
	if len(report.StudentIDs) > absenceAggregateThreshold {
		return fmt.Sprintf(absenceReportedBodyMany, len(report.StudentIDs))
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
