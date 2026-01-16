package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// Staff Operations - implements StaffOperations interface

// GetStaffByID retrieves a staff record by their ID
func (s *personService) GetStaffByID(ctx context.Context, staffID int64) (*userModels.Staff, error) {
	staff, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, &UsersError{Op: "get staff by ID", Err: err}
	}
	return staff, nil
}

// GetStaffByPersonID retrieves a staff record by person ID
func (s *personService) GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	staff, err := s.staffRepo.FindByPersonID(ctx, personID)
	if err != nil {
		return nil, &UsersError{Op: "get staff by person ID", Err: err}
	}
	return staff, nil
}

// ListStaff retrieves all staff members
func (s *personService) ListStaff(ctx context.Context, _ *base.QueryOptions) ([]*userModels.Staff, error) {
	// Note: StaffRepository.List currently only accepts filters map, not QueryOptions
	// Pass nil to get all staff members
	staff, err := s.staffRepo.List(ctx, nil)
	if err != nil {
		return nil, &UsersError{Op: "list staff", Err: err}
	}
	return staff, nil
}

// ListStaffWithPerson retrieves all staff members with their associated person data
func (s *personService) ListStaffWithPerson(ctx context.Context) ([]*userModels.Staff, error) {
	staff, err := s.staffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return nil, &UsersError{Op: "list staff with person", Err: err}
	}
	return staff, nil
}

// GetStaffWithPerson retrieves a staff member with their person details
func (s *personService) GetStaffWithPerson(ctx context.Context, staffID int64) (*userModels.Staff, error) {
	staff, err := s.staffRepo.FindWithPerson(ctx, staffID)
	if err != nil {
		return nil, &UsersError{Op: "get staff with person", Err: err}
	}
	return staff, nil
}

// CreateStaff creates a new staff record
func (s *personService) CreateStaff(ctx context.Context, staff *userModels.Staff) error {
	if err := s.staffRepo.Create(ctx, staff); err != nil {
		return &UsersError{Op: "create staff", Err: err}
	}
	return nil
}

// UpdateStaff updates an existing staff record
func (s *personService) UpdateStaff(ctx context.Context, staff *userModels.Staff) error {
	if err := s.staffRepo.Update(ctx, staff); err != nil {
		return &UsersError{Op: "update staff", Err: err}
	}
	return nil
}

// DeleteStaff removes a staff record
func (s *personService) DeleteStaff(ctx context.Context, staffID int64) error {
	if err := s.staffRepo.Delete(ctx, staffID); err != nil {
		return &UsersError{Op: "delete staff", Err: err}
	}
	return nil
}

// Teacher Operations - implements TeacherOperations interface

// GetTeacherByStaffID retrieves a teacher by their staff ID
func (s *personService) GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	teacher, err := s.teacherRepo.FindByStaffID(ctx, staffID)
	if err != nil {
		return nil, &UsersError{Op: "get teacher by staff ID", Err: err}
	}
	return teacher, nil
}

// GetTeachersByStaffIDs retrieves teachers by multiple staff IDs
func (s *personService) GetTeachersByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error) {
	teachers, err := s.teacherRepo.FindByStaffIDs(ctx, staffIDs)
	if err != nil {
		return nil, &UsersError{Op: "get teachers by staff IDs", Err: err}
	}
	return teachers, nil
}

// CreateTeacher creates a new teacher record
func (s *personService) CreateTeacher(ctx context.Context, teacher *userModels.Teacher) error {
	if err := s.teacherRepo.Create(ctx, teacher); err != nil {
		return &UsersError{Op: "create teacher", Err: err}
	}
	return nil
}

// UpdateTeacher updates an existing teacher record
func (s *personService) UpdateTeacher(ctx context.Context, teacher *userModels.Teacher) error {
	if err := s.teacherRepo.Update(ctx, teacher); err != nil {
		return &UsersError{Op: "update teacher", Err: err}
	}
	return nil
}

// DeleteTeacher removes a teacher record
func (s *personService) DeleteTeacher(ctx context.Context, teacherID int64) error {
	if err := s.teacherRepo.Delete(ctx, teacherID); err != nil {
		return &UsersError{Op: "delete teacher", Err: err}
	}
	return nil
}

// GetStudentsByTeacher retrieves students supervised by a teacher (through group assignments)
func (s *personService) GetStudentsByTeacher(ctx context.Context, teacherID int64) ([]*userModels.Student, error) {
	// First verify the teacher exists
	teacher, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsByTeacher, Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: opGetStudentsByTeacher, Err: ErrTeacherNotFound}
	}

	// Use the repository method to get students by teacher ID
	students, err := s.studentRepo.FindByTeacherID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsByTeacher, Err: err}
	}

	return students, nil
}

// GetStudentsWithGroupsByTeacher retrieves students with group info supervised by a teacher
func (s *personService) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]StudentWithGroup, error) {
	// First verify the teacher exists
	teacher, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: ErrTeacherNotFound}
	}

	// Use the enhanced repository method to get students with group info
	studentsWithGroups, err := s.studentRepo.FindByTeacherIDWithGroups(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}

	// Convert to service layer struct
	results := make([]StudentWithGroup, 0, len(studentsWithGroups))
	for _, swg := range studentsWithGroups {
		result := StudentWithGroup{
			Student:   swg.Student,
			GroupName: swg.GroupName,
		}
		results = append(results, result)
	}

	return results, nil
}

// GetTeachersBySpecialization retrieves teachers by their specialization
func (s *personService) GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
	teachers, err := s.teacherRepo.FindBySpecialization(ctx, specialization)
	if err != nil {
		return nil, &UsersError{Op: "get teachers by specialization", Err: err}
	}
	return teachers, nil
}

// ListTeachersWithStaffAndPerson retrieves all teachers with their staff and person data
func (s *personService) ListTeachersWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
	teachers, err := s.teacherRepo.ListAllWithStaffAndPerson(ctx)
	if err != nil {
		return nil, &UsersError{Op: "list teachers with staff and person", Err: err}
	}
	return teachers, nil
}

// GetTeacherWithDetails retrieves a teacher with their associated staff and person data
func (s *personService) GetTeacherWithDetails(ctx context.Context, teacherID int64) (*userModels.Teacher, error) {
	teacher, err := s.teacherRepo.FindWithStaffAndPerson(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: "get teacher with details", Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: "get teacher with details", Err: ErrTeacherNotFound}
	}
	return teacher, nil
}

// Student Operations - implements StudentOperations interface

// GetStudentByPersonID retrieves a student record by person ID
func (s *personService) GetStudentByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	student, err := s.studentRepo.FindByPersonID(ctx, personID)
	if err != nil {
		return nil, &UsersError{Op: "get student by person ID", Err: err}
	}
	return student, nil
}

// GetStudentByID retrieves a student by their ID
func (s *personService) GetStudentByID(ctx context.Context, studentID int64) (*userModels.Student, error) {
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return nil, &UsersError{Op: "get student by ID", Err: err}
	}
	return student, nil
}

// RFID Card Operations - implements RFIDCardOperations interface

// ListAvailableRFIDCards returns RFID cards that are not assigned to any person
func (s *personService) ListAvailableRFIDCards(ctx context.Context) ([]*userModels.RFIDCard, error) {
	// First, get all active RFID cards
	filters := map[string]interface{}{
		"active": true,
	}

	allCards, err := s.rfidRepo.List(ctx, filters)
	if err != nil {
		return nil, &UsersError{Op: "list all RFID cards", Err: err}
	}

	// Get all persons to check which cards are assigned
	persons, err := s.personRepo.List(ctx, nil)
	if err != nil {
		return nil, &UsersError{Op: "list all persons", Err: err}
	}

	// Create a map of assigned tag IDs for fast lookup
	assignedTags := make(map[string]bool)
	for _, person := range persons {
		if person.TagID != nil {
			assignedTags[*person.TagID] = true
		}
	}

	// Filter out assigned cards
	var availableCards []*userModels.RFIDCard
	for _, card := range allCards {
		if !assignedTags[card.ID] {
			availableCards = append(availableCards, card)
		}
	}

	return availableCards, nil
}
