package repositories

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	"github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	// Users domain
	Person   userModels.PersonRepository
	RFIDCard userModels.RFIDCardRepository
	Student  userModels.StudentRepository
	Teacher  userModels.TeacherRepository

	// Facilities domain
	Room facilityModels.RoomRepository

	// Education domain
	Group                 educationModels.GroupRepository
	GroupTeacher          educationModels.GroupTeacherRepository
	GroupSubstitution     educationModels.GroupSubstitutionRepository
	CombinedGroup         educationModels.CombinedGroupRepository
	CombinedGroupMember   educationModels.CombinedGroupMemberRepository
	CombinedGroupTeacher  educationModels.CombinedGroupTeacherRepository

	// Add other repositories here as they are implemented
	// Auth domain
	// Account  auth.AccountRepository

	// Activities domain
	// Activity   activities.ActivityRepository
	// Category   activities.CategoryRepository

	// ... and so on
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB) *Factory {
	return &Factory{
		// Initialize all repositories
		Person:   users.NewPersonRepository(db),
		RFIDCard: users.NewRFIDCardRepository(db),
		Student:  users.NewStudentRepository(db),
		Teacher:  users.NewTeacherRepository(db),

		// Facilities repositories
		Room: facilities.NewRoomRepository(db),

		// Education repositories
		Group:                 education.NewGroupRepository(db),
		GroupTeacher:          education.NewGroupTeacherRepository(db),
		GroupSubstitution:     education.NewGroupSubstitutionRepository(db),
		CombinedGroup:         education.NewCombinedGroupRepository(db),
		CombinedGroupMember:   education.NewCombinedGroupMemberRepository(db),
		CombinedGroupTeacher:  education.NewCombinedGroupTeacherRepository(db),

		// Add other repositories as they are implemented
	}
}
