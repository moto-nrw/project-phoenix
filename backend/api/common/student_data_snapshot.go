package common

import (
	"context"
	"fmt"
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
) (*StudentDataSnapshot, error) {
	return loadStudentDataSnapshot(ctx, personService, educationSvc, activeSvc, studentIDs, personIDs, groupIDs, false)
}

// LoadStudentDataSnapshotStrict loads the same bulk projection as
// LoadStudentDataSnapshot, but returns any sub-load error instead of replacing
// the failed section with empty data.
func LoadStudentDataSnapshotStrict(
	ctx context.Context,
	personService userService.PersonService,
	educationSvc educationService.Service,
	activeSvc activeService.Service,
	studentIDs []int64,
	personIDs []int64,
	groupIDs []int64,
) (*StudentDataSnapshot, error) {
	return loadStudentDataSnapshot(ctx, personService, educationSvc, activeSvc, studentIDs, personIDs, groupIDs, true)
}

func loadStudentDataSnapshot(
	ctx context.Context,
	personService userService.PersonService,
	educationSvc educationService.Service,
	activeSvc activeService.Service,
	studentIDs []int64,
	personIDs []int64,
	groupIDs []int64,
	strict bool,
) (*StudentDataSnapshot, error) {
	snapshot := &StudentDataSnapshot{
		Persons: make(map[int64]*userModels.Person),
		Groups:  make(map[int64]*educationModels.Group),
	}

	// Load persons. Permissive callers continue with an empty map on error.
	if len(personIDs) > 0 {
		if persons, err := personService.GetByIDs(ctx, personIDs); err != nil {
			if strict {
				return nil, fmt.Errorf("bulk load persons: %w", err)
			}
			slog.Default().Warn("failed to bulk load persons", slog.String("error", err.Error()))
		} else {
			snapshot.Persons = persons
		}
	}

	// Load groups. Permissive callers continue with an empty map on error.
	if len(groupIDs) > 0 {
		if groups, err := educationSvc.GetGroupsByIDs(ctx, groupIDs); err != nil {
			if strict {
				return nil, fmt.Errorf("bulk load groups: %w", err)
			}
			slog.Default().Warn("failed to bulk load groups", slog.String("error", err.Error()))
		} else {
			snapshot.Groups = groups
		}
	}

	// Load locations. Permissive callers continue without a snapshot on error.
	if len(studentIDs) > 0 {
		if locationSnapshot, err := LoadStudentLocationSnapshot(ctx, activeSvc, studentIDs); err != nil {
			if strict {
				return nil, fmt.Errorf("load student locations: %w", err)
			}
			slog.Default().Warn("failed to load student location snapshot", slog.String("error", err.Error()))
		} else {
			snapshot.LocationSnapshot = locationSnapshot
		}
	}

	return snapshot, nil
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
