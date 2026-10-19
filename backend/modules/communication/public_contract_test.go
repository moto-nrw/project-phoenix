package communication_test

import (
	"context"
	"testing"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	compose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type noViewerNames struct{}

func (noViewerNames) NamesByAccount(context.Context, []int64) (map[int64]string, compose.OperationStats, error) {
	return map[int64]string{}, compose.OperationStats{}, nil
}
func TestPublicCommunicationContractPersistsNormalizedAnnouncement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	var audited []recordports.AuditEntry
	module, err := api.NewCommunicationAnnouncements(compose.Dependencies{DB: db, Organizations: organizations, Viewers: noViewerNames{}, Audit: recordports.AuditFunc(func(_ context.Context, entry recordports.AuditEntry) error {
		audited = append(audited, entry)
		return nil
	}), Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	row := &communication.Announcement{Title: "  Wartung  ", Content: "  Heute Abend  ", Type: communication.TypeMaintenance, Severity: communication.SeverityWarning, CreatedBy: operator.ID}
	require.NoError(t, module.CreateAnnouncement(testpkg.Ctx(t), row, operator.ID, nil))
	require.Positive(t, row.ID)
	found, err := module.GetAnnouncement(testpkg.Ctx(t), row.ID)
	require.NoError(t, err)
	require.Equal(t, "Wartung", found.Title)
	require.Equal(t, "Heute Abend", found.Content)
	require.Equal(t, communication.TypeMaintenance, found.Type)
	require.Len(t, audited, 1)
	require.Equal(t, row.ID, audited[0].ResourceID)
}
