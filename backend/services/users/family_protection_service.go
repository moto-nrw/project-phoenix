package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

var (
	ErrFamilyProtectionForbidden = errors.New("parent requests: family protection requires configuration permission")
	ErrFamilyProtectionInvalid   = errors.New("parent requests: invalid family protection change")
	ErrFamilyProtectionNotFound  = errors.New("parent requests: student not found")
	// ErrFamilyProtectionUnchanged says the child is already in the requested
	// state, so the append-only ledger stays untouched. It travels with the
	// current event, not instead of it: the caller renders the state it asked
	// for and tells the user nothing changed.
	ErrFamilyProtectionUnchanged = errors.New("parent requests: family protection is already in the requested state")
)

type familyProtectionStudentLocker interface {
	FindByIDForUpdate(context.Context, int64) (*userModels.Student, error)
}

type FamilyProtectionService struct {
	events   userModels.FamilyProtectionEventRepository
	students familyProtectionStudentLocker
}

type FamilyProtectionManager interface {
	Current(context.Context, []int64) (map[int64]*userModels.FamilyProtectionEvent, error)
	Set(context.Context, SetFamilyProtectionInput) (*userModels.FamilyProtectionEvent, error)
}

func NewFamilyProtectionService(
	events userModels.FamilyProtectionEventRepository,
	students familyProtectionStudentLocker,
) *FamilyProtectionService {
	return &FamilyProtectionService{events: events, students: students}
}

type SetFamilyProtectionInput struct {
	StudentID      int64
	Enabled        bool
	Reason         string
	ActorAccountID int64
}

// Current returns the latest immutable privacy event per requested child.
// Children without an event are unprotected by default.
func (s *FamilyProtectionService) Current(ctx context.Context, studentIDs []int64) (map[int64]*userModels.FamilyProtectionEvent, error) {
	if s == nil || s.events == nil {
		return nil, errors.New("parent requests: family protection service is not configured")
	}
	unique := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]struct{}, len(studentIDs))
	for _, studentID := range studentIDs {
		if studentID <= 0 {
			return nil, ErrFamilyProtectionInvalid
		}
		if _, exists := seen[studentID]; exists {
			continue
		}
		seen[studentID] = struct{}{}
		unique = append(unique, studentID)
	}
	return s.events.CurrentForStudents(ctx, unique)
}

func (s *FamilyProtectionService) Set(ctx context.Context, input SetFamilyProtectionInput) (*userModels.FamilyProtectionEvent, error) {
	if !authorize.HasPermission(permissions.ConfigManage, jwt.PermissionsFromCtx(ctx)) {
		return nil, ErrFamilyProtectionForbidden
	}
	reason := strings.TrimSpace(input.Reason)
	if input.StudentID <= 0 || input.ActorAccountID <= 0 || reason == "" || len([]rune(reason)) > 500 {
		return nil, ErrFamilyProtectionInvalid
	}
	if s == nil || s.events == nil || s.students == nil {
		return nil, errors.New("parent requests: family protection service is not configured")
	}
	student, err := s.students.FindByIDForUpdate(ctx, input.StudentID)
	if err != nil {
		return nil, fmt.Errorf("parent requests: lock family protection student: %w", err)
	}
	if student == nil || student.IsAlumnus() {
		return nil, ErrFamilyProtectionNotFound
	}
	current, err := s.events.CurrentForStudents(ctx, []int64{input.StudentID})
	if err != nil {
		return nil, fmt.Errorf("parent requests: load family protection: %w", err)
	}
	if existing := current[input.StudentID]; existing != nil && existing.Enabled == input.Enabled {
		return existing, ErrFamilyProtectionUnchanged
	}
	event := &userModels.FamilyProtectionEvent{
		StudentID: input.StudentID, Enabled: input.Enabled, Reason: reason, ActorAccountID: input.ActorAccountID,
	}
	if err := s.events.Create(ctx, event); err != nil {
		return nil, fmt.Errorf("parent requests: append family protection: %w", err)
	}
	return event, nil
}
