package contracttest_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedStudentChange captures what the port hands the owner capability.
type recordedStudentChange struct {
	before, after peopleModule.StudentAuditSnapshot
	editedBy      int64
	editedByName  string
}

type stubStudentAuditCapability struct {
	recorded []recordedStudentChange
}

func (c *stubStudentAuditCapability) RecordStudentChanges(
	_ context.Context,
	before, after peopleModule.StudentAuditSnapshot,
	editedBy int64,
	editedByName string,
) error {
	c.recorded = append(c.recorded, recordedStudentChange{
		before: before, after: after, editedBy: editedBy, editedByName: editedByName,
	})
	return nil
}

func (*stubStudentAuditCapability) RecordStudentPickupPlan(
	context.Context, int64, string, string, string, string, int64, string,
) error {
	return nil
}

func (*stubStudentAuditCapability) ListStudentChangeHistory(
	context.Context, int64,
) ([]peopleModule.StudentFieldEdit, error) {
	return nil, nil
}

func TestStudentAuditCompositionRecordsSystemStatusChange(t *testing.T) {
	t.Parallel()

	capability := &stubStudentAuditCapability{}
	service := api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAuditFor(capability))

	err := service.RecordSystemStatusChange(
		context.Background(),
		42,
		peopleModule.StudentStatusPending,
		peopleModule.StudentStatusActive,
	)

	require.NoError(t, err)
	require.Len(t, capability.recorded, 1)
	recorded := capability.recorded[0]
	assert.Equal(t, int64(42), recorded.after.StudentID)
	assert.Equal(t, peopleModule.StudentAuditSystemActorID, recorded.editedBy)
	assert.Equal(t, peopleModule.StudentAuditSystemActorName, recorded.editedByName)
	assert.Equal(t, string(peopleModule.StudentStatusPending), recorded.before.Status)
	assert.Equal(t, string(peopleModule.StudentStatusActive), recorded.after.Status)
}

// The editor's display name comes from the authenticated caller, and a
// mismatched context is attributed to nobody rather than the wrong person.
func TestStudentAuditCompositionResolvesActorName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		claims     sessiontoken.AppClaims
		editedBy   int64
		wantEditor string
	}{
		{
			name:       "full name",
			claims:     sessiontoken.AppClaims{ID: 23, FirstName: "Mara", LastName: "Muster"},
			editedBy:   23,
			wantEditor: "Mara Muster",
		},
		{
			name:       "username fallback",
			claims:     sessiontoken.AppClaims{ID: 23, Username: "mara.muster"},
			editedBy:   23,
			wantEditor: "mara.muster",
		},
		{
			// The owner stores "Unbekannt" for an unnamed editor; the port
			// deliberately hands it nothing rather than the wrong name.
			name:       "mismatched actor",
			claims:     sessiontoken.AppClaims{ID: 99, FirstName: "Andere", LastName: "Person"},
			editedBy:   23,
			wantEditor: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			capability := &stubStudentAuditCapability{}
			service := api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAuditFor(capability))
			ctx := context.WithValue(context.Background(), sessiontoken.CtxClaims, tc.claims)

			err := service.RecordChangesForActor(
				ctx,
				&peopleModule.StudentState{HealthInfo: auditPortStrPtr("alt")},
				&peopleModule.StudentState{HealthInfo: auditPortStrPtr("neu")},
				tc.editedBy,
			)

			require.NoError(t, err)
			require.Len(t, capability.recorded, 1)
			assert.Equal(t, tc.editedBy, capability.recorded[0].editedBy)
			assert.Equal(t, tc.wantEditor, capability.recorded[0].editedByName)
		})
	}
}

// A nil snapshot is a no-op: the retained callers pass one when they have no
// "before" row to compare against.
func TestStudentAuditCompositionIgnoresMissingSnapshots(t *testing.T) {
	t.Parallel()

	capability := &stubStudentAuditCapability{}
	service := api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAuditFor(capability))

	require.NoError(t, service.RecordChanges(context.Background(), nil, &peopleModule.StudentState{}, 1, "Wer"))
	require.NoError(t, service.RecordChanges(context.Background(), &peopleModule.StudentState{}, nil, 1, "Wer"))
	assert.Empty(t, capability.recorded)
}

func auditPortStrPtr(s string) *string { return &s }
