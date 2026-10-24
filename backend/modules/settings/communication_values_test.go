package settings

import (
	"context"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type communicationValueProbe struct {
	boolean       func(context.Context, string) (bool, error)
	integer       func(context.Context, string) (int, error)
	tenantBoolean func(context.Context, int64, string) (bool, error)
	image         func(context.Context, int64) (string, error)
}

func (p communicationValueProbe) ResolveBool(ctx context.Context, key string) (bool, error) {
	return p.boolean(ctx, key)
}
func (p communicationValueProbe) ResolveInt(ctx context.Context, key string) (int, error) {
	return p.integer(ctx, key)
}
func (p communicationValueProbe) ResolveBoolForTenant(ctx context.Context, id int64, key string) (bool, error) {
	return p.tenantBoolean(ctx, id, key)
}
func (p communicationValueProbe) GetLoginImageURL(ctx context.Context, id int64) (string, error) {
	return p.image(ctx, id)
}

type communicationContextKey struct{}

func TestCommunicationValuesPreserveCanonicalBooleanKeysContextAndErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		key  string
		call func(context.Context, communicationValueProbe) (bool, error)
	}{
		{"staff name", configModel.KeyParentMessageStaffNameVisible, func(ctx context.Context, p communicationValueProbe) (bool, error) {
			return NewParentMessagingValues(p).ParentMessageStaffNameVisible(ctx)
		}},
		{"parent news", configModel.KeyParentNewsEnabled, func(ctx context.Context, p communicationValueProbe) (bool, error) {
			return NewParentAnnouncementValues(p).ParentNewsEnabled(ctx)
		}},
		{"staff messaging", configModel.KeyStaffMessagingEnabled, func(ctx context.Context, p communicationValueProbe) (bool, error) {
			return NewStaffMessagingValues(p).StaffMessagingEnabled(ctx)
		}},
		{"shared parent gate", configModel.KeyParentNotesEnabled, func(ctx context.Context, p communicationValueProbe) (bool, error) {
			return NewParentMessagingValues(p).ResolveBool(ctx, configModel.KeyParentNotesEnabled)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.WithValue(context.Background(), communicationContextKey{}, tc.name)
			failure := errors.New("flag unavailable")
			for _, expectedError := range []error{nil, failure} {
				calls := 0
				probe := communicationValueProbe{boolean: func(actual context.Context, key string) (bool, error) {
					calls++
					assert.Same(t, ctx, actual)
					assert.Equal(t, tc.key, key)
					return false, expectedError
				}}
				value, err := tc.call(ctx, probe)
				assert.False(t, value, "a false tenant choice is not replaced with a local default")
				assert.Equal(t, 1, calls)
				if expectedError == nil {
					require.NoError(t, err)
				} else {
					assert.Same(t, expectedError, err)
				}
			}
		})
	}
}

func TestCommunicationValuesPreserveExplicitSchoolAndLogoResults(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), communicationContextKey{}, "school")
	failure := errors.New("source refused")
	tenantID := int64(8123)
	probe := communicationValueProbe{
		tenantBoolean: func(actual context.Context, id int64, key string) (bool, error) {
			assert.Same(t, ctx, actual)
			assert.Equal(t, tenantID, id)
			assert.Equal(t, configModel.KeyParentNotesEnabled, key)
			return false, failure
		},
		image: func(actual context.Context, id int64) (string, error) {
			assert.Same(t, ctx, actual)
			assert.Equal(t, tenantID, id)
			return "/school-logo.svg?x=%2F", failure
		},
	}
	values := NewParentEventValues(probe)
	for _, call := range []func() (bool, error){
		func() (bool, error) { return values.ParentNotesEnabledForTenant(ctx, tenantID) },
		func() (bool, error) {
			return values.ResolveBoolForTenant(ctx, tenantID, configModel.KeyParentNotesEnabled)
		},
	} {
		value, err := call()
		assert.False(t, value)
		assert.Same(t, failure, err)
	}
	logo, err := NewParentAnnouncementValues(probe).GetLoginImageURL(ctx, tenantID)
	assert.Equal(t, "/school-logo.svg?x=%2F", logo)
	assert.Same(t, failure, err, "the facade must not drop a partial source result or wrap its error")
}

func TestCommunicationValuesPreserveZeroRetentionAndLoggingKeys(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), communicationContextKey{}, "retention")
	failure := errors.New("retention unavailable")
	for _, expectedKey := range []string{configModel.KeyGDPRParentMessageRetentionDays, configModel.KeyGDPRStaffMessageRetentionDays} {
		probe := communicationValueProbe{integer: func(actual context.Context, key string) (int, error) {
			assert.Same(t, ctx, actual)
			assert.Equal(t, expectedKey, key)
			return 0, failure
		}}
		var value int
		var err error
		if expectedKey == configModel.KeyGDPRParentMessageRetentionDays {
			value, err = NewParentMessageRetentionValues(probe).ParentMessageRetentionDays(ctx)
		} else {
			value, err = NewStaffMessagingValues(probe).StaffMessageRetentionDays(ctx)
		}
		assert.Zero(t, value, "a zero result must not become a guessed retention window")
		assert.Same(t, failure, err)
	}
	// No source function is configured: identifying a setting for the log must
	// never perform another resolution or touch the database.
	keys := NewStaffMessagingValues(communicationValueProbe{})
	assert.Equal(t, configModel.KeyStaffMessagingEnabled, keys.StaffMessagingEnabledKey())
	assert.Equal(t, configModel.KeyGDPRStaffMessageRetentionDays, keys.StaffMessageRetentionDaysKey())
}

func TestCommunicationValuesKeepMissingDependenciesNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewParentMessagingValues(nil))
	assert.Nil(t, NewParentAnnouncementValues(nil))
	assert.Nil(t, NewStaffMessagingValues(nil))
	assert.Nil(t, NewParentEventValues(nil))
	assert.Nil(t, NewParentMessageRetentionValues(nil))
}
