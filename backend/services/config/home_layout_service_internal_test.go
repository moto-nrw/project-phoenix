package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHomeLayoutRepo keeps the two stores in memory, keyed the way the tables
// are: one layout per account, one prescription per tenant. The tenant itself
// is implicit here, exactly as it is in the repository, where it comes from the
// tenant transaction and RLS.
type fakeHomeLayoutRepo struct {
	layouts  map[int64]*configModel.HomeLayout
	policies *configModel.HomeBlockPolicySet
	findErr  error
}

func newFakeHomeLayoutRepo() *fakeHomeLayoutRepo {
	return &fakeHomeLayoutRepo{layouts: map[int64]*configModel.HomeLayout{}}
}

func (f *fakeHomeLayoutRepo) FindByAccount(_ context.Context, accountID int64) (*configModel.HomeLayout, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.layouts[accountID], nil
}

func (f *fakeHomeLayoutRepo) UpsertForAccount(_ context.Context, layout *configModel.HomeLayout) error {
	f.layouts[layout.AccountID] = layout
	return nil
}

func (f *fakeHomeLayoutRepo) DeleteForAccount(_ context.Context, accountID int64) error {
	delete(f.layouts, accountID)
	return nil
}

func (f *fakeHomeLayoutRepo) FindPolicies(context.Context) (*configModel.HomeBlockPolicySet, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.policies, nil
}

func (f *fakeHomeLayoutRepo) UpsertPolicies(_ context.Context, policies *configModel.HomeBlockPolicySet) error {
	f.policies = policies
	return nil
}

// passthroughRuntime runs the closure without a real transaction; the tenant
// boundary itself is covered by the repository tests against Postgres.
type passthroughRuntime struct {
	tenants []int64
	locks   []string
	lockErr error
}

func (r *passthroughRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	r.tenants = append(r.tenants, tenantID)
	return fn(ctx)
}

func (r *passthroughRuntime) AcquireLock(_ context.Context, key string, shared bool) error {
	if shared {
		return errors.New("home layout lock must be exclusive")
	}
	r.locks = append(r.locks, key)
	return r.lockErr
}

func (*passthroughRuntime) AfterCommit(_ context.Context, fn func()) { fn() }

func newHomeLayoutTestService(t *testing.T) (*HomeLayoutService, *fakeHomeLayoutRepo, *passthroughRuntime) {
	t.Helper()
	repo := newFakeHomeLayoutRepo()
	runtime := &passthroughRuntime{}
	return NewHomeLayoutService(repo, runtime, slog.Default()), repo, runtime
}

const adminPermissions = "config:update"

func TestHomeLayoutService_View_EmptyStoresYieldEmptyMaps(t *testing.T) {
	t.Parallel()
	service, _, _ := newHomeLayoutTestService(t)

	view, err := service.View(context.Background(), 7, 42, nil)
	require.NoError(t, err)

	// Not nil: the client distinguishes "no deviations" from "could not read",
	// and a nil map would serialize as null.
	assert.NotNil(t, view.Overrides)
	assert.NotNil(t, view.Policies)
	assert.Empty(t, view.Overrides)
	assert.Empty(t, view.Policies)
	assert.False(t, view.CanManagePolicies)
}

func TestHomeLayoutService_View_ReportsPolicyPermission(t *testing.T) {
	t.Parallel()
	service, _, _ := newHomeLayoutTestService(t)

	view, err := service.View(context.Background(), 7, 42, []string{adminPermissions})
	require.NoError(t, err)
	assert.True(t, view.CanManagePolicies, "an admin is offered the school-wide dialog")

	view, err = service.View(context.Background(), 7, 42, []string{"groups:read"})
	require.NoError(t, err)
	assert.False(t, view.CanManagePolicies, "a care worker is not")
}

func TestHomeLayoutService_NotifiesAffectedStartPagesAfterWrites(t *testing.T) {
	t.Parallel()
	repo := newFakeHomeLayoutRepo()
	runtime := &passthroughRuntime{}
	var notifications []string
	service := NewHomeLayoutService(repo, runtime, slog.Default(), func(_ context.Context, tenantID int64, key string) {
		notifications = append(notifications, fmt.Sprintf("%d:%s", tenantID, key))
	})

	require.NoError(t, service.SetOverrides(context.Background(), 7, 42, map[string]bool{"section.birthdays": false}))
	require.NoError(t, service.ResetOverrides(context.Background(), 7, 42))
	require.NoError(t, service.SetPolicies(context.Background(), 7, 42, []string{adminPermissions}, map[string]configModel.BlockPolicy{
		"tile.students_sick": configModel.BlockRequired,
	}))

	assert.Equal(t, []string{
		"7:home-layout:42",
		"7:home-layout:42",
		"7:home-layout",
	}, notifications)
}

func TestHomeLayoutService_PersonalWritesUseTheAccountLock(t *testing.T) {
	t.Parallel()
	service, _, runtime := newHomeLayoutTestService(t)

	require.NoError(t, service.SetOverrides(context.Background(), 7, 42, map[string]bool{"section.birthdays": false}))
	require.NoError(t, service.ResetOverrides(context.Background(), 7, 42))

	assert.Equal(t, []string{"home-layout:7:42", "home-layout:7:42"}, runtime.locks)
}

func TestHomeLayoutService_PersonalWritesStopWhenTheAccountLockFails(t *testing.T) {
	t.Parallel()
	service, repo, runtime := newHomeLayoutTestService(t)
	runtime.lockErr = errors.New("lock failed")

	err := service.SetOverrides(context.Background(), 7, 42, map[string]bool{"section.birthdays": false})

	require.Error(t, err)
	assert.Empty(t, repo.layouts)
}

func TestHomeLayoutService_View_SchoolPrescriptionBeatsStoredChoice(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)

	// This person hid the birthdays and showed the sick count some time ago.
	repo.layouts[42] = &configModel.HomeLayout{
		TenantID: 7, AccountID: 42,
		Overrides: map[string]bool{
			"section.birthdays":  false,
			"tile.students_sick": true,
			"tile.students_home": true,
		},
	}
	// The school has since made birthdays mandatory and switched the sick
	// count off entirely.
	repo.policies = &configModel.HomeBlockPolicySet{
		TenantID: 7,
		Policies: map[string]configModel.BlockPolicy{
			"section.birthdays":  configModel.BlockRequired,
			"tile.students_sick": configModel.BlockDisabled,
		},
	}

	view, err := service.View(context.Background(), 7, 42, nil)
	require.NoError(t, err)

	// Settled blocks remain in the personal map. The client applies the school
	// policy for visibility, while this preserved choice becomes effective again
	// if the school releases the block.
	assert.Equal(t, map[string]bool{
		"section.birthdays":  false,
		"tile.students_sick": true,
		"tile.students_home": true,
	}, view.Overrides)
	assert.Equal(t, configModel.BlockRequired, view.Policies["section.birthdays"])
	assert.Equal(t, configModel.BlockDisabled, view.Policies["tile.students_sick"])
}

func TestHomeLayoutService_SetOverrides_DropsBlocksTheSchoolHasSettled(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)
	repo.policies = &configModel.HomeBlockPolicySet{
		TenantID: 7,
		Policies: map[string]configModel.BlockPolicy{"tile.students_sick": configModel.BlockDisabled},
	}

	// A client still showing the disabled block sends a choice for it. Dropping
	// it silently beats an error the person could not act on.
	err := service.SetOverrides(context.Background(), 7, 42, map[string]bool{
		"tile.students_sick": true,
		"section.birthdays":  false,
	})
	require.NoError(t, err)

	stored := repo.layouts[42]
	require.NotNil(t, stored)
	assert.Equal(t, map[string]bool{"section.birthdays": false}, stored.Overrides)
	assert.Equal(t, int64(7), stored.TenantID)
	assert.Equal(t, int64(42), stored.AccountID)
}

func TestHomeLayoutService_SetOverrides_PreservesChoiceForSettledBlock(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)
	repo.layouts[42] = &configModel.HomeLayout{
		TenantID:  7,
		AccountID: 42,
		Overrides: map[string]bool{"section.birthdays": false},
	}
	repo.policies = &configModel.HomeBlockPolicySet{
		TenantID: 7,
		Policies: map[string]configModel.BlockPolicy{
			"section.birthdays": configModel.BlockRequired,
		},
	}

	// A client that was open while the policy changed must not replace the
	// previous choice for the now-settled block by saving another choice.
	require.NoError(t, service.SetOverrides(context.Background(), 7, 42, map[string]bool{
		"section.birthdays":  true,
		"tile.students_home": false,
	}))

	assert.Equal(t, map[string]bool{
		"section.birthdays":  false,
		"tile.students_home": false,
	}, repo.layouts[42].Overrides)
}

func TestHomeLayoutService_SetOverrides_RejectsMalformedKey(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)

	err := service.SetOverrides(context.Background(), 7, 42, map[string]bool{"../../etc": true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidValue), "a malformed payload is a client error, not a 500")
	assert.Empty(t, repo.layouts, "nothing is stored when the payload is rejected")
}

func TestHomeLayoutService_SetOverrides_RequiresAccount(t *testing.T) {
	t.Parallel()
	service, _, _ := newHomeLayoutTestService(t)

	require.Error(t, service.SetOverrides(context.Background(), 7, 0, map[string]bool{}))
}

func TestHomeLayoutService_ResetOverrides_RestoresRecommendation(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)
	repo.layouts[42] = &configModel.HomeLayout{
		TenantID: 7, AccountID: 42,
		Overrides: map[string]bool{"section.birthdays": false},
	}

	require.NoError(t, service.ResetOverrides(context.Background(), 7, 42))
	assert.NotContains(t, repo.layouts, int64(42),
		"the row is dropped rather than emptied, so later default changes still reach this account")
}

func TestHomeLayoutService_SetPolicies_RequiresPermission(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)

	err := service.SetPolicies(context.Background(), 7, 42, []string{"groups:read"},
		map[string]configModel.BlockPolicy{"section.birthdays": configModel.BlockDisabled})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPermissionDenied))
	assert.Nil(t, repo.policies, "a caller without config:update changes nothing")
}

func TestHomeLayoutService_SetPolicies_AcceptsAdminWildcard(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)

	err := service.SetPolicies(context.Background(), 7, 42, []string{"admin:*"},
		map[string]configModel.BlockPolicy{"section.birthdays": configModel.BlockRequired})

	require.NoError(t, err)
	require.NotNil(t, repo.policies)
	assert.Equal(t, configModel.BlockRequired, repo.policies.Policies["section.birthdays"])
}

func TestHomeLayoutService_SetPolicies_DropsOptionalAndStampsAuthor(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)

	err := service.SetPolicies(context.Background(), 7, 42, []string{adminPermissions},
		map[string]configModel.BlockPolicy{
			"section.birthdays":  configModel.BlockOptional,
			"tile.students_sick": configModel.BlockRequired,
		})
	require.NoError(t, err)

	require.NotNil(t, repo.policies)
	// "The school has no opinion" is the default and is not stored, so a later
	// change of defaults stays distinguishable from a deliberate decision.
	assert.Equal(t, map[string]configModel.BlockPolicy{"tile.students_sick": configModel.BlockRequired},
		repo.policies.Policies)
	require.NotNil(t, repo.policies.UpdatedBy)
	assert.Equal(t, int64(42), *repo.policies.UpdatedBy)
}

func TestHomeLayoutService_SetPolicies_RejectsUnknownPolicy(t *testing.T) {
	t.Parallel()
	service, repo, _ := newHomeLayoutTestService(t)

	err := service.SetPolicies(context.Background(), 7, 42, []string{adminPermissions},
		map[string]configModel.BlockPolicy{"section.birthdays": "sometimes"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidValue))
	assert.Nil(t, repo.policies)
}

func TestHomeLayoutService_WritesRunInTheCallersTenant(t *testing.T) {
	t.Parallel()
	service, _, runtime := newHomeLayoutTestService(t)

	require.NoError(t, service.SetOverrides(context.Background(), 7, 42, map[string]bool{"section.birthdays": false}))
	require.NoError(t, service.SetOverrides(context.Background(), 9, 42, map[string]bool{"section.birthdays": true}))

	// The same person at two schools is two separate start pages, and each
	// write must reach the tenant it was made in.
	assert.Equal(t, []int64{7, 9}, runtime.tenants)
}

func TestHomeLayoutService_NotConfigured(t *testing.T) {
	t.Parallel()
	var service *HomeLayoutService

	_, err := service.View(context.Background(), 7, 42, nil)
	assert.True(t, errors.Is(err, ErrHomeLayoutUnavailable))
	assert.True(t, errors.Is(service.SetOverrides(context.Background(), 7, 42, nil), ErrHomeLayoutUnavailable))
	assert.True(t, errors.Is(service.ResetOverrides(context.Background(), 7, 42), ErrHomeLayoutUnavailable))
}
