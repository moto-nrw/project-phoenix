package parent_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// masterDataStubSettings answers only the two master-data feature flags.
type masterDataStubSettings struct {
	configService.SettingsService
	editEnabled    bool
	requestEnabled bool
}

func (s masterDataStubSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	switch key {
	case configModels.KeyParentMasterDataEditEnabled:
		return s.editEnabled, nil
	case configModels.KeyParentMasterDataRequestEnabled:
		return s.requestEnabled, nil
	default:
		return false, nil
	}
}

func buildMasterDataService(t *testing.T, editEnabled bool) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		ChangeRequestRepo:   repos.StudentDataChangeRequest,
		Settings:            masterDataStubSettings{editEnabled: editEnabled},
		Broadcaster:         &captureBroadcaster{},
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestGetChildMasterData_ReturnsChainData(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	data, err := svc.GetChildMasterData(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, data)
	assert.Equal(t, "Felix", data.FirstName)
	assert.Equal(t, "Schneider", data.LastName)
	assert.Equal(t, chain.GuardianProfileID, data.GuardianProfileID)
	assert.Equal(t, chain.Email, *data.Email)
	assert.Empty(t, data.PendingChanges)
}

func TestUpdateMasterDataField_HealthInfo_AppliesAndAudits(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	data, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetStudent, "health_info",
		json.RawMessage(`"Penicillin-Allergie"`),
	)
	require.NoError(t, err)
	require.NotNil(t, data.HealthInfo)
	assert.Equal(t, "Penicillin-Allergie", *data.HealthInfo)

	// The live student record was updated.
	student, err := repositories.NewFactory(db).Student.FindByID(context.Background(), chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, student.HealthInfo)
	assert.Equal(t, "Penicillin-Allergie", *student.HealthInfo)

	// An auto_applied audit row was written.
	var count int
	err = db.NewRaw(
		`SELECT count(*) FROM users.student_data_change_requests WHERE student_id = ? AND status = ?`,
		chain.StudentID, usersModels.DataChangeStatusAutoApplied,
	).Scan(context.Background(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestUpdateMasterDataField_NonAllowlistedField_Rejected(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetPerson, "first_name",
		json.RawMessage(`"Hacker"`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataFieldNotEditable)
}

func TestUpdateMasterDataField_FeatureDisabled_Rejected(t *testing.T) {
	svc, db := buildMasterDataService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetStudent, "health_info",
		json.RawMessage(`"x"`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataEditDisabled)
}
