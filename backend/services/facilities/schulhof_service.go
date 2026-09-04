package facilities

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

type SchulhofService interface {
	GetSchulhofStatus(context.Context, int64) (*SchulhofStatus, error)
	EnsureInfrastructure(context.Context, int64) (*SystemActivity, error)
}

type SchulhofStatus struct {
	Exists            bool             `json:"exists"`
	RoomID            *int64           `json:"room_id,omitempty"`
	RoomName          string           `json:"room_name"`
	ActivityGroupID   *int64           `json:"activity_group_id,omitempty"`
	ActiveGroupID     *int64           `json:"active_group_id,omitempty"`
	IsUserSupervising bool             `json:"is_user_supervising"`
	SupervisionID     *int64           `json:"supervision_id,omitempty"`
	SupervisorCount   int              `json:"supervisor_count"`
	StudentCount      int              `json:"student_count"`
	Supervisors       []SupervisorInfo `json:"supervisors"`
}

type SupervisorInfo struct {
	ID            int64  `json:"id"`
	StaffID       int64  `json:"staff_id"`
	Name          string `json:"name"`
	IsCurrentUser bool   `json:"is_current_user"`
}

type schulhofService struct {
	rooms      Service
	activities ActivityCatalog
	groups     OpenGroupCatalog
	logger     *slog.Logger
}

var errSchulhofActivityNotFound = errors.New("schulhof activity not found")

func NewSchulhofService(rooms Service, activities ActivityCatalog, groups OpenGroupCatalog, logger *slog.Logger) SchulhofService {
	if rooms == nil || activities == nil || groups == nil {
		panic("Schulhof service: rooms, activities, and open groups are required")
	}
	return &schulhofService{rooms: rooms, activities: activities, groups: groups, logger: logger}
}

func (s *schulhofService) getLogger() *slog.Logger { return loggerOrDefault(s.logger) }

func (s *schulhofService) GetSchulhofStatus(ctx context.Context, staffID int64) (*SchulhofStatus, error) {
	status := &SchulhofStatus{RoomName: facilitiesModule.SchulhofRoomName, Supervisors: []SupervisorInfo{}}
	room, err := FindCanonicalSchulhofRoom(ctx, s.rooms)
	if errors.Is(err, ErrRoomNotFound) {
		return status, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to look up Schulhof room: %w", err)
	}
	status.Exists, status.RoomID = true, &room.ID
	activity, err := s.findActivity(ctx, room)
	if err == nil {
		status.ActivityGroupID = &activity.ID
	} else if !errors.Is(err, errSchulhofActivityNotFound) {
		return nil, fmt.Errorf("failed to look up Schulhof activity: %w", err)
	}
	group, supervisors, err := s.preferredOpenGroup(ctx, room.ID, staffID)
	if err != nil || group == nil {
		return statusOrError(status, err)
	}
	status.ActiveGroupID = &group.ID
	applySupervisors(status, supervisors, staffID)
	visits, err := s.groups.ListVisits(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up Schulhof visits: %w", err)
	}
	for _, visit := range visits {
		if visit.ExitTime == nil {
			status.StudentCount++
		}
	}
	return status, nil
}

func statusOrError(status *SchulhofStatus, err error) (*SchulhofStatus, error) {
	if err != nil {
		return nil, fmt.Errorf("failed to look up open Schulhof group: %w", err)
	}
	return status, nil
}

func applySupervisors(status *SchulhofStatus, supervisors []OpenGroupSupervisor, staffID int64) {
	for _, supervisor := range supervisors {
		if supervisor.Ended {
			continue
		}
		info := SupervisorInfo{
			ID: supervisor.ID, StaffID: supervisor.StaffID,
			Name:          supervisor.FirstName + " " + supervisor.LastName,
			IsCurrentUser: supervisor.StaffID == staffID,
		}
		status.Supervisors = append(status.Supervisors, info)
		if info.IsCurrentUser {
			status.IsUserSupervising, status.SupervisionID = true, &supervisor.ID
		}
	}
	status.SupervisorCount = len(status.Supervisors)
}

func (s *schulhofService) EnsureInfrastructure(ctx context.Context, createdBy int64) (*SystemActivity, error) {
	room, err := s.ensureRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof room: %w", err)
	}
	activity, err := s.findActivity(ctx, room)
	if err == nil {
		return activity, nil
	}
	if !errors.Is(err, errSchulhofActivityNotFound) {
		return nil, fmt.Errorf("failed to look up Schulhof activity: %w", err)
	}
	category, err := s.ensureCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure Schulhof category: %w", err)
	}
	created := SystemActivity{
		Name: facilitiesModule.SchulhofActivityName, MaxParticipants: facilitiesModule.SchulhofMaxParticipants,
		IsOpen: true, CategoryID: category.ID, PlannedRoomID: &room.ID, IsSystem: true,
	}
	if createdBy > 0 {
		created.CreatedBy = &createdBy
	}
	created, err = s.activities.CreateActivity(ctx, created)
	if err != nil {
		return nil, fmt.Errorf("failed to create Schulhof activity: %w", err)
	}
	s.getLogger().Info("successfully created Schulhof infrastructure",
		"room_id", room.ID,
		"category_id", category.ID,
		"activity_id", created.ID,
	)
	return &created, nil
}

func (s *schulhofService) findActivity(ctx context.Context, room *facilitiesModule.Room) (*SystemActivity, error) {
	groups, err := s.activities.ListActivities(ctx, facilitiesModule.SchulhofActivityName)
	if err != nil {
		return nil, fmt.Errorf("failed to query Schulhof activity: %w", err)
	}
	for index := range groups {
		if ValidateSchulhofActivityRoom(&groups[index], room) == nil {
			return &groups[index], nil
		}
	}
	return nil, errSchulhofActivityNotFound
}

func (s *schulhofService) preferredOpenGroup(ctx context.Context, roomID, staffID int64) (*OpenGroup, []OpenGroupSupervisor, error) {
	groups, err := s.groups.ListByRoom(ctx, roomID)
	if err != nil {
		return nil, nil, err
	}
	open := make([]OpenGroup, 0, len(groups))
	ids := make([]int64, 0, len(groups))
	var newest *OpenGroup
	for index := range groups {
		group := &groups[index]
		if group.EndTime != nil || !group.IsToday {
			continue
		}
		open, ids = append(open, *group), append(ids, group.ID)
		if newerOpenGroup(group, newest) {
			newest = group
		}
	}
	if newest == nil {
		return nil, nil, nil
	}
	supervisors, err := s.groups.ListSupervisors(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	return selectPreferredGroup(open, supervisors, newest, staffID)
}

func selectPreferredGroup(groups []OpenGroup, supervisors []OpenGroupSupervisor, newest *OpenGroup, staffID int64) (*OpenGroup, []OpenGroupSupervisor, error) {
	byGroup := make(map[int64][]OpenGroupSupervisor, len(groups))
	for _, supervisor := range supervisors {
		byGroup[supervisor.GroupID] = append(byGroup[supervisor.GroupID], supervisor)
	}
	var supervised *OpenGroup
	for index := range groups {
		for _, supervisor := range byGroup[groups[index].ID] {
			if supervisor.StaffID == staffID && !supervisor.Ended && newerOpenGroup(&groups[index], supervised) {
				supervised = &groups[index]
			}
		}
	}
	selected := newest
	if supervised != nil {
		selected = supervised
	}
	return selected, byGroup[selected.ID], nil
}

func newerOpenGroup(candidate, current *OpenGroup) bool {
	return current == nil || candidate.StartTime.After(current.StartTime) ||
		(candidate.StartTime.Equal(current.StartTime) && candidate.ID > current.ID)
}

func (s *schulhofService) ensureRoom(ctx context.Context) (*facilitiesModule.Room, error) {
	room, err := FindCanonicalSchulhofRoom(ctx, s.rooms)
	if err == nil {
		return room, nil
	}
	if !errors.Is(err, ErrRoomNotFound) {
		return nil, err
	}
	room = &facilitiesModule.Room{
		Name: facilitiesModule.SchulhofRoomName, Capacity: pointer(facilitiesModule.SchulhofRoomCapacity),
		Category: pointer(facilitiesModule.SchulhofCategoryName), IsSystem: true,
	}
	if err := s.rooms.CreateRoom(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

func (s *schulhofService) ensureCategory(ctx context.Context) (*SystemCategory, error) {
	categories, err := s.activities.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	for index := range categories {
		if categories[index].Name == facilitiesModule.SchulhofCategoryName {
			return &categories[index], nil
		}
	}
	created, err := s.activities.CreateCategory(ctx, SystemCategory{
		Name: facilitiesModule.SchulhofCategoryName, Description: facilitiesModule.SchulhofCategoryDescription,
		Color: facilitiesModule.SchulhofColor, IsSystem: true,
	})
	return &created, err
}
