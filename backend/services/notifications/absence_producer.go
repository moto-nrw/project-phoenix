package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
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
	notifier     Service
	preferences  PreferenceService
	students     userModel.StudentRepository
	groups       educationModel.GroupRepository
	staff        userModel.StaffRepository
	accounts     authAccountReader
	settings     settingsBoolReader
	workSessions dutyReader
	db           *bun.DB
	logger       *slog.Logger
}

// authAccountReader is the slice of the account repository this producer needs.
type authAccountReader interface {
	ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
}

type settingsBoolReader interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type dutyReader interface {
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
}

// NewAbsenceNotifier builds the sick/excused producer.
func NewAbsenceNotifier(
	notifier Service,
	preferences PreferenceService,
	students userModel.StudentRepository,
	groups educationModel.GroupRepository,
	staff userModel.StaffRepository,
	accounts authAccountReader,
	settings settingsBoolReader,
	workSessions dutyReader,
	db *bun.DB,
	logger *slog.Logger,
) AbsenceNotifier {
	return &absenceNotifier{
		notifier:     notifier,
		preferences:  preferences,
		students:     students,
		groups:       groups,
		staff:        staff,
		accounts:     accounts,
		settings:     settings,
		workSessions: workSessions,
		db:           db,
		logger:       logger,
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
	if n.notifier == nil || n.preferences == nil || n.settings == nil || n.workSessions == nil {
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
	allStudentIDs := uniqueInt64s(report.StudentIDs)
	studentsByGroup, groupIDs, err := n.studentIDsByGroup(ctx, allStudentIDs)
	if err != nil {
		return nil, err
	}
	visibleByAccount := make(map[int64]map[int64]struct{})
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
		for _, pair := range pairs {
			accountID, ok := accountsByStaff[pair.StaffID]
			if !ok {
				continue
			}
			addVisibleStudents(visibleByAccount, accountID, studentsByGroup[pair.GroupID])
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
		addVisibleStudents(visibleByAccount, accountID, allStudentIDs)
	}

	delete(visibleByAccount, report.ActorAccountID)
	for _, accountID := range report.ExcludedAccountIDs {
		delete(visibleByAccount, accountID)
	}
	if len(visibleByAccount) == 0 {
		return nil, nil
	}

	candidateIDs := make([]int64, 0, len(visibleByAccount))
	for accountID := range visibleByAccount {
		candidateIDs = append(candidateIDs, accountID)
	}
	sort.Slice(candidateIDs, func(i, j int) bool { return candidateIDs[i] < candidateIDs[j] })

	optedIn, err := n.preferences.FilterOptedIn(ctx, TypeStudentAbsenceReported, candidateIDs)
	if err != nil {
		return nil, err
	}
	optedIn, err = n.filterOnDutyAccounts(ctx, optedIn)
	if err != nil {
		return nil, err
	}

	byScope := make(map[string]*absenceDelivery, len(optedIn))
	keys := make([]string, 0, len(optedIn))
	for _, accountID := range optedIn {
		studentIDs := sortedInt64Set(visibleByAccount[accountID])
		if len(studentIDs) == 0 {
			continue
		}
		key := fmt.Sprint(studentIDs)
		delivery := byScope[key]
		if delivery == nil {
			delivery = &absenceDelivery{studentIDs: studentIDs}
			byScope[key] = delivery
			keys = append(keys, key)
		}
		delivery.accountIDs = append(delivery.accountIDs, accountID)
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

func (n *absenceNotifier) filterOnDutyAccounts(ctx context.Context, accountIDs []int64) ([]int64, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	onDutyOnly, err := n.settings.ResolveBool(ctx, configModel.KeyNotificationsOnDutyOnly)
	if err != nil {
		return nil, fmt.Errorf("resolve on-duty notification setting: %w", err)
	}
	if !onDutyOnly {
		return accountIDs, nil
	}

	presence, err := n.workSessions.GetTodayPresenceMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("load staff presence: %w", err)
	}
	onDutyStaffIDs := make([]int64, 0, len(presence))
	for staffID, status := range presence {
		if status == activeModel.WorkSessionStatusPresent || status == activeModel.WorkSessionStatusHomeOffice {
			onDutyStaffIDs = append(onDutyStaffIDs, staffID)
		}
	}
	if len(onDutyStaffIDs) == 0 {
		return nil, nil
	}
	accountsByStaff, err := n.staff.ListAccountIDsByStaffIDs(ctx, onDutyStaffIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve staff accounts for duty filter: %w", err)
	}
	onDutyAccounts := make(map[int64]struct{}, len(accountsByStaff))
	for _, staffID := range onDutyStaffIDs {
		if accountID := accountsByStaff[staffID]; accountID > 0 {
			onDutyAccounts[accountID] = struct{}{}
		}
	}

	filtered := make([]int64, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := onDutyAccounts[accountID]; ok {
			filtered = append(filtered, accountID)
		}
	}
	return filtered, nil
}

func addVisibleStudents(visibleByAccount map[int64]map[int64]struct{}, accountID int64, studentIDs []int64) {
	if accountID <= 0 || len(studentIDs) == 0 {
		return
	}
	if visibleByAccount[accountID] == nil {
		visibleByAccount[accountID] = make(map[int64]struct{}, len(studentIDs))
	}
	for _, studentID := range studentIDs {
		visibleByAccount[accountID][studentID] = struct{}{}
	}
}

func uniqueInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func sortedInt64Set(ids map[int64]struct{}) []int64 {
	sorted := make([]int64, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted
}

// studentIDsByGroup retains which reported children belong to each education
// group; flattening this relation would leak the total submission size across
// group boundaries.
func (n *absenceNotifier) studentIDsByGroup(ctx context.Context, studentIDs []int64) (map[int64][]int64, []int64, error) {
	if n.students == nil {
		return map[int64][]int64{}, nil, nil
	}
	students, err := n.students.FindReadScopeByIDs(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("load reported students: %w", err)
	}
	studentsByGroup := make(map[int64][]int64)
	seen := make(map[int64]struct{}, len(students))
	groupIDs := make([]int64, 0, len(students))
	for _, studentID := range studentIDs {
		student := students[studentID]
		if student == nil || student.GroupID == nil {
			continue
		}
		studentsByGroup[*student.GroupID] = append(studentsByGroup[*student.GroupID], studentID)
		if _, dup := seen[*student.GroupID]; dup {
			continue
		}
		seen[*student.GroupID] = struct{}{}
		groupIDs = append(groupIDs, *student.GroupID)
	}
	return studentsByGroup, groupIDs, nil
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
