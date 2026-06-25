package parent_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
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

// masterDataStubSettings answers the parent-write feature flags used by the
// master-data services.
type masterDataStubSettings struct {
	configService.SettingsService
	editEnabled               bool
	requestEnabled            bool
	guardianManagementEnabled bool
}

func (s masterDataStubSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	switch key {
	case configModels.KeyParentMasterDataEditEnabled:
		return s.editEnabled, nil
	case configModels.KeyParentMasterDataRequestEnabled:
		return s.requestEnabled, nil
	case configModels.KeyParentGuardianManagementEnabled:
		return s.guardianManagementEnabled, nil
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
		Settings:            masterDataStubSettings{editEnabled: editEnabled, guardianManagementEnabled: true},
		Broadcaster:         &captureBroadcaster{},
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestUpdateMasterDataField_GuardianManagementDisabledRejectsContactEdits(t *testing.T) {
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
		Settings:            masterDataStubSettings{editEnabled: true, guardianManagementEnabled: false},
		Broadcaster:         &captureBroadcaster{},
		DB:                  db,
		Logger:              slog.Default(),
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(`"new.parent@example.test"`),
	)
	assert.ErrorIs(t, err, parentService.ErrGuardianManagementDisabled)

	data, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetStudent, "health_info",
		json.RawMessage(`"allowed"`),
	)
	require.NoError(t, err)
	require.NotNil(t, data.HealthInfo)
	assert.Equal(t, "allowed", *data.HealthInfo)
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

func TestUpdateMasterDataField_GuardianProfile_AppliesAndAudits(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	data, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(`"  NEW.PARENT@EXAMPLE.TEST  "`),
	)
	require.NoError(t, err)
	require.NotNil(t, data.Email)
	assert.Equal(t, "new.parent@example.test", *data.Email)

	data, err = svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "language_preference",
		json.RawMessage(`"en"`),
	)
	require.NoError(t, err)
	assert.Equal(t, "en", data.LanguagePreference)

	var rows []struct {
		Target      string
		FieldKey    string
		NewValue    string
		TargetRefID *int64
	}
	err = db.NewRaw(
		`SELECT target, field_key, new_value::text, target_ref_id
		   FROM users.student_data_change_requests
		  WHERE student_id = ? AND status = ?
		  ORDER BY id`,
		chain.StudentID, usersModels.DataChangeStatusAutoApplied,
	).Scan(context.Background(), &rows)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, usersModels.DataChangeTargetGuardianProfile, rows[0].Target)
	assert.Equal(t, "email", rows[0].FieldKey)
	assert.Equal(t, `"new.parent@example.test"`, rows[0].NewValue)
	require.NotNil(t, rows[0].TargetRefID)
	assert.Equal(t, chain.GuardianProfileID, *rows[0].TargetRefID)
}

func TestUpdateMasterDataField_GuardianProfile_DuplicateEmailConflict(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	other := testpkg.CreateTestGuardianProfile(t, db, "taken-parent")
	defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", other.ID)
	require.NotNil(t, other.Email)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(strconv.Quote(*other.Email)),
	)
	assert.ErrorIs(t, err, parentService.ErrGuardianEmailConflict)
}

func TestUpdateMasterDataField_GuardianProfile_AddressAndContactFields(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	cases := []struct {
		field string
		value string
		check func(*testing.T, *parentService.ChildMasterData)
	}{
		{
			field: "address_street",
			value: `"  Hauptstr. 7  "`,
			check: func(t *testing.T, data *parentService.ChildMasterData) {
				require.NotNil(t, data.AddressStreet)
				assert.Equal(t, "Hauptstr. 7", *data.AddressStreet)
			},
		},
		{
			field: "address_city",
			value: `" Köln "`,
			check: func(t *testing.T, data *parentService.ChildMasterData) {
				require.NotNil(t, data.AddressCity)
				assert.Equal(t, "Köln", *data.AddressCity)
			},
		},
		{
			field: "address_postal_code",
			value: `" 50667 "`,
			check: func(t *testing.T, data *parentService.ChildMasterData) {
				require.NotNil(t, data.AddressPostalCode)
				assert.Equal(t, "50667", *data.AddressPostalCode)
			},
		},
		{
			field: "preferred_contact_method",
			value: `"phone"`,
			check: func(t *testing.T, data *parentService.ChildMasterData) {
				assert.Equal(t, "phone", data.PreferredContactMethod)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			data, err := svc.UpdateMasterDataField(
				context.Background(), chain.AccountID, chain.StudentID,
				usersModels.DataChangeTargetGuardianProfile, tc.field,
				json.RawMessage(tc.value),
			)
			require.NoError(t, err)
			tc.check(t, data)
		})
	}
}

func TestUpdateMasterDataField_GuardianPhone_CreateUpdateClear(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	data, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianPhone, "primary",
		json.RawMessage(`"+49 151 12345678"`),
	)
	require.NoError(t, err)
	require.NotNil(t, data.PrimaryPhone)
	assert.Equal(t, "+49 151 12345678", *data.PrimaryPhone)
	require.NotNil(t, data.PrimaryPhoneID)
	firstPhoneID := *data.PrimaryPhoneID

	data, err = svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianPhone, "primary",
		json.RawMessage(`"+49 151 87654321"`),
	)
	require.NoError(t, err)
	require.NotNil(t, data.PrimaryPhone)
	assert.Equal(t, "+49 151 87654321", *data.PrimaryPhone)
	require.NotNil(t, data.PrimaryPhoneID)
	assert.Equal(t, firstPhoneID, *data.PrimaryPhoneID, "update should reuse the existing primary phone row")

	data, err = svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianPhone, "primary",
		json.RawMessage(`""`),
	)
	require.NoError(t, err)
	assert.Nil(t, data.PrimaryPhone)
	assert.Nil(t, data.PrimaryPhoneID)
}

func TestUpdateMasterDataField_InvalidDirectValues(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(`"not-an-email"`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "preferred_contact_method",
		json.RawMessage(`""`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)

	_, err = svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "language_preference",
		json.RawMessage(`"zz"`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)
}

func TestMasterDataField_InvalidOwnerAndPayloadRejected(t *testing.T) {
	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.GetChildMasterData(context.Background(), 0, chain.StudentID)
	require.Error(t, err)

	_, err = svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetStudent, "health_info",
		json.RawMessage(`123`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)
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
