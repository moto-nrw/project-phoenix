package studentdirectoryview

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// Reader is the projection the roots bind to the student directory ports of
// Care Plan and People Directory (ADR 0058). Its signatures carry only the
// shared kernel and the transaction, so the projection names no consumer type
// and no consumer names a projection type. Every read takes the caller's
// transaction.
type Reader struct{}

// StudentRows is the directory row source the consumers embed in their
// statements.
func (Reader) StudentRows(db bun.IDB, tenantID int64) *bun.SelectQuery {
	return Query(db, tenantID)
}

// StudentModelRows is the directory row source with BUN's model column
// selection.
func (Reader) StudentModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	return ModelQuery(db, tenantID, model)
}

// StudentDisplayRows is the display row source Care Plan embeds.
func (Reader) StudentDisplayRows(db bun.IDB, tenantID int64) *bun.SelectQuery {
	return StudentDisplayQuery(db, tenantID)
}

// ListEndedCare hands each archived child of ListEndedCare to each and
// returns the total.
func (Reader) ListEndedCare(ctx context.Context, db bun.IDB, tenantID int64, asOf, search string, classes []string, page, pageSize int,
	each func(studentID int64, firstName, lastName, schoolClass string, lastCareDay calendar.Date)) (int, error) {
	rows, total, err := ListEndedCare(ctx, db, tenantID, asOf, EndedCareFilter{Search: search, SchoolClasses: classes, Page: page, PageSize: pageSize})
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		each(row.StudentID, row.FirstName, row.LastName, row.SchoolClass, row.LastCareDay)
	}
	return total, nil
}

// LockCareExitPeople is LockCareExitPeople.
func (Reader) LockCareExitPeople(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64) error {
	return LockCareExitPeople(ctx, db, tenantID, studentIDs)
}

// ListCareStudents hands each child of ListCareStudents to each.
func (Reader) ListCareStudents(ctx context.Context, db bun.IDB, tenantID int64, onDay string, studentIDs []int64, inactiveStatus, activeStatus string,
	each func(studentID int64, firstName, lastName, schoolClass string, enrolledUntil *calendar.Date)) error {
	rows, err := ListCareStudents(ctx, db, tenantID, onDay, studentIDs, CareStudentStatuses{Inactive: inactiveStatus, Active: activeStatus})
	if err != nil {
		return err
	}
	for _, row := range rows {
		each(row.StudentID, row.FirstName, row.LastName, row.SchoolClass, row.EnrolledUntil)
	}
	return nil
}

func (Reader) AllSchoolsStudentRows(db bun.IDB) *bun.SelectQuery { return AllSchoolsQuery(db) }
func (Reader) AllSchoolsStudentModelRows(db bun.IDB, model any) *bun.SelectQuery {
	return AllSchoolsModelQuery(db, model)
}
