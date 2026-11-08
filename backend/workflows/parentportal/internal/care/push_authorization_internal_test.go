package care

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type documentPushAuthorizationRuntime struct{ pickupSignatureRuntimeStub }

func (documentPushAuthorizationRuntime) WithinTraeger(ctx context.Context, _ int64, visit func(context.Context) error) error {
	return visit(ctx)
}

type documentPushAuthorizationSchools map[int64]Traeger

func (s documentPushAuthorizationSchools) TraegerOfSchools(context.Context, []int64) (map[int64]Traeger, error) {
	return s, nil
}

func TestDocumentPushAuthorizationEnforcesSchoolAndStudentStatus(t *testing.T) {
	t.Parallel()
	school, otherSchool := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	account, student, object := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	for _, scenario := range []struct {
		name                              string
		studentSchool, relationshipSchool int64
		status                            string
		allowed                           bool
	}{
		{"matching school and active student", school, school, "active", true},
		{"foreign student school with matching relationship", otherSchool, school, "active", false},
		{"foreign relationship school with matching student", school, otherSchool, "active", false},
		{"alumnus with matching school and relationship", school, school, "alumnus", false},
	} {
		for _, kind := range []struct{ name, value string }{
			{"billing", DocumentPushBilling}, {"contract", DocumentPushContract}, {"release", DocumentPushRelease},
		} {
			t.Run(scenario.name+"/"+kind.name, func(t *testing.T) {
				t.Parallel()
				// The same account and child occur in every projection. Only the tested
				// school/status differs, so no other filter or RLS can mask its removal.
				authorization := documentPushAuthorization(t, school, account, student,
					DocumentPushStudent{TenantID: scenario.studentSchool, Status: scenario.status},
					DocumentPushRelationship{TenantID: scenario.relationshipSchool, AccountID: account, StudentID: student,
						Permissions: []string{GuardianPermissionPortalAccess, GuardianPermissionEnrollmentsView}})
				decisions, err := authorization.AuthorizeDocumentPushes(context.Background(), []DocumentPush{
					{Key: object, TenantID: school, AccountID: account, ObjectID: object, Kind: kind.value, StudentIDs: []int64{student}},
				})
				require.NoError(t, err)
				require.Equal(t, scenario.allowed, decisions[object], "only a current child and relationship at the queued school authorize dispatch")
			})
		}
	}
}

func documentPushAuthorization(t *testing.T, school, account, student int64, child DocumentPushStudent, relationship DocumentPushRelationship) *DocumentPushAuthorization {
	t.Helper()
	// Read-port snapshots deliberately disagree in the foreign-school cases.
	// The object is valid: an owner rejection must not hide a workflow defect.
	validObjects := func(_ context.Context, _ int64, candidates []DocumentPushCheck) (map[int64]bool, error) {
		allowed := make(map[int64]bool, len(candidates))
		for _, candidate := range candidates {
			allowed[candidate.Key] = true
		}
		return allowed, nil
	}
	runtime := documentPushAuthorizationRuntime{}
	return NewDocumentPushAuthorization(DocumentPushAuthorizationDependencies{
		Runtime: runtime, TraegerRuntime: runtime,
		Schools: documentPushAuthorizationSchools{school: {ID: testpkg.UniqueTestTenantID(t), Name: "Test Träger"}},
		Accounts: func(context.Context, []int64) (map[int64]DocumentPushAccount, error) {
			return map[int64]DocumentPushAccount{account: {ID: account, Email: "parent@example.org"}}, nil
		},
		Relationships: func(context.Context, []int64) ([]DocumentPushRelationship, error) {
			return []DocumentPushRelationship{relationship}, nil
		},
		Students: func(context.Context, []int64) (map[int64]DocumentPushStudent, error) {
			return map[int64]DocumentPushStudent{student: child}, nil
		},
		Billing: validObjects, Contracts: validObjects, Releases: validObjects,
	})
}
