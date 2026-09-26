package presenceservice

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
)

// PresenceOperations binds the presence application service to the
// operations port the active routes consume. Every method translates the
// session rows into the Student Presence owner's public values and passes the
// operation errors through unchanged, so the routes keep their exact error
// classification.
type PresenceOperations struct {
	active   presence.Service
	sessions studentpresence.SessionReads
	atSchool AtSchoolCounter
	logger   *slog.Logger
}

// AtSchoolCounter counts the students who read "Schule" right now (#3260).
// The day plan it needs lives outside the presence owner.
type AtSchoolCounter interface {
	CountAtSchoolToday(ctx context.Context, studentIDs []int64) (int, error)
}

// NewPresenceOperations wires a composed presence capability behind the
// presence operations contract. A nil atSchool leaves the dashboard's
// "Zuhause" figure unsplit.
func NewPresenceOperations(value studentpresence.Presence, atSchool AtSchoolCounter, logger *slog.Logger) (PresenceOperations, error) {
	service, ok := presenceEngineOf(value)
	if !ok {
		return PresenceOperations{}, errors.New("student presence compose: presence operations need a composed presence value")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return PresenceOperations{active: service, sessions: value, atSchool: atSchool, logger: logger}, nil
}

func (p PresenceOperations) StartSession(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	row := groupRow(group)
	if err := p.active.CreateActiveGroup(ctx, row); err != nil {
		return studentpresence.LiveGroup{}, err
	}
	return liveGroup(row), nil
}

func (p PresenceOperations) ReviseSession(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	row := groupRow(group)
	if err := p.active.UpdateActiveGroup(ctx, row); err != nil {
		return studentpresence.LiveGroup{}, err
	}
	return liveGroup(row), nil
}

func (p PresenceOperations) RemoveSession(ctx context.Context, id int64) error {
	return p.active.DeleteActiveGroup(ctx, id)
}

func (p PresenceOperations) EndSession(ctx context.Context, id int64) error {
	return p.active.EndActiveGroupSession(ctx, id)
}

func (p PresenceOperations) TouchSession(ctx context.Context, id int64) error {
	return p.active.UpdateSessionActivity(ctx, id)
}

func (p PresenceOperations) AdmitVisit(ctx context.Context, visit studentpresence.Visit) (studentpresence.Visit, error) {
	if err := p.active.CreateVisit(ctx, &visit); err != nil {
		return studentpresence.Visit{}, err
	}
	return visit, nil
}

func (p PresenceOperations) AmendVisit(ctx context.Context, visit studentpresence.Visit) error {
	return p.active.UpdateVisit(ctx, &visit)
}

func (p PresenceOperations) RemoveVisit(ctx context.Context, id int64) error {
	return p.active.DeleteVisit(ctx, id)
}

func (p PresenceOperations) EndVisit(ctx context.Context, id int64) error {
	return p.active.EndVisit(ctx, id)
}

func (p PresenceOperations) PresenceMode(ctx context.Context) (string, error) {
	return p.active.GetPresenceMode(ctx)
}

func (p PresenceOperations) AssignSupervision(ctx context.Context, row studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error) {
	supervisor, err := supervisorRow(row)
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	if err := p.active.CreateGroupSupervisor(ctx, supervisor); err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	return groupSupervision(supervisor), nil
}

func (p PresenceOperations) AmendSupervision(ctx context.Context, row studentpresence.GroupSupervision) error {
	supervisor, err := supervisorRow(row)
	if err != nil {
		return err
	}
	return p.active.UpdateGroupSupervisor(ctx, supervisor)
}

func (p PresenceOperations) RemoveSupervisionRecord(ctx context.Context, id int64) error {
	return p.active.DeleteGroupSupervisor(ctx, id)
}

func (p PresenceOperations) EndSupervision(ctx context.Context, id int64) error {
	return p.active.EndSupervision(ctx, id)
}

func (p PresenceOperations) ClaimSupervision(ctx context.Context, groupID, staffID int64, role string) (studentpresence.ClaimedSupervision, error) {
	row, err := p.active.ClaimActiveGroup(ctx, groupID, staffID, role)
	if err != nil {
		return studentpresence.ClaimedSupervision{}, err
	}
	return studentpresence.ClaimedSupervision{
		ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role, StartDate: row.StartDate.String(),
	}, nil
}

func (p PresenceOperations) UnclaimedSessions(ctx context.Context) ([]studentpresence.UnclaimedSession, error) {
	rows, err := p.active.GetUnclaimedActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.UnclaimedSession, 0, len(rows))
	for _, row := range rows {
		session := studentpresence.UnclaimedSession{UnclaimedGroup: studentpresence.UnclaimedGroup{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime,
			TimeoutMinutes: row.TimeoutMinutes, GroupID: row.GroupID, DeviceID: row.DeviceID, RoomID: row.RoomID,
		}}
		if row.Room != nil {
			session.Room = &studentpresence.SessionRoomSummary{ID: row.Room.ID, Name: row.Room.Name, Category: row.Room.Category, Color: row.Room.Color}
		}
		if row.ActualGroup != nil {
			session.Activity = &studentpresence.SessionActivitySummary{ID: row.ActualGroup.ID, Name: row.ActualGroup.Name}
		}
		result = append(result, session)
	}
	return result, nil
}

func (p PresenceOperations) CreateCombination(ctx context.Context, group studentpresence.CombinedGroup, groupIDs []int64) (studentpresence.CombinedGroup, error) {
	if err := p.active.CreateCombinedGroupWithGroups(ctx, &group, groupIDs); err != nil {
		return studentpresence.CombinedGroup{}, err
	}
	return group, nil
}

func (p PresenceOperations) AmendCombination(ctx context.Context, group studentpresence.CombinedGroup) (studentpresence.CombinedGroup, error) {
	if err := p.active.UpdateCombinedGroup(ctx, &group); err != nil {
		return studentpresence.CombinedGroup{}, err
	}
	return group, nil
}

func (p PresenceOperations) RemoveCombination(ctx context.Context, id int64) error {
	return p.active.DeleteCombinedGroup(ctx, id)
}

func (p PresenceOperations) CloseCombination(ctx context.Context, id int64) error {
	return p.active.EndCombinedGroup(ctx, id)
}

func attendanceStatus(status *presence.AttendanceStatus) *studentpresence.AttendanceStatus {
	if status == nil {
		return nil
	}
	return &studentpresence.AttendanceStatus{
		StudentID: status.StudentID, Status: status.Status, Date: status.Date.String(),
		CheckInTime: status.CheckInTime, CheckOutTime: status.CheckOutTime, YardSince: status.YardSince,
		CheckedInBy: status.CheckedInBy, CheckedOutBy: status.CheckedOutBy,
	}
}

func (p PresenceOperations) StudentAttendanceStatus(ctx context.Context, studentID int64) (*studentpresence.AttendanceStatus, error) {
	status, err := p.active.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return attendanceStatus(status), nil
}

func (p PresenceOperations) StudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.AttendanceStatus, error) {
	statuses, err := p.active.GetStudentsAttendanceStatuses(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*studentpresence.AttendanceStatus, len(statuses))
	for id, status := range statuses {
		result[id] = attendanceStatus(status)
	}
	return result, nil
}

func (p PresenceOperations) CheckOutStudent(ctx context.Context, studentID, staffID int64) (studentpresence.CheckoutOutcome, error) {
	result, err := p.active.CheckOutStudent(ctx, studentID, staffID, true)
	if err != nil {
		return studentpresence.CheckoutOutcome{}, err
	}
	return studentpresence.CheckoutOutcome{Action: result.Action, AttendanceID: result.AttendanceID}, nil
}

func studentMoveResult(result *presence.StudentMoveResult) studentpresence.StudentMoveResult {
	skipped := make([]studentpresence.StudentMoveSkipped, 0, len(result.Skipped))
	for _, item := range result.Skipped {
		skipped = append(skipped, studentpresence.StudentMoveSkipped{StudentID: item.StudentID, Reason: item.Reason})
	}
	return studentpresence.StudentMoveResult{
		Moved: result.Moved, Unchanged: result.Unchanged, Skipped: skipped,
		ActiveGroupID: result.ActiveGroupID, RoomID: result.RoomID,
	}
}

func (p PresenceOperations) AssignTransitStudents(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
	result, err := p.active.AssignTransitStudentsToActiveGroupAuthorized(ctx, studentIDs, activeGroupID, auth)
	if err != nil {
		return studentpresence.TransitAssignResult{}, err
	}
	skipped := make([]studentpresence.TransitAssignSkipped, 0, len(result.Skipped))
	for _, item := range result.Skipped {
		skipped = append(skipped, studentpresence.TransitAssignSkipped{StudentID: item.StudentID, Reason: item.Reason})
	}
	return studentpresence.TransitAssignResult{
		Assigned: result.Assigned, Skipped: skipped, ActiveGroupID: result.ActiveGroupID, RoomID: result.RoomID,
	}, nil
}

func (p PresenceOperations) MoveStudentsToSession(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
	result, err := p.active.MoveStudentsToActiveGroupAuthorized(ctx, studentIDs, activeGroupID, auth)
	if err != nil {
		return studentpresence.StudentMoveResult{}, err
	}
	return studentMoveResult(result), nil
}

func (p PresenceOperations) MoveStudentsToTransit(ctx context.Context, studentIDs []int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
	result, err := p.active.MoveStudentsToTransitAuthorized(ctx, studentIDs, auth)
	if err != nil {
		return studentpresence.StudentMoveResult{}, err
	}
	return studentMoveResult(result), nil
}

func (p PresenceOperations) SessionVisitsWithDisplay(ctx context.Context, groupID int64) ([]studentpresence.VisitDisplay, error) {
	rows, err := p.active.GetActiveGroupVisitsWithDisplay(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.VisitDisplay, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.VisitDisplay{
			VisitID: row.VisitID, StudentID: row.StudentID, ActiveGroupID: row.ActiveGroupID,
			EntryTime: row.EntryTime, ExitTime: row.ExitTime,
			FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass, OGSGroupName: row.OGSGroupName,
			Sick: row.Sick, SickSince: row.SickSince, Excused: row.Excused, ExcusedSince: row.ExcusedSince,
			PhotoPath: row.PhotoPath, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return result, nil
}

func (p PresenceOperations) SessionRooms(ctx context.Context, ids []int64) ([]studentpresence.SessionRoomSummary, error) {
	rooms, err := p.sessions.GetRoomsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.SessionRoomSummary, 0, len(rooms))
	for _, room := range rooms {
		if room == nil {
			continue
		}
		result = append(result, studentpresence.SessionRoomSummary{ID: room.ID, Name: room.Name, Category: room.Category, Color: room.Color})
	}
	return result, nil
}

func (p PresenceOperations) DashboardAnalytics(ctx context.Context) (studentpresence.DashboardAnalytics, error) {
	analytics, err := p.active.GetDashboardAnalytics(ctx)
	if err != nil {
		return studentpresence.DashboardAnalytics{}, err
	}
	result := studentpresence.DashboardAnalytics{
		StudentsPresent: analytics.StudentsPresent, StudentsInTransit: analytics.StudentsInTransit,
		StudentsOnPlayground: analytics.StudentsOnPlayground, StudentsInRooms: analytics.StudentsInRooms,
		StudentsSick: analytics.StudentsSick, StudentsExcused: analytics.StudentsExcused, StudentsHome: analytics.StudentsHome,
		ActiveActivities: analytics.ActiveActivities, FreeRooms: analytics.FreeRooms, TotalRooms: analytics.TotalRooms,
		CapacityUtilization: analytics.CapacityUtilization, ActivityCategories: analytics.ActivityCategories,
		ActiveOGSGroups: analytics.ActiveOGSGroups, StudentsInGroupRooms: analytics.StudentsInGroupRooms,
		SupervisorsToday: analytics.SupervisorsToday, StudentsInHomeRoom: analytics.StudentsInHomeRoom,
		RecentActivity:      make([]studentpresence.RecentActivity, 0, len(analytics.RecentActivity)),
		CurrentActivities:   make([]studentpresence.CurrentActivity, 0, len(analytics.CurrentActivities)),
		ActiveGroupsSummary: make([]studentpresence.ActiveGroupInfo, 0, len(analytics.ActiveGroupsSummary)),
		LastUpdated:         analytics.LastUpdated,
	}
	result.StudentsAtSchool, result.StudentsHome = splitAtSchoolOrKeepHome(ctx, p.atSchool, analytics.StudentsHome, analytics.HomeCandidateIDs, p.logger)
	for _, item := range analytics.RecentActivity {
		result.RecentActivity = append(result.RecentActivity, studentpresence.RecentActivity{
			Type: item.Type, GroupName: item.GroupName, RoomName: item.RoomName, Count: item.Count, Timestamp: item.Timestamp,
		})
	}
	for _, item := range analytics.CurrentActivities {
		result.CurrentActivities = append(result.CurrentActivities, studentpresence.CurrentActivity{
			ID: item.ID, Name: item.Name, Category: item.Category, Participants: item.Participants, MaxCapacity: item.MaxCapacity, Status: item.Status,
		})
	}
	for _, item := range analytics.ActiveGroupsSummary {
		result.ActiveGroupsSummary = append(result.ActiveGroupsSummary, studentpresence.ActiveGroupInfo{
			Name: item.Name, Type: item.Type, StudentCount: item.StudentCount, MaxCapacity: item.MaxCapacity, Location: item.Location, Status: item.Status,
		})
	}
	return result, nil
}

func splitAtSchoolOrKeepHome(ctx context.Context, counter AtSchoolCounter, home int, candidates []int64, logger *slog.Logger) (int, int) {
	atSchool, remainingHome, err := splitAtSchoolFromHome(ctx, counter, home, candidates)
	if err == nil {
		return atSchool, remainingHome
	}
	logger.Warn("failed to split at-school students from dashboard home count", "error", err)
	return 0, home
}

// splitAtSchoolFromHome moves the "Zuhause" candidates still in class into
// their own figure (#3260). Without a counter or candidates the home figure
// stays as it is; the result never drops below zero.
func splitAtSchoolFromHome(ctx context.Context, counter AtSchoolCounter, home int, candidates []int64) (int, int, error) {
	if counter == nil || len(candidates) == 0 {
		return 0, home, nil
	}
	atSchool, err := counter.CountAtSchoolToday(ctx, candidates)
	if err != nil {
		return 0, 0, err
	}
	return atSchool, max(0, home-atSchool), nil
}

func (p PresenceOperations) CrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]studentpresence.CrossTenantStudent, error) {
	rows, err := p.active.GetCrossTenantStudents(ctx, hostingTenantID)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.CrossTenantStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.CrossTenantStudent{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, GroupName: row.GroupName, HomeTenant: row.HomeTenant,
		})
	}
	return result, nil
}

func (p PresenceOperations) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	return p.active.GetTrackingIndicators(ctx, studentIDs, labels)
}
