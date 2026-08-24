package common

import (
	"context"
	"log/slog"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// StudentDataSnapshot caches all data needed for building student list responses.
// This eliminates N+1 query problems by loading all related data in bulk.
type StudentDataSnapshot struct {
	Persons          map[int64]*userModels.Person
	Groups           map[int64]*educationModels.Group
	LocationSnapshot *StudentLocationSnapshot
}

// LoadStudentDataSnapshot batches all data needed to build student list responses.
// This prevents N+1 queries by loading persons, groups, and locations in bulk.
func LoadStudentDataSnapshot(
	ctx context.Context,
	personService userService.PersonService,
	educationSvc educationService.Service,
	activeSvc activeService.Service,
	studentIDs []int64,
	personIDs []int64,
	groupIDs []int64,
) *StudentDataSnapshot {
	snapshot := &StudentDataSnapshot{
		Persons: make(map[int64]*userModels.Person),
		Groups:  make(map[int64]*educationModels.Group),
	}

	loadSnapshotPersons(ctx, snapshot, personService, personIDs)
	loadSnapshotGroups(ctx, snapshot, educationSvc, groupIDs)
	loadSnapshotLocations(ctx, snapshot, activeSvc, studentIDs)

	return snapshot
}

func loadSnapshotPersons(ctx context.Context, snapshot *StudentDataSnapshot, svc userService.PersonService, ids []int64) {
	if len(ids) == 0 {
		return
	}
	persons, err := svc.GetByIDs(ctx, ids)
	if err == nil {
		snapshot.Persons = persons
		return
	}
	slog.Default().Warn("failed to bulk load persons", slog.String("error", err.Error()))
}

func loadSnapshotGroups(ctx context.Context, snapshot *StudentDataSnapshot, svc educationService.Service, ids []int64) {
	if len(ids) == 0 {
		return
	}
	groups, err := svc.GetGroupsByIDs(ctx, ids)
	if err == nil {
		snapshot.Groups = groups
		return
	}
	slog.Default().Warn("failed to bulk load groups", slog.String("error", err.Error()))
}

func loadSnapshotLocations(ctx context.Context, snapshot *StudentDataSnapshot, svc activeService.Service, ids []int64) {
	if len(ids) == 0 {
		return
	}
	locations, err := LoadStudentLocationSnapshot(ctx, svc, ids)
	if err == nil {
		snapshot.LocationSnapshot = locations
		return
	}
	slog.Default().Warn("failed to load student location snapshot", slog.String("error", err.Error()))
}

// GetPerson retrieves a person from the snapshot with nil safety
func (s *StudentDataSnapshot) GetPerson(personID int64) *userModels.Person {
	if s == nil || s.Persons == nil {
		return nil
	}
	return s.Persons[personID]
}

// GetGroup retrieves a group from the snapshot with nil safety
func (s *StudentDataSnapshot) GetGroup(groupID int64) *educationModels.Group {
	if s == nil || s.Groups == nil {
		return nil
	}
	return s.Groups[groupID]
}

// ResolveLocationWithTime retrieves location info including entry time from the snapshot
func (s *StudentDataSnapshot) ResolveLocationWithTime(studentID int64, hasFullAccess bool) StudentLocationInfo {
	if s == nil || s.LocationSnapshot == nil {
		return StudentLocationInfo{Location: "Abwesend"}
	}
	return s.LocationSnapshot.ResolveStudentLocationWithTime(studentID, hasFullAccess)
}
