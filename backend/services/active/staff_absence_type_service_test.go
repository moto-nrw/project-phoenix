package active

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The consumer tests provide read facts only. Unconfigured booking checks
// remain unexpected calls through the embedded interface.
type absTypeReaderMock struct {
	AbsenceTypeReader
	rows      []*activeModels.StaffAbsenceType
	labelsErr error
	listCalls int
}

func (m *absTypeReaderMock) GetAbsenceType(_ context.Context, id int64) (*activeModels.StaffAbsenceType, error) {
	for _, row := range m.rows {
		if row.ID == id {
			return row, nil
		}
	}
	return nil, ErrAbsenceTypeNotFound
}

func (m *absTypeReaderMock) ResolveForAbsence(ctx context.Context, id int64) (*activeModels.StaffAbsenceType, error) {
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
	typ := &activeModels.StaffAbsenceType{Name: "Regenerationstag"}
	svc := &absTypeReaderMock{rows: []*activeModels.StaffAbsenceType{typ}}
	custom := &activeModels.StaffAbsence{AbsenceType: activeModels.AbsenceTypeOther, AbsenceTypeID: &typ.ID}
	standard := &activeModels.StaffAbsence{AbsenceType: activeModels.AbsenceTypeSick}
	StampAbsenceTypeLabels(context.Background(), svc, []*activeModels.StaffAbsence{custom, standard, nil}, nil)
	assert.Equal(t, "Regenerationstag", custom.AbsenceTypeLabel)
	assert.Empty(t, standard.AbsenceTypeLabel, "standard types retain their built-in label")
	assert.Equal(t, 1, svc.listCalls)
}

func TestStampAbsenceTypeLabelsIsNilSafe(t *testing.T) {
	t.Parallel()
	absence := &activeModels.StaffAbsence{AbsenceTypeID: new(int64)}
	StampAbsenceTypeLabels(context.Background(), nil, []*activeModels.StaffAbsence{absence}, nil)
	assert.Empty(t, absence.AbsenceTypeLabel)
}

func TestStampAbsenceTypeLabelsSkipsUnusedLookup(t *testing.T) {
	t.Parallel()
	svc := &absTypeReaderMock{}
	StampAbsenceTypeLabels(context.Background(), svc, []*activeModels.StaffAbsence{nil, {}}, nil)
	assert.Zero(t, svc.listCalls)
}

func TestStampAbsenceTypeLabelsPreservesLabelsAndLogsFailure(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	svc := &absTypeReaderMock{labelsErr: errors.New("lookup failed")}
	absence := &activeModels.StaffAbsence{AbsenceTypeID: new(int64), AbsenceTypeLabel: "Existing"}
	StampAbsenceTypeLabels(context.Background(), svc, []*activeModels.StaffAbsence{absence}, logger)
	assert.Equal(t, "Existing", absence.AbsenceTypeLabel)
	require.Contains(t, output.String(), "resolving absence type labels failed")
	assert.Contains(t, output.String(), "lookup failed")
	assert.Contains(t, output.String(), "WARN")
}
