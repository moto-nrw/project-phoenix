package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wsStaffAccessMock struct {
	WorkSessionStaff
	ScheduleFn func(context.Context, int64) (*StaffScheduleAssignment, error)
	UpdateFn   func(context.Context, *users.Staff) error
}

func (m *wsStaffAccessMock) ScheduleAssignment(ctx context.Context, id int64) (*StaffScheduleAssignment, error) {
	return m.ScheduleFn(ctx, id)
}

func (m *wsStaffAccessMock) Update(ctx context.Context, staff *users.Staff) error {
	return m.UpdateFn(ctx, staff)
}

func TestWorkSessionStaffNamesPreserveDisplayContracts(t *testing.T) {
	t.Parallel()
	svc, _, _, audit, _ := wsCreateTestService()
	audit.getBySessionIDFunc = func(context.Context, int64) ([]*WorkSessionEdit, error) {
		return []*WorkSessionEdit{{EditedBy: 42}, {EditedBy: 0}, {EditedBy: 99}}, nil
	}
	svc.staffRepo = &wsMockStaffRepository{staffNamesFunc: func(context.Context, []int64) (map[int64]WorkSessionStaffName, error) {
		return map[int64]WorkSessionStaffName{42: {FirstName: "Anna", LastName: ""}}, nil
	}}
	views, err := svc.loadSessionEditsView(context.Background(), &activeModels.WorkSession{StaffID: 42}, 1)
	require.NoError(t, err)
	require.Len(t, views, 3)
	assert.Equal(t, "Anna", views[0].EditorName)
	assert.True(t, views[0].IsSelfEdit)
	assert.Equal(t, "System", views[1].EditorName)
	assert.Empty(t, views[2].EditorName)
	date := timezone.NewDate(2026, time.July, 1)
	doc, err := svc.buildTimeTrackingDocument(context.Background(), 42, nil, date, date)
	require.NoError(t, err)
	assert.Equal(t, "Anna ", doc.Subtitle, "document names retain the original spacing")
}

func TestWorkSessionEditorLookupFailureDiscardsPartialNames(t *testing.T) {
	t.Parallel()
	svc, _, _, audit, _ := wsCreateTestService()
	audit.getBySessionIDFunc = func(context.Context, int64) ([]*WorkSessionEdit, error) {
		return []*WorkSessionEdit{{EditedBy: 42}}, nil
	}
	svc.staffRepo = &wsMockStaffRepository{staffNamesFunc: func(context.Context, []int64) (map[int64]WorkSessionStaffName, error) {
		return map[int64]WorkSessionStaffName{42: {FirstName: "Partial"}}, errors.New("name lookup failed")
	}}
	views, err := svc.loadSessionEditsView(context.Background(), &activeModels.WorkSession{StaffID: 42}, 1)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Empty(t, views[0].EditorName)
}
