package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/test"
)

const mailIdentityTenantID int64 = 4711

func schoolRepoReturning(school *platformModels.School, err error) *test.SchoolRepoMock {
	return &test.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platformModels.School, error) {
			return school, err
		},
	}
}

func settingsReturning(value string, err error) *configtest.Mock {
	return &configtest.Mock{
		ResolveStringForTenantFn: func(_ context.Context, _ int64, key string) (string, error) {
			if key != configModels.KeyEmailReplyToAddress {
				return "", nil
			}
			return value, err
		},
	}
}

// The explicit setting is the school's own decision and must win over the
// Stammdaten contact address.
func TestResolveTenantMailIdentity_SettingWinsOverSchoolContact(t *testing.T) {
	t.Parallel()

	svc := NewTenantMailIdentityService(
		schoolRepoReturning(&platformModels.School{Name: "OGS Am Berg", Email: "buero@schule.example"}, nil),
		settingsReturning("eltern@schule.example", nil),
		nil,
	)

	identity, err := svc.ResolveTenantMailIdentity(context.Background(), mailIdentityTenantID)
	require.NoError(t, err)
	assert.Equal(t, "eltern@schule.example", identity.ReplyToAddress)
	assert.Equal(t, "OGS Am Berg", identity.ReplyToName)
}

// Without an explicit setting the school contact address is used. This is the
// fallback that makes the fix reach schools that never open the setting —
// the reported failure was answers landing at moto (#1936).
func TestResolveTenantMailIdentity_FallsBackToSchoolContact(t *testing.T) {
	t.Parallel()

	svc := NewTenantMailIdentityService(
		schoolRepoReturning(&platformModels.School{Name: "OGS Am Berg", Email: "buero@schule.example"}, nil),
		settingsReturning("", nil),
		nil,
	)

	identity, err := svc.ResolveTenantMailIdentity(context.Background(), mailIdentityTenantID)
	require.NoError(t, err)
	assert.Equal(t, "buero@schule.example", identity.ReplyToAddress)
}

// Neither source configured must stay silent: no Reply-To header, mail
// behaves exactly as it did before this feature.
func TestResolveTenantMailIdentity_NothingConfigured_IsZero(t *testing.T) {
	t.Parallel()

	svc := NewTenantMailIdentityService(
		schoolRepoReturning(&platformModels.School{Name: "OGS Am Berg"}, nil),
		settingsReturning("", nil),
		nil,
	)

	identity, err := svc.ResolveTenantMailIdentity(context.Background(), mailIdentityTenantID)
	require.NoError(t, err)
	assert.True(t, identity.IsZero())
}

// Whitespace-only values are not addresses. Trimming them to empty keeps the
// fallback chain working instead of writing a broken header.
func TestResolveTenantMailIdentity_BlankSettingFallsThrough(t *testing.T) {
	t.Parallel()

	svc := NewTenantMailIdentityService(
		schoolRepoReturning(&platformModels.School{Name: "OGS", Email: "  buero@schule.example  "}, nil),
		settingsReturning("   ", nil),
		nil,
	)

	identity, err := svc.ResolveTenantMailIdentity(context.Background(), mailIdentityTenantID)
	require.NoError(t, err)
	assert.Equal(t, "buero@schule.example", identity.ReplyToAddress)
}

// A failing settings lookup must degrade to the school contact address, never
// cost the mail. Losing the return path is bad; losing the invitation is worse.
func TestResolveTenantMailIdentity_SettingsErrorFallsBackToSchool(t *testing.T) {
	t.Parallel()

	svc := NewTenantMailIdentityService(
		schoolRepoReturning(&platformModels.School{Name: "OGS", Email: "buero@schule.example"}, nil),
		settingsReturning("", errors.New("settings unavailable")),
		nil,
	)

	identity, err := svc.ResolveTenantMailIdentity(context.Background(), mailIdentityTenantID)
	require.NoError(t, err)
	assert.Equal(t, "buero@schule.example", identity.ReplyToAddress)
}

// Same rule for a failing school lookup: no identity, no error, mail still goes.
func TestResolveTenantMailIdentity_SchoolErrorDegradesToNoReplyTo(t *testing.T) {
	t.Parallel()

	svc := NewTenantMailIdentityService(
		schoolRepoReturning(nil, errors.New("school lookup failed")),
		settingsReturning("", nil),
		nil,
	)

	identity, err := svc.ResolveTenantMailIdentity(context.Background(), mailIdentityTenantID)
	require.NoError(t, err)
	assert.True(t, identity.IsZero())
}

// A cross-tenant send path with no tenant (operator/system mail) must never
// pick up a school reply address.
func TestResolveTenantMailIdentity_NoTenant_IsZero(t *testing.T) {
	t.Parallel()

	svc := NewTenantMailIdentityService(
		schoolRepoReturning(&platformModels.School{Name: "OGS", Email: "buero@schule.example"}, nil),
		settingsReturning("eltern@schule.example", nil),
		nil,
	)

	identity, err := svc.ResolveTenantMailIdentity(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, identity.IsZero())
}
