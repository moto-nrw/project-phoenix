package timetracking

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The consumer tests provide read facts only. Unconfigured booking checks
// remain unexpected calls through the embedded interface.
type absTypeReaderMock struct {
	AbsenceTypeReader
	rows      []*StaffAbsenceType
	labelsErr error
	listCalls int
}

func (m *absTypeReaderMock) GetAbsenceType(_ context.Context, id int64) (*StaffAbsenceType, error) {
	for _, row := range m.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return nil, ErrAbsenceTypeNotFound
}

func (m *absTypeReaderMock) ResolveForAbsence(ctx context.Context, id int64) (*StaffAbsenceType, error) {
	return m.GetAbsenceType(ctx, id)
}

func (m *absTypeReaderMock) LabelsByID(context.Context) (map[int64]string, error) {
	m.listCalls++
	if m.labelsErr != nil {
		return nil, m.labelsErr
	}
	labels := make(map[int64]string, len(m.rows))
	for _, row := range m.rows {
		labels[row.ID] = row.Name
	}
	return labels, nil
}

func TestStampAbsenceTypeLabelsFillsOnlyCustomRows(t *testing.T) {
	t.Parallel()
	typ := &StaffAbsenceType{Name: "Regenerationstag"}
	svc := &absTypeReaderMock{rows: []*StaffAbsenceType{typ}}
	custom := &StaffAbsence{AbsenceType: AbsenceTypeOther, AbsenceTypeID: &typ.ID}
	standard := &StaffAbsence{AbsenceType: AbsenceTypeSick}
	StampAbsenceTypeLabels(context.Background(), svc, []*StaffAbsence{custom, standard, nil}, nil)
	assert.Equal(t, "Regenerationstag", custom.AbsenceTypeLabel)
	assert.Empty(t, standard.AbsenceTypeLabel, "standard types retain their built-in label")
	assert.Equal(t, 1, svc.listCalls)
}

func TestStampAbsenceTypeLabelsIsNilSafe(t *testing.T) {
	t.Parallel()
	absence := &StaffAbsence{AbsenceTypeID: new(int64)}
	StampAbsenceTypeLabels(context.Background(), nil, []*StaffAbsence{absence}, nil)
	assert.Empty(t, absence.AbsenceTypeLabel)
}

func TestStampAbsenceTypeLabelsSkipsUnusedLookup(t *testing.T) {
	t.Parallel()
	svc := &absTypeReaderMock{}
	StampAbsenceTypeLabels(context.Background(), svc, []*StaffAbsence{nil, {}}, nil)
	assert.Zero(t, svc.listCalls)
}

func TestStampAbsenceTypeLabelsPreservesLabelsAndLogsFailure(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	svc := &absTypeReaderMock{labelsErr: errors.New("lookup failed")}
	absence := &StaffAbsence{AbsenceTypeID: new(int64), AbsenceTypeLabel: "Existing"}
	StampAbsenceTypeLabels(context.Background(), svc, []*StaffAbsence{absence}, logger)
	assert.Equal(t, "Existing", absence.AbsenceTypeLabel)
	require.Contains(t, output.String(), "resolving absence type labels failed")
	assert.Contains(t, output.String(), "lookup failed")
	assert.Contains(t, output.String(), "WARN")
}
