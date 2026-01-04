package common

import (
	"context"
	"log"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// StudentDataSnapshot caches all data needed for building student list responses.
// This eliminates N+1 query problems by loading all related data in bulk.
type StudentDataSnapshot struct {
	Persons            map[int64]*userModels.Person
	Groups             map[int64]*educationModels.Group
	ScheduledCheckouts map[int64]*activeModels.ScheduledCheckout
	LocationSnapshot   *StudentLocationSnapshot
}

// LoadStudentDataSnapshot batches all data needed to build student list responses.
// This prevents N+1 queries by loading persons, groups, scheduled checkouts, and locations in bulk.
func LoadStudentDataSnapshot(
	ctx context.Context,
	personService userService.PersonService,
	educationSvc educationService.Service,
	activeSvc activeService.Service,
	studentIDs []int64,
	personIDs []int64,
	groupIDs []int64,
) (*StudentDataSnapshot, error) {
	snapshot := &StudentDataSnapshot{
		Persons:            make(map[int64]*userModels.Person),
		Groups:             make(map[int64]*educationModels.Group),
		ScheduledCheckouts: make(map[int64]*activeModels.ScheduledCheckout),
	}

	// Load persons (continue with empty map on error)
	if len(personIDs) > 0 {
		if persons, err := personService.GetByIDs(ctx, personIDs); err != nil {
			log.Printf("Failed to bulk load persons: %v", err)
		} else {
			snapshot.Persons = persons
		}
	}

	// Load groups (continue with empty map on error)
	if len(groupIDs) > 0 {
		if groups, err := educationSvc.GetGroupsByIDs(ctx, groupIDs); err != nil {
			log.Printf("Failed to bulk load groups: %v", err)
		} else {
			snapshot.Groups = groups
		}
	}

	// Load scheduled checkouts and location snapshot (continue on error)
	if len(studentIDs) > 0 {
		if checkouts, err := activeSvc.GetPendingScheduledCheckouts(ctx, studentIDs); err != nil {
			log.Printf("Failed to bulk load scheduled checkouts: %v", err)
		} else {
			snapshot.ScheduledCheckouts = checkouts
		}

		if locationSnapshot, err := LoadStudentLocationSnapshot(ctx, activeSvc, studentIDs); err != nil {
			log.Printf("Failed to load student location snapshot: %v", err)
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

// GetScheduledCheckout retrieves a scheduled checkout from the snapshot with nil safety
func (s *StudentDataSnapshot) GetScheduledCheckout(studentID int64) *activeModels.ScheduledCheckout {
	if s == nil || s.ScheduledCheckouts == nil {
		return nil
	}
	return s.ScheduledCheckouts[studentID]
}

// ResolveLocationWithTime retrieves location info including entry time from the snapshot
func (s *StudentDataSnapshot) ResolveLocationWithTime(studentID int64, hasFullAccess bool) StudentLocationInfo {
	if s == nil || s.LocationSnapshot == nil {
		return StudentLocationInfo{Location: "Abwesend"}
	}
	return s.LocationSnapshot.ResolveStudentLocationWithTime(studentID, hasFullAccess)
}
