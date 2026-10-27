package dataimport

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type sampleSchoolsDouble struct{ ids []int64 }

func (d sampleSchoolsDouble) RecordSampleChildrenRequest(context.Context, int64, int64, net.IP) ([]int64, error) {
	return d.ids, nil
}

// sampleImportDouble answers each school with the row errors in errs.
type sampleImportDouble struct {
	errs    map[int64][]ImportError[StudentImportRow]
	current int64
}

func (d *sampleImportDouble) Import(_ context.Context, request ImportRequest[StudentImportRow]) (*ImportResult[StudentImportRow], error) {
	rowErrs := d.errs[d.current]
	return &ImportResult[StudentImportRow]{
		TotalRows: len(request.Rows), CreatedCount: len(request.Rows) - len(rowErrs),
		ErrorCount: len(rowErrs), Errors: rowErrs,
	}, nil
}

func (*sampleImportDouble) ImportBatches(context.Context, ImportRequest[StudentImportRow], BatchAudit) (*ImportResult[StudentImportRow], error) {
	return nil, errors.New("not used")
}

func (*sampleImportDouble) RecordAuditInTransaction(context.Context, string, string, *ImportResult[StudentImportRow], int64, bool, int64) error {
	return errors.New("not used")
}

func rowError(codes ...string) ImportError[StudentImportRow] {
	row := ImportError[StudentImportRow]{}
	for _, code := range codes {
		row.Errors = append(row.Errors, ValidationError{Code: code, Severity: ErrorSeverityError})
	}
	row.Errors = append(row.Errors, ValidationError{Code: "group_empty", Severity: ErrorSeverityInfo})
	return row
}

func TestSampleChildrenCountOnlyExistingChildrenAsSkipped(t *testing.T) {
	t.Parallel()
	importer := &sampleImportDouble{errs: map[int64][]ImportError[StudentImportRow]{
		7: {rowError("already_exists"), rowError("already_exists")},
		8: {rowError("already_exists", "required"), rowError("create_failed")},
	}}
	workflow := NewSampleChildren(sampleSchoolsDouble{ids: []int64{7, 8}}, importer,
		func(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
			importer.current = schoolID
			return fn(ctx)
		},
		func() calendar.Date { return calendar.NewDate(2026, 10, 25) })

	result, err := workflow.FillTestTraeger(context.Background(), 3, 4, nil)

	require.ErrorIs(t, err, ErrSampleChildrenFailed, "a row refused for another reason is no existing child")
	require.Len(t, result.Schools, 2)
	assert.Equal(t, SampleSchoolResult{SchoolID: 7, CreatedCount: 22, SkippedCount: 2}, result.Schools[0])
	assert.Equal(t, SampleSchoolResult{SchoolID: 8, CreatedCount: 22, SkippedCount: 0, FailedCount: 2}, result.Schools[1])
	assert.Equal(t, 44, result.CreatedCount)
}

func TestSampleChildRowsFollowTheSchoolNotItsPosition(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 10, 25)
	first := SampleChildRows(4711, today)

	assert.Equal(t, first, SampleChildRows(4711, today), "the same school gets the same children on every run")
	assert.NotEqual(t, first, SampleChildRows(4712, today), "another school gets other children")
	names := map[string]bool{}
	for _, row := range first {
		names[row.FirstName+" "+row.LastName] = true
		assert.Empty(t, row.Guardians, "sample children have no guardians")
		assert.Empty(t, row.AddressStreet)
		assert.Empty(t, row.HealthInfo)
	}
	assert.Len(t, names, 24, "no name repeats within a school")
}
