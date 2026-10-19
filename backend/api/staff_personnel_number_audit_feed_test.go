package api

import (
	"encoding/json"
	"testing"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A personnel number change written by the staff administration appears in
// the Audit Platform's time-tracking audit feed. The suite moved here from the
// People Directory contract suite, which may not name the feed's filter; the
// root composes both owners.
func TestUpdatePersonnelNumber_AppearsInAuditFeed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposMembership := NewSchoolMembershipSuiteRepositories(testpkg.NewDatabase(t, db))
	audits := owners.NewTestAuditPersistence(testpkg.MustDatabase(db), nil)
	svc, err := NewStaffAdmin(testpkg.MustDatabase(db), repos.Staff, repos.Person, reposMembership.SchoolMembership(), StaffAdminDependencies{
		MasterDataAudit: audits.StaffMasterDataChange, PersonnelNumber: audits.PersonnelNumberChange, DataAccessLog: audits.DataAccessLog,
	})
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Payroll", "Feed")
	actor := testpkg.CreateTestStaff(t, db, "Payroll", "FeedAktor")

	value := "90030"
	_, err = svc.UpdatePersonnelNumber(ctx, staff.ID, &value, actor.ID, "Feed-Test")
	require.NoError(t, err)

	entries, err := testpkg.TenantQuery1(t, ctx, db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).TimeTrackingAuditLog.ListEntries, auditModels.TimeTrackingAuditLogFilter{
		StaffID: staff.ID,
		Sources: []string{testpkg.AuditLogSourcePersonnelNumber},
	})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, testpkg.AuditLogSourcePersonnelNumber, entry.Source)
	require.NotNil(t, entry.StaffID)
	assert.Equal(t, staff.ID, *entry.StaffID)
	require.NotNil(t, entry.ActorStaffID)
	assert.Equal(t, actor.ID, *entry.ActorStaffID)
	assert.Equal(t, "Feed-Test", entry.Reason)

	var detail struct {
		OldValue string `json:"old_value"`
		NewValue string `json:"new_value"`
	}
	require.NoError(t, json.Unmarshal(entry.Detail, &detail))
	assert.Equal(t, "", detail.OldValue)
	assert.Equal(t, "90030", detail.NewValue)
}
