package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var _ CaregiverCapabilityService = (*stubCaregiverCapabilityService)(nil)

// stubCaregiverCapabilityService is a func-field double of People
// Directory's caregiver capability; nil fields answer the zero value.
type stubCaregiverCapabilityService struct {
	GetFn     func(context.Context, int64) (*userModels.CaregiverCapabilityState, error)
	EnableFn  func(context.Context, int64, userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error)
	DisableFn func(context.Context, int64) (*userModels.CaregiverCapabilityState, error)
}

func (s *stubCaregiverCapabilityService) GetCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
	if s.GetFn != nil {
		return s.GetFn(ctx, accountID)
	}
	return nil, nil
}

func (s *stubCaregiverCapabilityService) EnableCaregiverCapability(ctx context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error) {
	if s.EnableFn != nil {
		return s.EnableFn(ctx, accountID, input)
	}
	return nil, nil
}

func (s *stubCaregiverCapabilityService) DisableCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
	if s.DisableFn != nil {
		return s.DisableFn(ctx, accountID)
	}
	return nil, nil
}

// adminRuntimeRecorder is a unit of work whose administrative transaction
// hands out a placeholder bun.Tx and records what the callback returned, so
// a test can tell the call ran inside it and that its failure reached the
// transaction (which rolls it back).
type adminRuntimeRecorder struct {
	calls int
	err   error
}

func (r *adminRuntimeRecorder) context(t *testing.T) context.Context {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(context.Context, int64, func(context.Context, any) error) error {
			return errors.New("tenant transaction is not available in this test")
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			r.calls++
			r.err = fn(ctx, bun.Tx{})
			return r.err
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return tenant.WithUnitOfWork(context.Background(), runtime)
}

func assertInSchoolAdminTx(t *testing.T, ctx context.Context, schoolID int64) {
	t.Helper()
	assert.Equal(t, schoolID, tenant.FromContext(ctx))
	assert.True(t, tenant.IsAdminTx(ctx))
	tx, ok := tenant.TransactionFromContext(ctx)
	require.True(t, ok)
	require.NotNil(t, tx)
}

func TestCaregiverCapabilityViewsRenderStateAsJSON(t *testing.T) {
	t.Parallel()

	personID := int64(56)
	state := &userModels.CaregiverCapabilityState{
		AccountID:   34,
		Email:       "ada@example.com",
		FirstName:   "Ada",
		LastName:    "Lovelace",
		PersonID:    &personID,
		HasUserRole: true,
	}
	views := CaregiverCapabilityViews{service: &stubCaregiverCapabilityService{
		GetFn: func(_ context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
			assert.Equal(t, int64(34), accountID)
			return state, nil
		},
	}}

	view, err := views.GetCaregiverCapability(context.Background(), 34)
	require.NoError(t, err)

	want, err := json.Marshal(state)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(view))
	assert.Contains(t, string(view), `"account_id":34`)
	assert.Contains(t, string(view), `"person_id":56`)
	assert.Contains(t, string(view), `"has_user_role":true`)
}

func TestCaregiverCapabilityViewsEnablePassesProfileInput(t *testing.T) {
	t.Parallel()

	views := CaregiverCapabilityViews{service: &stubCaregiverCapabilityService{
		EnableFn: func(_ context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error) {
			assert.Equal(t, int64(34), accountID)
			assert.Equal(t, userModels.EnableCaregiverCapabilityInput{
				FirstName: "Ada",
				LastName:  "Lovelace",
				Position:  "Springer",
			}, input)
			return &userModels.CaregiverCapabilityState{AccountID: accountID, HasUserRole: true}, nil
		},
	}}

	view, err := views.EnableCaregiverCapability(context.Background(), 34, "Ada", "Lovelace", "Springer")
	require.NoError(t, err)
	assert.Contains(t, string(view), `"has_user_role":true`)
}

func TestCaregiverCapabilityViewsSchoolAccountRunsInSchoolAdminTx(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		recorder := &adminRuntimeRecorder{}
		views := CaregiverCapabilityViews{service: &stubCaregiverCapabilityService{
			GetFn: func(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
				assertInSchoolAdminTx(t, ctx, 12)
				assert.Equal(t, int64(34), accountID)
				return &userModels.CaregiverCapabilityState{AccountID: accountID, HasUserRole: true}, nil
			},
		}}

		view, err := views.GetSchoolAccountCaregiverCapability(recorder.context(t), 12, 34)
		require.NoError(t, err)
		assert.Contains(t, string(view), `"account_id":34`)
		assert.Equal(t, 1, recorder.calls)
		assert.NoError(t, recorder.err)
	})

	t.Run("enable", func(t *testing.T) {
		recorder := &adminRuntimeRecorder{}
		views := CaregiverCapabilityViews{service: &stubCaregiverCapabilityService{
			EnableFn: func(ctx context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error) {
				assertInSchoolAdminTx(t, ctx, 12)
				assert.Equal(t, int64(34), accountID)
				assert.Equal(t, userModels.EnableCaregiverCapabilityInput{
					FirstName: "Ada",
					LastName:  "Lovelace",
					Position:  "Springer",
				}, input)
				return &userModels.CaregiverCapabilityState{AccountID: accountID, HasUserRole: true}, nil
			},
		}}

		view, err := views.EnableSchoolAccountCaregiverCapability(recorder.context(t), 12, 34, "Ada", "Lovelace", "Springer")
		require.NoError(t, err)
		assert.Contains(t, string(view), `"has_user_role":true`)
		assert.Equal(t, 1, recorder.calls)
		assert.NoError(t, recorder.err)
	})

	t.Run("disable failure fails the transaction", func(t *testing.T) {
		recorder := &adminRuntimeRecorder{}
		views := CaregiverCapabilityViews{service: &stubCaregiverCapabilityService{
			DisableFn: func(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
				assertInSchoolAdminTx(t, ctx, 12)
				assert.Equal(t, int64(34), accountID)
				return nil, &CaregiverCapabilityBlockedError{
					Reasons: []userModels.CaregiverCapabilityBlockerCode{
						userModels.CaregiverCapabilityBlockerActiveGroupSupervisions,
					},
				}
			},
		}}

		view, err := views.DisableSchoolAccountCaregiverCapability(recorder.context(t), 12, 34)
		require.Error(t, err)
		assert.Nil(t, view)
		assert.Equal(t, 1, recorder.calls)
		require.Error(t, recorder.err, "the admin transaction must see the failure and roll back")
		assert.Equal(t, err.Error(), recorder.err.Error())
		_, blockedInTx := errors.AsType[caregiverBlockersBehaviour](recorder.err)
		assert.True(t, blockedInTx)

		blocked, ok := errors.AsType[caregiverBlockersBehaviour](err)
		require.True(t, ok)
		assert.Equal(t, []string{"active_group_supervisions"}, blocked.CaregiverCapabilityBlockers())
	})
}

type (
	caregiverBlockersBehaviour interface {
		error
		CaregiverCapabilityBlockers() []string
	}
	caregiverMissingBehaviour interface {
		error
		CaregiverAccountMissing() bool
	}
	caregiverInvalidBehaviour interface {
		error
		CaregiverRequestInvalid() error
	}
	caregiverFailureBehaviour interface {
		error
		CaregiverFailure() error
	}
)

func assertNoOtherCaregiverBehaviour(t *testing.T, err error, keep string) {
	t.Helper()
	if keep != "blocked" {
		_, ok := errors.AsType[caregiverBlockersBehaviour](err)
		assert.False(t, ok, "unexpected blocked behaviour")
	}
	if keep != "missing" {
		_, ok := errors.AsType[caregiverMissingBehaviour](err)
		assert.False(t, ok, "unexpected missing behaviour")
	}
	if keep != "invalid" {
		_, ok := errors.AsType[caregiverInvalidBehaviour](err)
		assert.False(t, ok, "unexpected invalid behaviour")
	}
	if keep != "failure" {
		_, ok := errors.AsType[caregiverFailureBehaviour](err)
		assert.False(t, ok, "unexpected failure behaviour")
	}
}

func TestCaregiverCapabilityErrorClassification(t *testing.T) {
	t.Parallel()

	t.Run("blocked keeps the blocked text and lists the reasons", func(t *testing.T) {
		blockedErr := &CaregiverCapabilityBlockedError{
			Reasons: []userModels.CaregiverCapabilityBlockerCode{
				userModels.CaregiverCapabilityBlockerActiveGroupSupervisions,
				userModels.CaregiverCapabilityBlockerGroupAssignments,
			},
		}
		err := caregiverCapabilityError(fmt.Errorf("disable caregiver capability: %w", blockedErr))

		blocked, ok := errors.AsType[caregiverBlockersBehaviour](err)
		require.True(t, ok)
		assert.Equal(t, blockedErr.Error(), err.Error(), "the routes render the blocked error's own text")
		assert.Equal(t, []string{"active_group_supervisions", "group_assignments"}, blocked.CaregiverCapabilityBlockers())
		chained, ok := errors.AsType[*CaregiverCapabilityBlockedError](err)
		require.True(t, ok)
		assert.Same(t, blockedErr, chained)
		assertNoOtherCaregiverBehaviour(t, err, "blocked")
	})

	t.Run("blocked without reasons reports nil blockers", func(t *testing.T) {
		err := caregiverCapabilityError(&CaregiverCapabilityBlockedError{})

		blocked, ok := errors.AsType[caregiverBlockersBehaviour](err)
		require.True(t, ok)
		assert.Nil(t, blocked.CaregiverCapabilityBlockers())
	})

	t.Run("blocked with empty reasons reports empty blockers", func(t *testing.T) {
		err := caregiverCapabilityError(&CaregiverCapabilityBlockedError{
			Reasons: []userModels.CaregiverCapabilityBlockerCode{},
		})

		blocked, ok := errors.AsType[caregiverBlockersBehaviour](err)
		require.True(t, ok)
		assert.NotNil(t, blocked.CaregiverCapabilityBlockers())
		assert.Empty(t, blocked.CaregiverCapabilityBlockers())
	})

	t.Run("unknown account is missing", func(t *testing.T) {
		source := fmt.Errorf("get caregiver capability: %w", ErrAccountNotFound)
		err := caregiverCapabilityError(source)

		missing, ok := errors.AsType[caregiverMissingBehaviour](err)
		require.True(t, ok)
		assert.True(t, missing.CaregiverAccountMissing())
		assert.Equal(t, source.Error(), err.Error())
		assert.ErrorIs(t, err, ErrAccountNotFound)
		assertNoOtherCaregiverBehaviour(t, err, "missing")
	})

	t.Run("account outside the school is missing", func(t *testing.T) {
		source := &AccountNotAssignedToTenantError{AccountID: 77, TenantID: 44}
		err := caregiverCapabilityError(source)

		missing, ok := errors.AsType[caregiverMissingBehaviour](err)
		require.True(t, ok)
		assert.True(t, missing.CaregiverAccountMissing())
		assert.Equal(t, "account 77 is not assigned to tenant 44", err.Error())
		chained, ok := errors.AsType[*AccountNotAssignedToTenantError](err)
		require.True(t, ok)
		assert.Same(t, source, chained)
		assertNoOtherCaregiverBehaviour(t, err, "missing")
	})

	t.Run("wrapped validation error is invalid", func(t *testing.T) {
		validation := &ValidationError{Err: errors.New("first_name is required")}
		source := &UsersError{Op: "enable caregiver capability", Err: validation}
		err := caregiverCapabilityError(source)

		invalid, ok := errors.AsType[caregiverInvalidBehaviour](err)
		require.True(t, ok)
		assert.Same(t, validation, invalid.CaregiverRequestInvalid())
		assert.Equal(t, "first_name is required", errors.Unwrap(invalid.CaregiverRequestInvalid()).Error(),
			"the routes render the validation error's cause")
		assert.Equal(t, source.Error(), err.Error())
		assertNoOtherCaregiverBehaviour(t, err, "invalid")
	})

	t.Run("other users error is a failure with its cause", func(t *testing.T) {
		cause := errors.New("audit write failed")
		source := &UsersError{Op: "enable caregiver capability", Err: cause}
		err := caregiverCapabilityError(source)

		failure, ok := errors.AsType[caregiverFailureBehaviour](err)
		require.True(t, ok)
		assert.Same(t, cause, failure.CaregiverFailure())
		assert.Equal(t, source.Error(), err.Error())
		assert.ErrorIs(t, err, cause)
		assertNoOtherCaregiverBehaviour(t, err, "failure")
	})

	t.Run("plain errors pass through", func(t *testing.T) {
		source := errors.New("connection reset")
		err := caregiverCapabilityError(source)

		assert.Same(t, source, err)
		assertNoOtherCaregiverBehaviour(t, err, "")
	})

	t.Run("views classify service failures", func(t *testing.T) {
		views := CaregiverCapabilityViews{service: &stubCaregiverCapabilityService{
			GetFn: func(context.Context, int64) (*userModels.CaregiverCapabilityState, error) {
				return nil, ErrAccountNotFound
			},
		}}

		view, err := views.GetCaregiverCapability(context.Background(), 34)
		assert.Nil(t, view)
		_, ok := errors.AsType[caregiverMissingBehaviour](err)
		assert.True(t, ok)
	})
}
