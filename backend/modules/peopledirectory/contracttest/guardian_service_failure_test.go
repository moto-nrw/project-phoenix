// The error branches of the guardian service that a real database cannot
// reach deterministically (forced repository failures). These are regression
// tests: each one fails if the corresponding guard is removed or weakened.
// The unexported helpers are covered in the compose package (#3755).
package contracttest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddGuardianPhone_CreateFailureRollsBackPrimary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "phone-create-rollback")
	service := setupGuardianService(t, db)
	first, err := testpkg.TenantQuery2(t, ctx, db, service.AddPhoneNumber, guardian.ID, peoplePorts.PhoneNumberCreateRequest{
		PhoneNumber: "0171 1234567", PhoneType: "mobile",
	})
	require.NoError(t, err)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	failure := errors.New("phone insert failed")
	failedService := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
		GuardianProfileRepo: repos.GuardianProfile,
		GuardianPhoneNumberRepo: failingPhoneCreate{
			GuardianPhoneNumberRepository: repos.GuardianPhoneNumber,
			failure:                       failure,
		},
	}, db)
	_, err = testpkg.TenantQuery2(t, ctx, db, failedService.AddPhoneNumber, guardian.ID, peoplePorts.PhoneNumberCreateRequest{
		PhoneNumber: "0171 7654321", PhoneType: "work", IsPrimary: true,
	})
	require.ErrorIs(t, err, failure)
	phones, err := testpkg.TenantQuery1(t, ctx, db, service.GuardianPhoneNumbers().FindByGuardianID, guardian.ID)
	require.NoError(t, err)
	require.Equal(t, []*peopleValues.GuardianProfilePhoneSnapshot{first}, phones, "the failed add must roll back unsetting the existing primary")
}

type failingPhoneCreate struct {
	peoplePorts.GuardianPhoneNumberRepository
	failure error
}

func (r failingPhoneCreate) Create(context.Context, *peopleValues.GuardianProfilePhoneSnapshot) error {
	return r.failure
}

func guardianAmbientTxContext() context.Context {
	return tenant.WithTransactionForTest(context.Background(), struct{}{})
}

// A fresh create fact replaces only the guardian existence read. Missing,
// reused or differently scoped facts keep the guardian-before-child error.
func TestLinkGuardian_CreatedProfileFactPreservesLookupFallback(t *testing.T) {
	t.Parallel()
	const guardianID, studentID, schoolID int64 = 21, 22, 23
	ctx := tenant.WithTenantID(guardianAmbientTxContext(), schoolID)
	created := peopleValues.Guardian{ID: guardianID, TenantID: schoolID}
	reused, foreign, mismatched := created, created, created
	reused.ReusedExisting = true
	foreign.TenantID++
	mismatched.ID++
	guardianFailure := errors.New("guardian lookup failed")
	studentFailure := errors.New("student lookup failed")
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		created *peopleValues.Guardian
		want    []string
		err     error
	}{
		{"new in transaction", ctx, &created, []string{"student"}, studentFailure},
		{"no fact", ctx, nil, []string{"guardian"}, guardianFailure},
		{"reused profile", ctx, &reused, []string{"guardian"}, guardianFailure},
		{"other tenant", ctx, &foreign, []string{"guardian"}, guardianFailure},
		{"other profile", ctx, &mismatched, []string{"guardian"}, guardianFailure},
		{"no transaction", tenant.WithTenantID(context.Background(), schoolID), &created, []string{"guardian"}, guardianFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var reads []string
			service := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
				GuardianProfileRepo: &fakeProfileRepo{findByIDFn: func(_ context.Context, id int64) (*peopleValues.GuardianProfileSnapshot, error) {
					require.Equal(t, guardianID, id)
					reads = append(reads, "guardian")
					return nil, guardianFailure
				}},
				StudentRepo: &fakeStudentRepo{findByIDFn: func(_ context.Context, id interface{}) (*peopleValues.StudentState, error) {
					require.Equal(t, studentID, id)
					reads = append(reads, "student")
					return nil, studentFailure
				}},
			}, nil)
			link, err := service.LinkGuardianToStudent(tc.ctx, peoplePorts.StudentGuardianCreateRequest{
				StudentID: studentID, GuardianProfileID: guardianID, CreatedGuardian: tc.created,
			})
			require.ErrorIs(t, err, tc.err)
			require.ErrorIs(t, err, peopleValues.ErrGuardianNotFound)
			require.Nil(t, link)
			require.Equal(t, tc.want, reads)
		})
	}
}

// --- fakes -----------------------------------------------------------------
//
// Each fake embeds the full repository interface so it satisfies the type;
// only the methods a given test exercises are overridden. Any un-overridden
// method panics (nil interface) — which is exactly what we want, since calling
// one would mean the code under test took an unexpected path.

type fakeProfileRepo struct {
	peoplePorts.GuardianProfileRepository
	findByEmailFn func(ctx context.Context, email string) (*peopleValues.GuardianProfileSnapshot, error)
	createFn      func(ctx context.Context, p *peopleValues.GuardianProfileSnapshot) error
	findByIDFn    func(ctx context.Context, id int64) (*peopleValues.GuardianProfileSnapshot, error)
	updateFn      func(ctx context.Context, p *peopleValues.GuardianProfileSnapshot) error
	lockFn        func(ctx context.Context, id int64) error
	deleteFn      func(ctx context.Context, id int64) error
}

func (f *fakeProfileRepo) FindByEmail(ctx context.Context, email string) (*peopleValues.GuardianProfileSnapshot, error) {
	return f.findByEmailFn(ctx, email)
}
func (f *fakeProfileRepo) Create(ctx context.Context, p *peopleValues.GuardianProfileSnapshot) error {
	return f.createFn(ctx, p)
}
func (f *fakeProfileRepo) FindByID(ctx context.Context, id int64) (*peopleValues.GuardianProfileSnapshot, error) {
	return f.findByIDFn(ctx, id)
}

// The combined port can fail while acquiring the lock or reading the profile.
// Keep both injected failures and their existing ordering assertions.
func (f *fakeProfileRepo) FindByIDForUpdate(ctx context.Context, id int64) (*peopleValues.GuardianProfileSnapshot, error) {
	if err := f.lockFn(ctx, id); err != nil {
		return nil, err
	}
	return f.findByIDFn(ctx, id)
}
func (f *fakeProfileRepo) Update(ctx context.Context, p *peopleValues.GuardianProfileSnapshot) error {
	return f.updateFn(ctx, p)
}
func (f *fakeProfileRepo) LockByIDForUpdate(ctx context.Context, id int64) error {
	return f.lockFn(ctx, id)
}
func (f *fakeProfileRepo) Delete(ctx context.Context, id int64) error {
	return f.deleteFn(ctx, id)
}

type fakeStudentGuardianRepo struct {
	peoplePorts.StudentGuardianRepository
	findByGuardianFn func(ctx context.Context, guardianProfileID int64) ([]*peopleValues.GuardianRelationshipSnapshot, error)
	deleteFn         func(ctx context.Context, id interface{}) error
}

func (f *fakeStudentGuardianRepo) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*peopleValues.GuardianRelationshipSnapshot, error) {
	return f.findByGuardianFn(ctx, guardianProfileID)
}
func (f *fakeStudentGuardianRepo) Delete(ctx context.Context, id interface{}) error {
	return f.deleteFn(ctx, id)
}

type fakeStudentRepo struct {
	peoplePorts.StudentRepository
	findByIDFn           func(ctx context.Context, id interface{}) (*peopleValues.StudentState, error)
	findByIDsFn          func(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	findByIDsForUpdateFn func(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	findByIDForUpdateFn  func(ctx context.Context, id int64) (*peopleValues.StudentState, error)
}

type fakeGuardianFinancialRepo struct {
	peoplePorts.GuardianFinancialDataRepository
	findFn func(ctx context.Context, guardianProfileID int64) (*peopleValues.GuardianPayment, error)
}

func (f *fakeGuardianFinancialRepo) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) (*peopleValues.GuardianPayment, error) {
	return f.findFn(ctx, guardianProfileID)
}

func (f *fakeStudentRepo) FindByID(ctx context.Context, id interface{}) (*peopleValues.StudentState, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakeStudentRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error) {
	return f.findByIDsFn(ctx, ids)
}

func (f *fakeStudentRepo) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error) {
	if f.findByIDsForUpdateFn == nil {
		return map[int64]*peopleValues.StudentState{}, nil
	}
	return f.findByIDsForUpdateFn(ctx, ids)
}

func (f *fakeStudentRepo) FindByIDForUpdate(ctx context.Context, id int64) (*peopleValues.StudentState, error) {
	if f.findByIDForUpdateFn == nil {
		return &peopleValues.StudentState{}, nil
	}
	return f.findByIDForUpdateFn(ctx, id)
}

type fakePersonRepo struct {
	peoplePorts.PersonRepository
	findByIDFn  func(ctx context.Context, id interface{}) (*peopleValues.DirectoryPersonSnapshot, error)
	findByIDsFn func(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
}

func (f *fakePersonRepo) FindByID(ctx context.Context, id interface{}) (*peopleValues.DirectoryPersonSnapshot, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakePersonRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error) {
	return f.findByIDsFn(ctx, ids)
}

// --- forced-failure service branches ---------------------------------------

func TestCreateGuardian_NonUniqueCreateErrorIsWrapped(t *testing.T) {
	t.Parallel()

	email := "new-guardian@example.com"
	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{

		// Pre-check finds no existing guardian, so creation proceeds.
		GuardianProfileRepo: &fakeProfileRepo{

			findByEmailFn: func(_ context.Context, _ string) (*peopleValues.GuardianProfileSnapshot, error) {
				return nil, peopleValues.ErrGuardianProfileNotFound
			},
			// Insert fails with a NON-unique error: must surface as a wrapped
			// internal error, NOT be misclassified as a duplicate-email 400.
			createFn: func(_ context.Context, _ *peopleValues.GuardianProfileSnapshot) error {
				return errors.New("connection refused")
			},
		}}, nil)

	_, err := svc.CreateGuardian(context.Background(), peoplePorts.GuardianCreateRequest{
		FirstName: "New", LastName: "Guardian", Email: &email,
	})

	require.Error(t, err)
	var validationErr *userscontract.ValidationError
	assert.False(t, errors.As(err, &validationErr),
		"a non-unique DB failure must not be classified as a validation (400) error")
	assert.Contains(t, err.Error(), "failed to create guardian profile")
}

func TestUpdateGuardian_NonUniqueUpdateErrorIsReturned(t *testing.T) {
	t.Parallel()

	email := "edited@example.com"
	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{

		// UpdateGuardian now locks the profile row FOR UPDATE before reading
		// it (serializes with the parents-portal contact path — #1667 review);
		// the fake returns a no-op lock so this branch reaches the Update.
		GuardianProfileRepo: &fakeProfileRepo{

			lockFn: func(_ context.Context, _ int64) error { return nil },
			findByIDFn: func(_ context.Context, _ int64) (*peopleValues.GuardianProfileSnapshot, error) {
				return &peopleValues.GuardianProfileSnapshot{}, nil
			},
			// No other guardian owns the email → pre-check passes.
			findByEmailFn: func(_ context.Context, _ string) (*peopleValues.GuardianProfileSnapshot, error) {
				return nil, peopleValues.ErrGuardianProfileNotFound
			},
			updateFn: func(_ context.Context, _ *peopleValues.GuardianProfileSnapshot) error {
				return errors.New("connection refused")
			},
		}}, nil)

	err := svc.UpdateGuardian(context.Background(), 1, peoplePorts.GuardianCreateRequest{
		FirstName: "Edited", LastName: "Name", Email: &email,
	})

	require.Error(t, err)
	var validationErr *userscontract.ValidationError
	assert.False(t, errors.As(err, &validationErr),
		"a non-unique update failure must not be classified as a 400")
}

func TestDeleteGuardianWithLinks_LockError(t *testing.T) {
	t.Parallel()

	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{GuardianProfileRepo: &fakeProfileRepo{
		lockFn: func(_ context.Context, _ int64) error { return errors.New("lock timeout") },
	}}, nil)
	err := svc.DeleteGuardianWithLinks(guardianAmbientTxContext(), 1, []int64{1}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to lock guardian profile")
}

// TestDeleteGuardian_WithoutFinancialRepoRefuses pins that a service wired
// without the financial-data repository fails the delete with a clear error
// instead of dereferencing nil (review round 10).
func TestDeleteGuardian_WithoutFinancialRepoRefuses(t *testing.T) {
	t.Parallel()

	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{GuardianProfileRepo: &fakeProfileRepo{
		lockFn: func(_ context.Context, _ int64) error { return nil },
	}}, nil)
	for name, run := range map[string]func() error{
		"DeleteGuardian":          func() error { return svc.DeleteGuardian(guardianAmbientTxContext(), 1, 1) },
		"DeleteGuardianWithLinks": func() error { return svc.DeleteGuardianWithLinks(guardianAmbientTxContext(), 1, []int64{1}, 1) },
	} {
		t.Run(name, func(t *testing.T) {
			require.NotPanics(t, func() {
				err := run()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "guardian financial repository is not wired")
			})
		})
	}
}

func TestDeleteGuardianWithLinks_LoadLinksError(t *testing.T) {
	t.Parallel()

	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{GuardianProfileRepo: &fakeProfileRepo{
		lockFn: func(_ context.Context, _ int64) error { return nil },
	}, StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*peopleValues.GuardianRelationshipSnapshot, error) {
			return nil, errors.New("query failed")
		},
	}, GuardianFinancialRepo: &fakeGuardianFinancialRepo{
		findFn: func(_ context.Context, _ int64) (*peopleValues.GuardianPayment, error) { return nil, nil },
	}}, nil)
	err := svc.DeleteGuardianWithLinks(guardianAmbientTxContext(), 1, []int64{1}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load guardian links")
}

func TestDeleteGuardianWithLinks_LinkDeleteError(t *testing.T) {
	t.Parallel()

	link := &peopleValues.GuardianRelationshipSnapshot{StudentID: 7, GuardianProfileID: 1}
	link.ID = 5
	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{GuardianProfileRepo: &fakeProfileRepo{
		lockFn: func(_ context.Context, _ int64) error { return nil },
	}, StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*peopleValues.GuardianRelationshipSnapshot, error) {
			return []*peopleValues.GuardianRelationshipSnapshot{link}, nil
		},
		deleteFn: func(_ context.Context, _ interface{}) error {
			return errors.New("delete failed")
		},
	}, StudentRepo: &fakeStudentRepo{
		findByIDsForUpdateFn: func(_ context.Context, _ []int64) (map[int64]*peopleValues.StudentState, error) {
			return map[int64]*peopleValues.StudentState{7: {}}, nil
		},
	}, GuardianFinancialRepo: &fakeGuardianFinancialRepo{
		findFn: func(_ context.Context, _ int64) (*peopleValues.GuardianPayment, error) { return nil, nil },
	}}, nil)
	// expectedLinkIDs matches the current set, so the stale-preview guard passes
	// and execution reaches the link-deletion loop, which then fails.
	err := svc.DeleteGuardianWithLinks(guardianAmbientTxContext(), 1, []int64{5}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove guardian link")
}

func TestGetGuardianDeleteImpact_StudentLoadError(t *testing.T) {
	t.Parallel()

	rel := &peopleValues.GuardianRelationshipSnapshot{StudentID: 7, GuardianProfileID: 1}
	rel.ID = 5
	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*peopleValues.GuardianRelationshipSnapshot, error) {
			return []*peopleValues.GuardianRelationshipSnapshot{rel}, nil
		},
	}, StudentRepo: &fakeStudentRepo{
		findByIDsFn: func(_ context.Context, _ []int64) (map[int64]*peopleValues.StudentState, error) {
			return nil, errors.New("student gone")
		},
	}}, nil)
	_, err := svc.GetGuardianDeleteImpact(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load student")
}

func TestGetGuardianDeleteImpact_NilPersonIsRejected(t *testing.T) {
	t.Parallel()

	rel := &peopleValues.GuardianRelationshipSnapshot{StudentID: 7, GuardianProfileID: 1}
	rel.ID = 5
	student := &peopleValues.StudentState{PersonID: 9}
	svc := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*peopleValues.GuardianRelationshipSnapshot, error) {
			return []*peopleValues.GuardianRelationshipSnapshot{rel}, nil
		},
	}, StudentRepo: &fakeStudentRepo{
		findByIDsFn: func(_ context.Context, _ []int64) (map[int64]*peopleValues.StudentState, error) {
			return map[int64]*peopleValues.StudentState{7: student}, nil
		},
	}, PersonRepo: &fakePersonRepo{
		// A missing row surfaces as an absent/nil map entry — the impact
		// builder must treat that as a hard error, not a nil-pointer panic.
		findByIDsFn: func(_ context.Context, _ []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error) {
			return map[int64]*peopleValues.DirectoryPersonSnapshot{9: nil}, nil
		},
	}}, nil)
	_, err := svc.GetGuardianDeleteImpact(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "person record")
}
