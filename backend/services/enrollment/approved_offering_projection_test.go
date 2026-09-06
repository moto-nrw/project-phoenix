package enrollment

import (
	"context"
	"errors"
	"testing"

	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

type approvedSelectionFixture struct {
	ApprovedSelectionReader
	rows []*owner.ApprovedOfferingSelection
	err  error
}

func (f approvedSelectionFixture) ApprovedSelectionsForOfferings(context.Context, []int64, owner.Date) ([]*owner.ApprovedOfferingSelection, error) {
	return f.rows, f.err
}

type offeringStudentFixture struct {
	rows []OfferingStudent
	ids  []int64
	err  error
}

func (f *offeringStudentFixture) ListOfferingStudents(_ context.Context, ids []int64) ([]OfferingStudent, error) {
	f.ids = ids
	return f.rows, f.err
}

func TestApprovedOfferingProjectionExcludesAlumniAndMissingStudents(t *testing.T) {
	t.Parallel()
	students := &offeringStudentFixture{rows: []OfferingStudent{{ID: 21, SchoolClass: "3b"}, {ID: 22, Alumnus: true}}}
	selections := approvedSelectionFixture{rows: []*owner.ApprovedOfferingSelection{
		{Selection: &owner.RequestChildOffering{ID: 101}, StudentID: 21},
		{Selection: &owner.RequestChildOffering{ID: 102}, StudentID: 22},
		{Selection: &owner.RequestChildOffering{ID: 103}, StudentID: 23},
		{Selection: &owner.RequestChildOffering{ID: 104}, StudentID: 21},
	}}
	projection := NewApprovedOfferingProjection(selections, students)
	rows, err := projection.ListApprovedChildrenByCareOfferingIDs(t.Context(), []int64{31}, "2026-09-01")
	require.NoError(t, err)
	require.Equal(t, []int64{21, 22, 23}, students.ids)
	require.Len(t, rows, 2)
	require.Equal(t, selections.rows[0].Selection.ID, rows[0].Link.ID)
	require.Equal(t, selections.rows[3].Selection.ID, rows[1].Link.ID)
	require.Equal(t, "3b", rows[0].SchoolClass)
}

func TestApprovedOfferingProjectionPreservesDependencyErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("projection failure")
	projection := NewApprovedOfferingProjection(approvedSelectionFixture{err: failure}, nil)
	rows, err := projection.ListApprovedChildrenByCareOfferingIDs(t.Context(), []int64{31}, "2026-09-01")
	require.ErrorIs(t, err, failure)
	require.Nil(t, rows)
	projection = NewApprovedOfferingProjection(approvedSelectionFixture{rows: []*owner.ApprovedOfferingSelection{{StudentID: 21}}}, &offeringStudentFixture{err: failure})
	rows, err = projection.ListApprovedChildrenByCareOfferingIDs(t.Context(), []int64{31}, "2026-09-01")
	require.ErrorIs(t, err, failure)
	require.Nil(t, rows)
	rows, err = NewApprovedOfferingProjection(nil, nil).ListApprovedChildrenByCareOfferingIDs(t.Context(), nil, "2026-09-01")
	require.NoError(t, err)
	require.Empty(t, rows)
}

func (f approvedSelectionFixture) ApprovedSelectionsForStudents(context.Context, []int64, owner.Date, owner.Date) ([]*owner.ApprovedOfferingSelection, error) {
	return f.rows, f.err
}

func TestApprovedOfferingProjectionStudentRangeExcludesAlumniBeforeLookup(t *testing.T) {
	t.Parallel()
	projection := NewApprovedOfferingProjection(nil, &offeringStudentFixture{rows: []OfferingStudent{{ID: 21, Alumnus: true}}})
	rows, err := projection.ListApprovedByStudentIDsInRange(t.Context(), []int64{21}, "2026-09-01", "2026-09-01")
	require.NoError(t, err)
	require.Empty(t, rows)
	failure := errors.New("student selection failure")
	projection = NewApprovedOfferingProjection(approvedSelectionFixture{err: failure}, &offeringStudentFixture{rows: []OfferingStudent{{ID: 21}}})
	rows, err = projection.ListApprovedByStudentIDsInRange(t.Context(), []int64{21}, "2026-09-01", "2026-09-01")
	require.ErrorIs(t, err, failure)
	require.Nil(t, rows)
}
