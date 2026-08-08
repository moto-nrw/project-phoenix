package usercontext

import (
	"context"
	"errors"
)

const opGetMySchoolClasses = "get my school classes"

// GetMySchoolClasses retrieves the school classes assigned to the current
// staff member (#1772). It mirrors GetMyGroups: an account that is cleanly
// not linked to a person/staff resolves to an empty set (a guardian-only or
// operator account simply has no classes), any other lookup failure is an
// error. Results are request-memoized like the rest of the identity chain.
func (s *userContextService) GetMySchoolClasses(ctx context.Context) ([]string, error) {
	if !isAuthenticated(ctx) {
		return nil, &UserContextError{Op: opGetMySchoolClasses, Err: ErrUserNotAuthenticated}
	}

	cache, key, memo := identityMemo(ctx)
	if memo {
		if classes, ok := cache.schoolClassesFor(key); ok {
			return classes, nil
		}
	}

	staff, err := s.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, ErrUserNotLinkedToStaff) || errors.Is(err, ErrUserNotLinkedToPerson) {
			empty := []string{}
			if memo {
				cache.storeSchoolClasses(key, empty)
			}
			return empty, nil
		}
		return nil, &UserContextError{Op: opGetMySchoolClasses, Err: err}
	}

	assignments, err := s.classTeacherRepo.FindByStaff(ctx, staff.ID)
	if err != nil {
		return nil, &UserContextError{Op: opGetMySchoolClasses, Err: err}
	}

	classes := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		classes = append(classes, assignment.SchoolClass)
	}

	if memo {
		cache.storeSchoolClasses(key, classes)
	}
	return classes, nil
}
