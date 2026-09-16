package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
)

// presenceOperations binds the retained active service to the presence
// operations port the active routes consume. Every method translates the
// retained models into the Student Presence owner's public values and passes
// the retained operation errors through unchanged, so the routes keep their
// exact error classification.
type presenceOperations struct {
	active   active.Service
	atSchool AtSchoolCounter
}

// AtSchoolCounter counts the students who read "Schule" right now (#3260).
// The day plan it needs lives outside the presence owner.
type AtSchoolCounter interface {
	CountAtSchoolToday(ctx context.Context, studentIDs []int64) (int, error)
}

// NewPresenceOperations wires the retained active service behind the
// presence operations contract. A nil atSchool leaves the dashboard's
// "Zuhause" figure unsplit.
func NewPresenceOperations(service active.Service, atSchool AtSchoolCounter) presenceOperations {
	return presenceOperations{active: service, atSchool: atSchool}
}

func legacyGroup(group studentpresence.LiveGroup) *activeModels.Group {
	row := &activeModels.Group{
		StartTime:      group.StartTime,
		EndTime:        group.EndTime,
		LastActivity:   group.LastActivity,
		TimeoutMinutes: group.TimeoutMinutes,
		GroupID:        group.ActivityGroupID,
		DeviceID:       group.DeviceID,
		RoomID:         group.RoomID,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = group.ID, group.CreatedAt, group.UpdatedAt
	row.SetTenantID(group.TenantID)
	return row
}

func liveGroup(group *activeModels.Group) studentpresence.LiveGroup {
	return studentpresence.LiveGroup{
		ID: group.ID, TenantID: group.TenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		StartTime: group.StartTime, LastActivity: group.LastActivity, EndTime: group.EndTime,
		TimeoutMinutes: group.TimeoutMinutes, ActivityGroupID: group.GroupID, DeviceID: group.DeviceID, RoomID: group.RoomID,
	}
}

func (p presenceOperations) StartSession(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	row := legacyGroup(group)
	if err := p.active.CreateActiveGroup(ctx, row); err != nil {
		return studentpresence.LiveGroup{}, err
	}
	return liveGroup(row), nil
}

func (p presenceOperations) ReviseSession(ctx context.Context, group studentpresence.LiveGroup) (studentpresence.LiveGroup, error) {
	row := legacyGroup(group)
	if err := p.active.UpdateActiveGroup(ctx, row); err != nil {
		return studentpresence.LiveGroup{}, err
	}
	return liveGroup(row), nil
}

func (p presenceOperations) RemoveSession(ctx context.Context, id int64) error {
	return p.active.DeleteActiveGroup(ctx, id)
}

func (p presenceOperations) EndSession(ctx context.Context, id int64) error {
	return p.active.EndActiveGroupSession(ctx, id)
}

func (p presenceOperations) TouchSession(ctx context.Context, id int64) error {
	return p.active.UpdateSessionActivity(ctx, id)
}

func (p presenceOperations) AdmitVisit(ctx context.Context, visit studentpresence.Visit) (studentpresence.Visit, error) {
	if err := p.active.CreateVisit(ctx, &visit); err != nil {
		return studentpresence.Visit{}, err
	}
	return visit, nil
}

func (p presenceOperations) AmendVisit(ctx context.Context, visit studentpresence.Visit) error {
	return p.active.UpdateVisit(ctx, &visit)
}

func (p presenceOperations) RemoveVisit(ctx context.Context, id int64) error {
	return p.active.DeleteVisit(ctx, id)
}

func (p presenceOperations) EndVisit(ctx context.Context, id int64) error {
	return p.active.EndVisit(ctx, id)
}

func (p presenceOperations) PresenceMode(ctx context.Context) (string, error) {
	return p.active.GetPresenceMode(ctx)
}

func legacySupervision(row studentpresence.GroupSupervision) (*activeModels.GroupSupervisor, error) {
	start, err := timezone.ParseDate(row.StartDate)
	if err != nil {
		return nil, &active.ActiveError{Op: "ParseSupervisionDate", Err: studentpresence.ErrInvalidData}
	}
	supervisor := &activeModels.GroupSupervisor{
		StaffID:   row.StaffID,
		GroupID:   row.GroupID,
		Role:      row.Role,
		StartDate: start,
	}
	supervisor.ID, supervisor.CreatedAt, supervisor.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	supervisor.SetTenantID(row.TenantID)
	if row.EndDate != nil {
		end, err := timezone.ParseDate(*row.EndDate)
		if err != nil {
			return nil, &active.ActiveError{Op: "ParseSupervisionDate", Err: studentpresence.ErrInvalidData}
		}
		supervisor.EndDate = &end
	}
	return supervisor, nil
}

func groupSupervision(row *activeModels.GroupSupervisor) studentpresence.GroupSupervision {
	result := studentpresence.GroupSupervision{
		ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role, StartDate: row.StartDate.String(),
	}
	if row.EndDate != nil {
		end := row.EndDate.String()
		result.EndDate = &end
	}
	return result
}

func (p presenceOperations) AssignSupervision(ctx context.Context, row studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error) {
	supervisor, err := legacySupervision(row)
	if err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	if err := p.active.CreateGroupSupervisor(ctx, supervisor); err != nil {
		return studentpresence.GroupSupervision{}, err
	}
	return groupSupervision(supervisor), nil
}

func (p presenceOperations) AmendSupervision(ctx context.Context, row studentpresence.GroupSupervision) error {
	supervisor, err := legacySupervision(row)
	if err != nil {
		return err
	}
	return p.active.UpdateGroupSupervisor(ctx, supervisor)
}

func (p presenceOperations) RemoveSupervisionRecord(ctx context.Context, id int64) error {
	return p.active.DeleteGroupSupervisor(ctx, id)
}

func (p presenceOperations) EndSupervision(ctx context.Context, id int64) error {
	return p.active.EndSupervision(ctx, id)
}

func (p presenceOperations) ClaimSupervision(ctx context.Context, groupID, staffID int64, role string) (studentpresence.ClaimedSupervision, error) {
	row, err := p.active.ClaimActiveGroup(ctx, groupID, staffID, role)
	if err != nil {
		return studentpresence.ClaimedSupervision{}, err
	}
	return studentpresence.ClaimedSupervision{
		ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StaffID: row.StaffID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Role: row.Role, StartDate: row.StartDate.String(),
	}, nil
}

func (p presenceOperations) UnclaimedSessions(ctx context.Context) ([]studentpresence.UnclaimedSession, error) {
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

func (p presenceOperations) CreateCombination(ctx context.Context, group studentpresence.CombinedGroup, groupIDs []int64) (studentpresence.CombinedGroup, error) {
	if err := p.active.CreateCombinedGroupWithGroups(ctx, &group, groupIDs); err != nil {
		return studentpresence.CombinedGroup{}, err
	}
	return group, nil
}

func (p presenceOperations) AmendCombination(ctx context.Context, group studentpresence.CombinedGroup) (studentpresence.CombinedGroup, error) {
	if err := p.active.UpdateCombinedGroup(ctx, &group); err != nil {
		return studentpresence.CombinedGroup{}, err
	}
	return group, nil
}

func (p presenceOperations) RemoveCombination(ctx context.Context, id int64) error {
	return p.active.DeleteCombinedGroup(ctx, id)
}

func (p presenceOperations) CloseCombination(ctx context.Context, id int64) error {
	return p.active.EndCombinedGroup(ctx, id)
}

func attendanceStatus(status *active.AttendanceStatus) *studentpresence.AttendanceStatus {
	if status == nil {
		return nil
	}
	return &studentpresence.AttendanceStatus{
		StudentID: status.StudentID, Status: status.Status, Date: status.Date.String(),
		CheckInTime: status.CheckInTime, CheckOutTime: status.CheckOutTime, YardSince: status.YardSince,
		CheckedInBy: status.CheckedInBy, CheckedOutBy: status.CheckedOutBy,
	}
}

func (p presenceOperations) StudentAttendanceStatus(ctx context.Context, studentID int64) (*studentpresence.AttendanceStatus, error) {
	status, err := p.active.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return attendanceStatus(status), nil
}

func (p presenceOperations) StudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.AttendanceStatus, error) {
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

func (p presenceOperations) CheckOutStudent(ctx context.Context, studentID, staffID int64) (studentpresence.CheckoutOutcome, error) {
	result, err := p.active.CheckOutStudent(ctx, studentID, staffID, true)
	if err != nil {
		return studentpresence.CheckoutOutcome{}, err
	}
	return studentpresence.CheckoutOutcome{Action: result.Action, AttendanceID: result.AttendanceID}, nil
}

func legacyMoveAuthorization(auth studentpresence.StudentMoveAuthorization) active.StudentMoveAuthorization {
	return active.StudentMoveAuthorization{
		StaffID: auth.StaffID, BypassResourceChecks: auth.BypassResourceChecks,
		SchoolWideAttendanceEligible: auth.SchoolWideAttendanceEligible,
	}
}

func studentMoveResult(result *active.StudentMoveResult) studentpresence.StudentMoveResult {
	skipped := make([]studentpresence.StudentMoveSkipped, 0, len(result.Skipped))
	for _, item := range result.Skipped {
		skipped = append(skipped, studentpresence.StudentMoveSkipped{StudentID: item.StudentID, Reason: item.Reason})
	}
	return studentpresence.StudentMoveResult{
		Moved: result.Moved, Unchanged: result.Unchanged, Skipped: skipped,
		ActiveGroupID: result.ActiveGroupID, RoomID: result.RoomID,
	}
}

func (p presenceOperations) AssignTransitStudents(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
	result, err := p.active.AssignTransitStudentsToActiveGroupAuthorized(ctx, studentIDs, activeGroupID, legacyMoveAuthorization(auth))
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

func (p presenceOperations) MoveStudentsToSession(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
	result, err := p.active.MoveStudentsToActiveGroupAuthorized(ctx, studentIDs, activeGroupID, legacyMoveAuthorization(auth))
	if err != nil {
		return studentpresence.StudentMoveResult{}, err
	}
	return studentMoveResult(result), nil
}

func (p presenceOperations) MoveStudentsToTransit(ctx context.Context, studentIDs []int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
	result, err := p.active.MoveStudentsToTransitAuthorized(ctx, studentIDs, legacyMoveAuthorization(auth))
	if err != nil {
		return studentpresence.StudentMoveResult{}, err
	}
	return studentMoveResult(result), nil
}

func (p presenceOperations) SessionVisitsWithDisplay(ctx context.Context, groupID int64) ([]studentpresence.VisitDisplay, error) {
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

func (p presenceOperations) SessionRooms(ctx context.Context, ids []int64) ([]studentpresence.SessionRoomSummary, error) {
	rooms, err := p.active.GetRoomsByIDs(ctx, ids)
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

func (p presenceOperations) DashboardAnalytics(ctx context.Context) (studentpresence.DashboardAnalytics, error) {
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
	if p.atSchool != nil && len(analytics.HomeCandidateIDs) > 0 {
		atSchool, err := p.atSchool.CountAtSchoolToday(ctx, analytics.HomeCandidateIDs)
		if err != nil {
			return studentpresence.DashboardAnalytics{}, err
		}
		result.StudentsAtSchool = atSchool
		result.StudentsHome = max(0, result.StudentsHome-atSchool)
	}
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
			Name: item.Name, Type: item.Type, StudentCount: item.StudentCount, Location: item.Location, Status: item.Status,
		})
	}
	return result, nil
}

func (p presenceOperations) CrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]studentpresence.CrossTenantStudent, error) {
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

func (p presenceOperations) TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	return p.active.GetTrackingIndicators(ctx, studentIDs, labels)
}
