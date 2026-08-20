// Package users internal tests cover the unexported helpers and the error
// branches of the guardian service that a real database cannot reach
// deterministically (forced repository failures). These are regression tests:
// each one fails if the corresponding guard is removed or weakened.
package users

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes -----------------------------------------------------------------
//
// Each fake embeds the full repository interface so it satisfies the type;
// only the methods a given test exercises are overridden. Any un-overridden
// method panics (nil interface) — which is exactly what we want, since calling
// one would mean the code under test took an unexpected path.

type fakeProfileRepo struct {
	users.GuardianProfileRepository
	findByEmailFn func(ctx context.Context, email string) (*users.GuardianProfile, error)
	createFn      func(ctx context.Context, p *users.GuardianProfile) error
	findByIDFn    func(ctx context.Context, id int64) (*users.GuardianProfile, error)
	updateFn      func(ctx context.Context, p *users.GuardianProfile) error
	lockFn        func(ctx context.Context, id int64) error
	deleteFn      func(ctx context.Context, id int64) error
}

func (f *fakeProfileRepo) FindByEmail(ctx context.Context, email string) (*users.GuardianProfile, error) {
	return f.findByEmailFn(ctx, email)
}
func (f *fakeProfileRepo) Create(ctx context.Context, p *users.GuardianProfile) error {
	return f.createFn(ctx, p)
}
func (f *fakeProfileRepo) FindByID(ctx context.Context, id int64) (*users.GuardianProfile, error) {
	return f.findByIDFn(ctx, id)
}
func (f *fakeProfileRepo) Update(ctx context.Context, p *users.GuardianProfile) error {
	return f.updateFn(ctx, p)
}
func (f *fakeProfileRepo) LockByIDForUpdate(ctx context.Context, id int64) error {
	return f.lockFn(ctx, id)
}
func (f *fakeProfileRepo) Delete(ctx context.Context, id int64) error {
	return f.deleteFn(ctx, id)
}

type fakeStudentGuardianRepo struct {
	users.StudentGuardianRepository
	findByGuardianFn func(ctx context.Context, guardianProfileID int64) ([]*users.StudentGuardian, error)
	deleteFn         func(ctx context.Context, id interface{}) error
}

func (f *fakeStudentGuardianRepo) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*users.StudentGuardian, error) {
	return f.findByGuardianFn(ctx, guardianProfileID)
}
func (f *fakeStudentGuardianRepo) Delete(ctx context.Context, id interface{}) error {
	return f.deleteFn(ctx, id)
}

type fakeStudentRepo struct {
	users.StudentRepository
	findByIDFn  func(ctx context.Context, id interface{}) (*users.Student, error)
	findByIDsFn func(ctx context.Context, ids []int64) (map[int64]*users.Student, error)
}

func (f *fakeStudentRepo) FindByID(ctx context.Context, id interface{}) (*users.Student, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakeStudentRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Student, error) {
	return f.findByIDsFn(ctx, ids)
}

type fakePersonRepo struct {
	users.PersonRepository
	findByIDFn  func(ctx context.Context, id interface{}) (*users.Person, error)
	findByIDsFn func(ctx context.Context, ids []int64) (map[int64]*users.Person, error)
}

func (f *fakePersonRepo) FindByID(ctx context.Context, id interface{}) (*users.Person, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakePersonRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error) {
	return f.findByIDsFn(ctx, ids)
}

// --- unexported helpers ----------------------------------------------------

func TestSameInt64Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b []int64
		want bool
	}{
		{"equal same order", []int64{1, 2, 3}, []int64{1, 2, 3}, true},
		{"equal different order", []int64{3, 1, 2}, []int64{1, 2, 3}, true},
		{"both empty", []int64{}, []int64{}, true},
		{"different length", []int64{1, 2}, []int64{1, 2, 3}, false},
		{"same length different elements", []int64{1, 2, 3}, []int64{1, 2, 4}, false},
		{"duplicates differ", []int64{1, 1, 2}, []int64{1, 2, 2}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sameInt64Set(tc.a, tc.b))
		})
	}
}

func TestIsGuardianUniqueViolation(t *testing.T) {
	t.Parallel()

	t.Run("nil error is not a violation", func(t *testing.T) {
		assert.False(t, base.IsUniqueViolation(nil))
	})
	t.Run("plain error is not a violation", func(t *testing.T) {
		assert.False(t, base.IsUniqueViolation(errors.New("boom")))
	})
	t.Run("wrapped non-pg DatabaseError is not a violation", func(t *testing.T) {
		// Exercises the DatabaseError-unwrap branch: it must unwrap and then
		// still report false because the inner error is not a pg integrity error.
		err := &base.DatabaseError{Op: "create", Err: errors.New("connection reset")}
		assert.False(t, base.IsUniqueViolation(err))
	})
}

// --- forced-failure service branches ---------------------------------------

func TestCreateGuardian_NonUniqueCreateErrorIsWrapped(t *testing.T) {
	t.Parallel()

	email := "new-guardian@example.com"
	svc := &GuardianService{GuardianServiceDependencies: GuardianServiceDependencies{

		// Pre-check finds no existing guardian, so creation proceeds.
		GuardianProfileRepo: &fakeProfileRepo{

			findByEmailFn: func(_ context.Context, _ string) (*users.GuardianProfile, error) {
				return nil, users.ErrGuardianProfileNotFound
			},
			// Insert fails with a NON-unique error: must surface as a wrapped
			// internal error, NOT be misclassified as a duplicate-email 400.
			createFn: func(_ context.Context, _ *users.GuardianProfile) error {
				return errors.New("connection refused")
			},
		}},
	}

	_, err := svc.CreateGuardian(context.Background(), GuardianCreateRequest{
		FirstName: "New", LastName: "Guardian", Email: &email,
	})

	require.Error(t, err)
	var validationErr *ValidationError
	assert.False(t, errors.As(err, &validationErr),
		"a non-unique DB failure must not be classified as a validation (400) error")
	assert.Contains(t, err.Error(), "failed to create guardian profile")
}

func TestUpdateGuardian_NonUniqueUpdateErrorIsReturned(t *testing.T) {
	t.Parallel()

	email := "edited@example.com"
	svc := &GuardianService{GuardianServiceDependencies: GuardianServiceDependencies{

		// UpdateGuardian now locks the profile row FOR UPDATE before reading
		// it (serializes with the parents-portal contact path — #1667 review);
		// the fake returns a no-op lock so this branch reaches the Update.
		GuardianProfileRepo: &fakeProfileRepo{

			lockFn: func(_ context.Context, _ int64) error { return nil },
			findByIDFn: func(_ context.Context, _ int64) (*users.GuardianProfile, error) {
				return &users.GuardianProfile{}, nil
			},
			// No other guardian owns the email → pre-check passes.
			findByEmailFn: func(_ context.Context, _ string) (*users.GuardianProfile, error) {
				return nil, users.ErrGuardianProfileNotFound
			},
			updateFn: func(_ context.Context, _ *users.GuardianProfile) error {
				return errors.New("connection refused")
			},
		}},
	}

	err := svc.UpdateGuardian(context.Background(), 1, GuardianCreateRequest{
		FirstName: "Edited", LastName: "Name", Email: &email,
	})

	require.Error(t, err)
	var validationErr *ValidationError
	assert.False(t, errors.As(err, &validationErr),
		"a non-unique update failure must not be classified as a 400")
}

func TestDeleteGuardianWithLinks_LockError(t *testing.T) {
	t.Parallel()

	svc := &GuardianService{GuardianServiceDependencies: GuardianServiceDependencies{GuardianProfileRepo: &fakeProfileRepo{
		lockFn: func(_ context.Context, _ int64) error { return errors.New("lock timeout") },
	}},
	}
	err := svc.DeleteGuardianWithLinks(context.Background(), 1, []int64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to lock guardian profile")
}

func TestDeleteGuardianWithLinks_LoadLinksError(t *testing.T) {
	t.Parallel()

	svc := &GuardianService{GuardianServiceDependencies: GuardianServiceDependencies{GuardianProfileRepo: &fakeProfileRepo{
		lockFn: func(_ context.Context, _ int64) error { return nil },
	}, StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*users.StudentGuardian, error) {
			return nil, errors.New("query failed")
		},
	}},
	}
	err := svc.DeleteGuardianWithLinks(context.Background(), 1, []int64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load guardian links")
}

func TestDeleteGuardianWithLinks_LinkDeleteError(t *testing.T) {
	t.Parallel()

	link := &users.StudentGuardian{StudentID: 7, GuardianProfileID: 1}
	link.ID = 5
	svc := &GuardianService{GuardianServiceDependencies: GuardianServiceDependencies{GuardianProfileRepo: &fakeProfileRepo{
		lockFn: func(_ context.Context, _ int64) error { return nil },
	}, StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*users.StudentGuardian, error) {
			return []*users.StudentGuardian{link}, nil
		},
		deleteFn: func(_ context.Context, _ interface{}) error {
			return errors.New("delete failed")
		},
	}},
	}
	// expectedLinkIDs matches the current set, so the stale-preview guard passes
	// and execution reaches the link-deletion loop, which then fails.
	err := svc.DeleteGuardianWithLinks(context.Background(), 1, []int64{5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove guardian link")
}

func TestGetGuardianDeleteImpact_StudentLoadError(t *testing.T) {
	t.Parallel()

	rel := &users.StudentGuardian{StudentID: 7, GuardianProfileID: 1}
	rel.ID = 5
	svc := &GuardianService{GuardianServiceDependencies: GuardianServiceDependencies{StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*users.StudentGuardian, error) {
			return []*users.StudentGuardian{rel}, nil
		},
	}, StudentRepo: &fakeStudentRepo{
		findByIDsFn: func(_ context.Context, _ []int64) (map[int64]*users.Student, error) {
			return nil, errors.New("student gone")
		},
	}},
	}
	_, err := svc.GetGuardianDeleteImpact(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load student")
}

func TestGetGuardianDeleteImpact_NilPersonIsRejected(t *testing.T) {
	t.Parallel()

	rel := &users.StudentGuardian{StudentID: 7, GuardianProfileID: 1}
	rel.ID = 5
	student := &users.Student{PersonID: 9}
	svc := &GuardianService{GuardianServiceDependencies: GuardianServiceDependencies{StudentGuardianRepo: &fakeStudentGuardianRepo{
		findByGuardianFn: func(_ context.Context, _ int64) ([]*users.StudentGuardian, error) {
			return []*users.StudentGuardian{rel}, nil
		},
	}, StudentRepo: &fakeStudentRepo{
		findByIDsFn: func(_ context.Context, _ []int64) (map[int64]*users.Student, error) {
			return map[int64]*users.Student{7: student}, nil
		},
	}, PersonRepo: &fakePersonRepo{
		// A missing row surfaces as an absent/nil map entry — the impact
		// builder must treat that as a hard error, not a nil-pointer panic.
		findByIDsFn: func(_ context.Context, _ []int64) (map[int64]*users.Person, error) {
			return map[int64]*users.Person{9: nil}, nil
		},
	}},
	}
	_, err := svc.GetGuardianDeleteImpact(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "person record")
}
