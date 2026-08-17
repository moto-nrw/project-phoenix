package notifications

import (
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
)

// StaffRecipientRequest describes a child-related staff notification. The
// resolver builds recipients from responsibility first, then applies personal
// consent and the tenant's on-duty rule.
type StaffRecipientRequest struct {
	StudentIDs         []int64
	NotificationType   string
	ExcludedAccountIDs []int64
}

// StaffRecipientScope is one account and the requested children that account
// is responsible for. Keeping the child subset prevents aggregate copy from
// revealing children outside a group supervisor's responsibility.
type StaffRecipientScope struct {
	AccountID  int64
	StudentIDs []int64
}

// StaffRecipientResolver resolves the responsible staff accounts for child-
// related notifications.
type StaffRecipientResolver interface {
	Resolve(ctx context.Context, request StaffRecipientRequest) ([]StaffRecipientScope, error)
}

type staffRecipientResolver struct {
	preferences  PreferenceService
	students     userModel.StudentRepository
	groups       educationModel.GroupRepository
	staff        userModel.StaffRepository
	accounts     authAccountReader
	settings     settingsBoolReader
	workSessions dutyReader
}

// authAccountReader is the slice of the account repository this resolver needs.
type authAccountReader interface {
	ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
}

type settingsBoolReader interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type dutyReader interface {
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
}

// NewStaffRecipientResolver builds the shared responsibility resolver used by
// personal staff notifications about children.
func NewStaffRecipientResolver(
	preferences PreferenceService,
	students userModel.StudentRepository,
	groups educationModel.GroupRepository,
	staff userModel.StaffRepository,
	accounts authAccountReader,
	settings settingsBoolReader,
	workSessions dutyReader,
) StaffRecipientResolver {
	return &staffRecipientResolver{
		preferences:  preferences,
		students:     students,
		groups:       groups,
		staff:        staff,
		accounts:     accounts,
		settings:     settings,
		workSessions: workSessions,
	}
}

func (r *staffRecipientResolver) Resolve(ctx context.Context, request StaffRecipientRequest) ([]StaffRecipientScope, error) {
	if !r.configured() {
		return nil, nil
	}
	allStudentIDs := uniqueInt64s(request.StudentIDs)
	if len(allStudentIDs) == 0 {
		return nil, nil
	}

	visibleByAccount, err := r.resolveVisibility(ctx, allStudentIDs)
	if err != nil {
		return nil, err
	}
	for _, accountID := range request.ExcludedAccountIDs {
		delete(visibleByAccount, accountID)
	}
	if len(visibleByAccount) == 0 {
		return nil, nil
	}

	optedIn, err := r.preferences.FilterOptedIn(ctx, request.NotificationType, sortedAccountIDs(visibleByAccount))
	if err != nil {
		return nil, err
	}
	optedIn, err = r.filterOnDutyAccounts(ctx, optedIn)
	if err != nil {
		return nil, err
	}

	return staffRecipientScopes(visibleByAccount, optedIn), nil
}

func (r *staffRecipientResolver) configured() bool {
	return r.preferences != nil && r.students != nil && r.groups != nil && r.staff != nil &&
		r.accounts != nil && r.settings != nil && r.workSessions != nil
}

func (r *staffRecipientResolver) resolveVisibility(ctx context.Context, studentIDs []int64) (map[int64]map[int64]struct{}, error) {
	studentsByGroup, groupIDs, err := r.studentIDsByGroup(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	visibleByAccount := make(map[int64]map[int64]struct{})
	if err := r.addGroupStaff(ctx, visibleByAccount, studentsByGroup, groupIDs); err != nil {
		return nil, err
	}
	adminIDs, err := r.accounts.ListEffectiveAdminAccountIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve admin accounts: %w", err)
	}
	for _, accountID := range adminIDs {
		addVisibleStudents(visibleByAccount, accountID, studentIDs)
	}
	return visibleByAccount, nil
}

func (r *staffRecipientResolver) addGroupStaff(ctx context.Context, visibleByAccount map[int64]map[int64]struct{}, studentsByGroup map[int64][]int64, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	pairs, err := r.groups.ListStaffIDsByEducationGroupIDs(ctx, groupIDs, timezone.TodayDate())
	if err != nil {
		return fmt.Errorf("resolve supervising staff: %w", err)
	}
	staffIDs := make([]int64, 0, len(pairs))
	for _, pair := range pairs {
		staffIDs = append(staffIDs, pair.StaffID)
	}
	accountsByStaff, err := r.staff.ListAccountIDsByStaffIDs(ctx, staffIDs)
	if err != nil {
		return fmt.Errorf("resolve staff accounts: %w", err)
	}
	for _, pair := range pairs {
		if accountID := accountsByStaff[pair.StaffID]; accountID > 0 {
			addVisibleStudents(visibleByAccount, accountID, studentsByGroup[pair.GroupID])
		}
	}
	return nil
}

func (r *staffRecipientResolver) filterOnDutyAccounts(ctx context.Context, accountIDs []int64) ([]int64, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	onDutyOnly, err := r.settings.ResolveBool(ctx, configModel.KeyNotificationsOnDutyOnly)
	if err != nil {
		return nil, fmt.Errorf("resolve on-duty notification setting: %w", err)
	}
	if !onDutyOnly {
		return accountIDs, nil
	}

	onDutyAccounts, err := r.onDutyAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]int64, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, ok := onDutyAccounts[accountID]; ok {
			filtered = append(filtered, accountID)
		}
	}
	return filtered, nil
}

func (r *staffRecipientResolver) onDutyAccountIDs(ctx context.Context) (map[int64]struct{}, error) {
	presence, err := r.workSessions.GetTodayPresenceMap(ctx)
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
		return map[int64]struct{}{}, nil
	}
	accountsByStaff, err := r.staff.ListAccountIDsByStaffIDs(ctx, onDutyStaffIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve staff accounts for duty filter: %w", err)
	}
	onDutyAccounts := make(map[int64]struct{}, len(accountsByStaff))
	for _, staffID := range onDutyStaffIDs {
		if accountID := accountsByStaff[staffID]; accountID > 0 {
			onDutyAccounts[accountID] = struct{}{}
		}
	}
	return onDutyAccounts, nil
}

// studentIDsByGroup retains which requested children belong to each education
// group; flattening this relation would leak the total submission size across
// group boundaries.
func (r *staffRecipientResolver) studentIDsByGroup(ctx context.Context, studentIDs []int64) (map[int64][]int64, []int64, error) {
	students, err := r.students.FindReadScopeByIDs(ctx, studentIDs)
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

func sortedAccountIDs(visibleByAccount map[int64]map[int64]struct{}) []int64 {
	accountIDs := make([]int64, 0, len(visibleByAccount))
	for accountID := range visibleByAccount {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	return accountIDs
}

func staffRecipientScopes(visibleByAccount map[int64]map[int64]struct{}, accountIDs []int64) []StaffRecipientScope {
	scopes := make([]StaffRecipientScope, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		studentIDs := sortedInt64Set(visibleByAccount[accountID])
		if len(studentIDs) > 0 {
			scopes = append(scopes, StaffRecipientScope{AccountID: accountID, StudentIDs: studentIDs})
		}
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].AccountID < scopes[j].AccountID })
	return scopes
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
