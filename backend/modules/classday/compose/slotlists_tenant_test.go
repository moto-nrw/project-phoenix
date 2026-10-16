package compose_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestBuildList_TenantIsolation proves the projection's tenant safety (#2701):
// a second school's slot and children on the same date never reach the first
// school's list, and the second school sees only its own rows, even though
// every owner facade reads the same tables.
func TestBuildList_TenantIsolation(t *testing.T) {
	t.Parallel()
	f := buildMensaFixture(t)
	ctx := testpkg.Ctx(t)

	otherTenant, _ := testpkg.CreateTestTenant(t, f.db)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoomForTenant(t, f.db, otherTenant, fmt.Sprintf("Other-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroupForTenant(t, f.db, otherTenant, fmt.Sprintf("Other-Act-%d", suffix))
	activeGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, f.db, otherTenant, activity.ID, room.ID)
	instance := testpkg.CreateTestActivityInstanceForTenant(t, f.db, otherTenant, listDate, room.ID, testpkg.ActivityInstanceOpts{
		Title: fmt.Sprintf("Other Mensa %d", suffix), Status: "active", ActivityGroupID: &activity.ID, ActiveGroupID: &activeGroup.ID,
		StartHHMM: "11:30", EndHHMM: "13:00",
	})
	otherStudent := testpkg.CreateTestStudentForTenant(t, f.db, otherTenant, "Other", fmt.Sprintf("Kind-%d", suffix), "2b")

	params := classday.Params{Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourceReconciliation}

	own, err := f.svc.BuildList(ctx, params)
	require.NoError(t, err)
	for _, slot := range own.Slots {
		assert.NotEqual(t, instance.ID, slot.InstanceID, "the other school's slot must not be listed")
	}
	assert.Nil(t, rowByStudent(own.Rows, otherStudent.ID), "the other school's child must not be listed")
	require.NotNil(t, rowByStudent(own.Rows, f.plannedID))

	other, err := f.svc.BuildList(testpkg.TenantContext(otherTenant), params)
	require.NoError(t, err)
	require.Len(t, other.Slots, 1)
	assert.Equal(t, instance.ID, other.Slots[0].InstanceID)
	assert.Empty(t, other.Rows, "the other school has no roster and no visits on its slot")
	assert.Nil(t, rowByStudent(other.Rows, f.plannedID))
}

// capturedExport records the document the projection hands the renderer.
type capturedExport struct{ docs []classday.ExportDocument }

func (c *capturedExport) Render(doc classday.ExportDocument, format classday.Format, filenameBase string) (classday.File, error) {
	c.docs = append(c.docs, doc)
	return classday.File{Filename: filenameBase + "." + string(format)}, nil
}

func exportedNames(doc classday.ExportDocument) []string {
	var names []string
	for _, row := range doc.Rows {
		if name, ok := row.Values[classday.ExportColumnName]; ok {
			names = append(names, name)
		}
	}
	return names
}

// TestRenderList_TenantIsolation proves the export carries only the school's
// own children (#4077): a second school plans its own child on a slot of the
// same date, and neither school's printed list names the other's child.
func TestRenderList_TenantIsolation(t *testing.T) {
	t.Parallel()
	f := buildMensaFixture(t)
	ctx := testpkg.Ctx(t)

	otherTenant, _ := testpkg.CreateTestTenant(t, f.db)
	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoomForTenant(t, f.db, otherTenant, fmt.Sprintf("Other-Room-%d", suffix))
	activity := testpkg.CreateTestActivityGroupForTenant(t, f.db, otherTenant, fmt.Sprintf("Other-Act-%d", suffix))
	activeGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, f.db, otherTenant, activity.ID, room.ID)
	instance := testpkg.CreateTestActivityInstanceForTenant(t, f.db, otherTenant, listDate, room.ID, testpkg.ActivityInstanceOpts{
		Title: fmt.Sprintf("Other Mensa %d", suffix), Status: "active", ActivityGroupID: &activity.ID, ActiveGroupID: &activeGroup.ID,
		StartHHMM: "11:30", EndHHMM: "13:00",
	})
	otherLastName := fmt.Sprintf("Kind-%d", suffix)
	otherStudent := testpkg.CreateTestStudentForTenant(t, f.db, otherTenant, "Fremd", otherLastName, "2b")
	otherRow := &testpkg.InstanceStudent{InstanceID: instance.ID, StudentID: otherStudent.ID, Status: testpkg.ScheduleAttendanceStatusExpected}
	otherRow.TenantID = otherTenant
	testpkg.InsertInstanceStudentRow(t, testpkg.TenantContext(otherTenant), f.db, otherRow)

	exports := &capturedExport{}
	svc := newTestServiceWithRenderer(f.db, newTestRoomRepository(f.db), stubSlotListSettings{},
		slotListUserContext{currentStaff: &testpkg.Staff{}}, slotListParticipation{}, exports)
	params := classday.Params{Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourcePlanned}

	_, err := svc.RenderList(ctx, params, classday.FormatPDF)
	require.NoError(t, err)
	_, err = svc.RenderList(testpkg.TenantContext(otherTenant), params, classday.FormatPDF)
	require.NoError(t, err)
	require.Len(t, exports.docs, 2)

	own, other := exportedNames(exports.docs[0]), exportedNames(exports.docs[1])
	require.Len(t, own, 2, "the school's export lists its two planned children")
	require.Len(t, other, 1, "the second school's export lists its own planned child")
	for _, name := range own {
		assert.NotContains(t, name, otherLastName, "the other school's child must not be printed")
	}
	assert.Contains(t, other[0], otherLastName)
	for _, name := range other {
		assert.NotContains(t, name, "SL-Planned", "the school's child must not reach the other school's export")
		assert.NotContains(t, name, "SL-Missing", "the school's child must not reach the other school's export")
	}
}
