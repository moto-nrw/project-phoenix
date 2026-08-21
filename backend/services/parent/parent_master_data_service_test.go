package parent_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// masterDataSettings builds a parentSettingsStub with the master-data feature
// flags used by the master-data services, plus the related-accounts invite
// mode defaulted to disabled — matching masterDataStubSettings' prior
// per-key behavior.
func masterDataSettings(editEnabled, requestEnabled, guardianManagementEnabled bool) parentSettingsStub {
	return parentSettingsStub{
		boolValues: map[string]bool{
			configModels.KeyParentMasterDataEditEnabled:     editEnabled,
			configModels.KeyParentMasterDataRequestEnabled:  requestEnabled,
			configModels.KeyParentGuardianManagementEnabled: guardianManagementEnabled,
		},
		stringValues: map[string]string{
			configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
		},
	}
}

func buildMasterDataService(t *testing.T, editEnabled bool) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		ChangeRequestRepo:   repos.StudentDataChangeRequest,
		StudentAudit:        userService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default()),
		Settings:            masterDataSettings(editEnabled, false, true),
		Broadcaster:         testpkg.NewRecordingBroadcaster(),
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db
}

func TestUpdateMasterDataField_RecordsStudentAudit(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{
		ID:        int(chain.AccountID),
		FirstName: "Petra",
		LastName:  "Parent",
	})
	_, err := svc.UpdateMasterDataField(
		ctx,
		chain.AccountID,
		chain.StudentID,
		usersModels.DataChangeTargetStudent,
		"health_info",
		json.RawMessage(`"Neue Information"`),
	)
	require.NoError(t, err)

	repos := repositories.NewFactory(db)
	edits, err := repos.StudentFieldEdit.GetByStudentID(
		tenant.WithTenantID(context.Background(), chain.TenantID),
		chain.StudentID,
	)
	require.NoError(t, err)
	require.NotEmpty(t, edits)
	var healthEditFound bool
	for _, edit := range edits {
		if edit.FieldName == auditModels.StudentFieldHealthInfo {
			healthEditFound = true
			assert.Equal(t, chain.AccountID, edit.EditedBy)
			assert.Equal(t, "Petra Parent", edit.EditedByName)
		}
	}
	assert.True(t, healthEditFound)
}

func TestUpdateMasterDataField_GuardianManagementDisabledRejectsContactEdits(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		ChangeRequestRepo:   repos.StudentDataChangeRequest,
		Settings:            masterDataSettings(true, false, false),
		Broadcaster:         testpkg.NewRecordingBroadcaster(),
		DB:                  db,
		Logger:              slog.Default(),
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)

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

func TestChildFeatures_SplitsMasterDataContactCapability(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: repos.ParentChild,
		Settings:  masterDataSettings(true, true, false),
		DB:        db,
		Logger:    slog.Default(),
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	flags, err := svc.ChildFeatures(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.MasterDataEditEnabled, "health/direct master-data fields stay editable")
	assert.False(t, flags.MasterDataContactEditEnabled, "guardian contact fields follow guardian-management gate")
	assert.True(t, flags.MasterDataRequestEnabled)
}

func TestGetChildMasterData_ReturnsChainData(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	data, err := svc.GetChildMasterData(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, data)
	assert.Equal(t, "Felix", data.FirstName)
	assert.Equal(t, "Schneider", data.LastName)
	assert.Equal(t, chain.GuardianProfileID, data.GuardianProfileID)
	assert.Equal(t, chain.Email, *data.Email)
	assert.Empty(t, data.PendingChanges)
}

func TestMasterDataUsesSelectedChildGuardianProfile(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()
	secondTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, secondTenantID)
	testpkg.MapAccountToTenant(t, db, chain.AccountID, secondTenantID)
	secondStudent := testpkg.CreateTestStudentForTenant(t, db, secondTenantID, "Mila", "Schneider", "2a")

	secondEmail := "second-child-guardian@example.test"
	secondProfile := &usersModels.GuardianProfile{
		FirstName:              "Sabine",
		LastName:               "Zweite",
		Email:                  &secondEmail,
		AccountID:              &chain.AccountID,
		HasAccount:             true,
		PreferredContactMethod: "phone",
		LanguagePreference:     "en",
	}
	secondProfile.SetTenantID(secondTenantID)
	_, err := db.NewInsert().Model(secondProfile).ModelTableExpr(`users.guardian_profiles`).Exec(ctx)
	require.NoError(t, err)

	link := &usersModels.StudentGuardian{
		StudentID:         secondStudent.ID,
		GuardianProfileID: secondProfile.ID,
		RelationshipType:  "parent",
	}
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRolePrimaryGuardian)
	link.SetTenantID(secondTenantID)
	_, err = db.NewInsert().Model(link).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(t, err)

	data, err := svc.GetChildMasterData(ctx, chain.AccountID, secondStudent.ID)
	require.NoError(t, err)
	assert.Equal(t, secondProfile.ID, data.GuardianProfileID)
	require.NotNil(t, data.Email)
	assert.Equal(t, secondEmail, *data.Email)
	assert.Equal(t, "phone", data.PreferredContactMethod)
	assert.Equal(t, "en", data.LanguagePreference)

	updated, err := svc.UpdateMasterDataField(
		ctx, chain.AccountID, secondStudent.ID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(`"selected-child@example.test"`),
	)
	require.NoError(t, err)
	require.NotNil(t, updated.Email)
	assert.Equal(t, secondProfile.ID, updated.GuardianProfileID)
	assert.Equal(t, "selected-child@example.test", *updated.Email)

	firstProfile, err := repositories.NewFactory(db).GuardianProfile.FindByID(ctx, chain.GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, firstProfile.Email)
	assert.Equal(t, chain.Email, *firstProfile.Email)
}

func TestUpdateMasterDataField_HealthInfo_AppliesAndAudits(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

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

func TestUpdateMasterDataField_NormalizedNoopSkipsAudit(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	data, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(strconv.Quote("  "+chain.Email+"  ")),
	)
	require.NoError(t, err)
	require.NotNil(t, data.Email)
	assert.Equal(t, chain.Email, *data.Email)

	var count int
	err = db.NewRaw(
		`SELECT count(*) FROM users.student_data_change_requests WHERE student_id = ? AND status = ?`,
		chain.StudentID, usersModels.DataChangeStatusAutoApplied,
	).Scan(context.Background(), &count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestUpdateMasterDataField_GuardianProfile_AppliesAndAudits(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

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

func TestUpdateMasterDataField_GuardianProfile_NormalizesDisplayNameEmail(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	data, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(`"Mom <MOM@EXAMPLE.TEST>"`),
	)
	require.NoError(t, err)
	require.NotNil(t, data.Email)
	assert.Equal(t, "mom@example.test", *data.Email)

	profile, err := repositories.NewFactory(db).GuardianProfile.FindByID(context.Background(), chain.GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, profile.Email)
	assert.Equal(t, "mom@example.test", *profile.Email)
}

func TestUpdateMasterDataField_GuardianProfile_DuplicateEmailConflict(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestGuardianProfile(t, db, "taken-parent")
	require.NotNil(t, other.Email)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetGuardianProfile, "email",
		json.RawMessage(strconv.Quote(*other.Email)),
	)
	assert.ErrorIs(t, err, parentService.ErrGuardianEmailConflict)
}

func TestUpdateMasterDataField_GuardianProfile_AddressAndContactFields(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

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
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

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

	var clearRef *int64
	err = db.NewRaw(
		`SELECT target_ref_id
		   FROM users.student_data_change_requests
		  WHERE student_id = ? AND target = ? AND field_key = ? AND status = ?
		  ORDER BY id DESC
		  LIMIT 1`,
		chain.StudentID,
		usersModels.DataChangeTargetGuardianPhone,
		"primary",
		usersModels.DataChangeStatusAutoApplied,
	).Scan(context.Background(), &clearRef)
	require.NoError(t, err)
	require.NotNil(t, clearRef)
	assert.Equal(t, firstPhoneID, *clearRef)
}

func TestUpdateMasterDataField_InvalidDirectValues(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

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

func TestUpdateMasterDataField_RejectsOversizedDirectValues(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	tests := []struct {
		name   string
		target string
		field  string
		value  string
	}{
		{
			name:   "health info",
			target: usersModels.DataChangeTargetStudent,
			field:  "health_info",
			value:  strings.Repeat("x", 2001),
		},
		{
			name:   "email",
			target: usersModels.DataChangeTargetGuardianProfile,
			field:  "email",
			value:  strings.Repeat("a", 255) + "@example.test",
		},
		{
			name:   "address",
			target: usersModels.DataChangeTargetGuardianProfile,
			field:  "address_street",
			value:  strings.Repeat("x", 201),
		},
		{
			name:   "phone",
			target: usersModels.DataChangeTargetGuardianPhone,
			field:  "primary",
			value:  strings.Repeat("1", 41),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateMasterDataField(
				context.Background(), chain.AccountID, chain.StudentID,
				tt.target, tt.field,
				json.RawMessage(strconv.Quote(tt.value)),
			)
			assert.ErrorIs(t, err, parentService.ErrMasterDataInvalidValue)
		})
	}
}

func TestMasterDataField_InvalidOwnerAndPayloadRejected(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

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
	t.Parallel()

	svc, db := buildMasterDataService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetPerson, "first_name",
		json.RawMessage(`"Hacker"`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataFieldNotEditable)
}

func TestUpdateMasterDataField_FeatureDisabled_Rejected(t *testing.T) {
	t.Parallel()

	svc, db := buildMasterDataService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.UpdateMasterDataField(
		context.Background(), chain.AccountID, chain.StudentID,
		usersModels.DataChangeTargetStudent, "health_info",
		json.RawMessage(`"x"`),
	)
	assert.ErrorIs(t, err, parentService.ErrMasterDataEditDisabled)
}
