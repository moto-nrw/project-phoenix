package device

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

// mockStaffLookup implements StaffLookup for resolveStaffContext tests.
type mockStaffLookup struct {
	staff *users.Staff
	err   error
}

func (m *mockStaffLookup) GetStaffByID(_ context.Context, _ int64) (*users.Staff, error) {
	return m.staff, m.err
}

func testDeviceForTenant(tenantID int64) *iot.Device {
	dev := &iot.Device{DeviceID: "test-device"}
	dev.TenantID = tenantID
	return dev
}

func tenantStaff(id, tenantID int64) *users.Staff {
	staff := &users.Staff{}
	staff.ID = id
	staff.SetTenantID(tenantID)
	return staff
}

func TestResolveStaffContext_ValidStaff(t *testing.T) {
	staff := tenantStaff(42, 7)
	lookup := &mockStaffLookup{staff: staff}

	result := resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "42")
	assert.Equal(t, staff, result)
}

func TestResolveStaffContext_EmptyHeader(t *testing.T) {
	lookup := &mockStaffLookup{staff: tenantStaff(42, 7)}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), ""))
}

func TestResolveStaffContext_NilLookup(t *testing.T) {
	assert.Nil(t, resolveStaffContext(context.Background(), nil, testDeviceForTenant(7), "42"))
}

func TestResolveStaffContext_InvalidHeader(t *testing.T) {
	lookup := &mockStaffLookup{staff: tenantStaff(42, 7)}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "abc"))
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "-1"))
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "0"))
}

func TestResolveStaffContext_LookupError(t *testing.T) {
	lookup := &mockStaffLookup{err: errors.New("not found")}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "42"))
}

func TestResolveStaffContext_CrossTenantRejected(t *testing.T) {
	lookup := &mockStaffLookup{staff: tenantStaff(42, 99)}
	assert.Nil(t, resolveStaffContext(context.Background(), lookup, testDeviceForTenant(7), "42"))
}
