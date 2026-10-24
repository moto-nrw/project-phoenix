package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// fakeModuleReader answers module switches per tenant; requestTenant stands
// for the tenant of the request context.
type fakeModuleReader struct {
	requestTenant int64
	values        map[int64]map[string]bool
	err           error
}

func (r *fakeModuleReader) ResolveBool(ctx context.Context, key string) (bool, error) {
	return r.ResolveBoolForTenant(ctx, r.requestTenant, key)
}

func (r *fakeModuleReader) ResolveBoolForTenant(_ context.Context, tenantID int64, key string) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	value, ok := r.values[tenantID][key]
	if !ok {
		return true, nil // registry default: booked
	}
	return value, nil
}

// The fake reader holds no rows; the school IDs only key its map.
const (
	requestSchool int64 = 4101
	otherSchool   int64 = 4102
)

func TestModuleBookingReadsEachSchoolsOwnBooking(t *testing.T) {
	t.Parallel()
	reader := &fakeModuleReader{requestTenant: requestSchool, values: map[int64]map[string]bool{
		requestSchool: {configModel.KeyModuleZeiterfassung: false},
	}}
	booking := NewModuleBooking(reader)
	ctx := context.Background()

	booked, err := booking.IsBooked(ctx, ModuleZeiterfassung)
	require.NoError(t, err)
	assert.False(t, booked)
	booked, err = booking.IsBookedForTenant(ctx, otherSchool, ModuleZeiterfassung)
	require.NoError(t, err)
	assert.True(t, booked, "the other school keeps the default")

	var notBooked *ModuleNotBookedError
	require.ErrorAs(t, booking.Require(ctx, requestSchool, ModuleDienstplan, ModuleZeiterfassung), &notBooked)
	assert.Equal(t, ModuleZeiterfassung, notBooked.Module)
	assert.Equal(t, requestSchool, notBooked.TenantID)
	require.NoError(t, booking.RequireForTenant(ctx, otherSchool, ModuleZeiterfassung))
}

// A failed read is an error, never a booking decision.
func TestModuleBookingPassesReadErrorsThrough(t *testing.T) {
	t.Parallel()
	readErr := errors.New("settings unavailable")
	booking := NewModuleBooking(&fakeModuleReader{err: readErr})

	_, err := booking.IsBooked(context.Background(), ModuleMittagessen)
	require.ErrorIs(t, err, readErr)
	err = booking.RequireForTenant(context.Background(), otherSchool, ModuleMittagessen)
	require.ErrorIs(t, err, readErr)
	var notBooked *ModuleNotBookedError
	assert.False(t, errors.As(err, &notBooked))
}

func TestModuleBookingRejectsUnknownModules(t *testing.T) {
	t.Parallel()
	booking := NewModuleBooking(&fakeModuleReader{})
	_, err := booking.IsBooked(context.Background(), ModuleKey("basis"))
	require.ErrorIs(t, err, ErrUnknownModule)
}

func TestNewModuleBookingRequiresAReader(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { NewModuleBooking(nil) })
}

func TestBookedModulesFollowsTheModuleList(t *testing.T) {
	t.Parallel()
	off := map[string]bool{configModel.KeyModuleNFCAnwesenheit: true, configModel.KeyModuleVertraege: true}
	booked, err := BookedModules(func(key string) (bool, error) { return !off[key], nil })
	require.NoError(t, err)
	assert.Len(t, booked, len(Modules())-2)
	assert.NotContains(t, booked, ModuleNFCAnwesenheit)
	assert.Equal(t, ModuleMittagessen, booked[0])

	_, err = BookedModules(func(string) (bool, error) { return false, errors.New("boom") })
	require.Error(t, err)
}
