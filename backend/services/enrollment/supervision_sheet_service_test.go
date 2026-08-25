package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type failingSupervisionCompanionRepo struct {
	userModels.StudentCompanionRepository
}

func (failingSupervisionCompanionRepo) ListLinksForStudents(context.Context, []int64) (map[int64][]userModels.CompanionLink, error) {
	return nil, errors.New("companion lookup failed")
}

func TestSupervisionDepartureFailsWhenCompanionLookupFails(t *testing.T) {
	t.Parallel()

	svc := &reportService{ReportServiceConfig: ReportServiceConfig{
		StudentCompanionRepo: failingSupervisionCompanionRepo{},
	}}
	student := &userModels.Student{}

	_, err := svc.supervisionDeparture(context.Background(), student, timezone.NewDate(2026, 8, 24), []int64{7})
	require.Error(t, err)
	assert.ErrorContains(t, err, "companion lookup failed")
}

func TestRecordSupervisionSheetAuditStoresStudentID(t *testing.T) {
	t.Parallel()

	repo := &fakeClassDayAccessLogRepo{}
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{DataAccessLogRepo: repo}}
	sheet := &SupervisionStudentSheet{StudentID: 17, Date: timezone.NewDate(2026, 8, 24)}

	require.NoError(t, svc.recordSupervisionSheetAudit(context.Background(), sheet, 42, "lehrkraft"))
	require.Len(t, repo.entries, 1)
	entry := repo.entries[0]
	assert.Equal(t, auditModels.ResourceTypeSupervisionStudentSheet, entry.ResourceType)
	if assert.NotNil(t, entry.StudentID) {
		assert.Equal(t, sheet.StudentID, *entry.StudentID)
	}
}
