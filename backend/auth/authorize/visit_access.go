package authorize

import "context"

// VisitAccessQuery exposes only the identity relationships needed by the
// visit policy. Persistence models and service implementations stay at the
// API boundary.
type VisitAccessQuery interface {
	VisitStudentID(ctx context.Context, visitID int64) (int64, error)
	PersonIDByAccount(ctx context.Context, accountID int64) (int64, bool)
	StudentIDByPerson(ctx context.Context, personID int64) (int64, bool)
	StaffIDByPerson(ctx context.Context, personID int64) (int64, bool)
	TeacherIDByStaff(ctx context.Context, staffID int64) (int64, bool)
	TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error)
	StudentGroupID(ctx context.Context, studentID int64) (int64, bool)
	SupervisedActiveGroupIDs(ctx context.Context, staffID int64) []int64
	StudentCurrentActiveGroupID(ctx context.Context, studentID int64) (int64, bool)
}

// CanViewVisit evaluates the relationship-based part of visit access. Broad
// permission access is resolved by the caller from the security principal.
func CanViewVisit(ctx context.Context, accountID int64, canReadAll bool, visitID int64, query VisitAccessQuery) (bool, error) {
	if canReadAll {
		return true, nil
	}
	if visitID <= 0 || query == nil {
		return false, nil
	}

	studentID, err := query.VisitStudentID(ctx, visitID)
	if err != nil {
		return false, err
	}
	if studentID <= 0 {
		return false, nil
	}

	personID, ok := query.PersonIDByAccount(ctx, accountID)
	if !ok {
		return false, nil
	}
	if callerStudentID, ok := query.StudentIDByPerson(ctx, personID); ok && callerStudentID == studentID {
		return true, nil
	}
	return canViewVisitAsStaff(ctx, personID, studentID, query)
}

func canViewVisitAsStaff(ctx context.Context, personID, studentID int64, query VisitAccessQuery) (bool, error) {
	staffID, ok := query.StaffIDByPerson(ctx, personID)
	if !ok {
		return false, nil
	}
	teacherID, ok := query.TeacherIDByStaff(ctx, staffID)
	if !ok {
		return false, nil
	}

	teacherGroupIDs, err := query.TeacherGroupIDs(ctx, teacherID)
	if err != nil {
		return false, err
	}
	if studentGroupID, ok := query.StudentGroupID(ctx, studentID); ok && containsID(teacherGroupIDs, studentGroupID) {
		return true, nil
	}

	activeGroupID, ok := query.StudentCurrentActiveGroupID(ctx, studentID)
	return ok && containsID(query.SupervisedActiveGroupIDs(ctx, staffID), activeGroupID), nil
}

func containsID(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
