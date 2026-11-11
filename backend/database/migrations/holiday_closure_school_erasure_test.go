package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	holidayClosureErasureTables = `{"name": "calendar.holiday_closure_events","predicate": "r.tenant_id = $1"},
  {"name": "calendar.holiday_closure_proposals","predicate": "r.tenant_id = $1"},
  {"name": "calendar.holiday_closure_revisions","predicate": "r.tenant_id = $1"},
  `
	holidayClosureErasurePlanAnchor     = `{"name": "calendar.parent_conference_bookings",`
	holidayClosureErasureLockAnchor     = "    calendar.parent_conference_bookings,\n"
	holidayClosureErasureLock           = "    calendar.holiday_closure_events,\n    calendar.holiday_closure_proposals,\n    calendar.holiday_closure_revisions,\n"
	holidayClosureErasureScopeAnchor    = "    SELECT 1 FROM calendar.parent_conference_bookings AS c JOIN calendar.appointments AS p"
	holidayClosureErasureScope          = "    SELECT 1 FROM calendar.holiday_closure_events AS c JOIN calendar.holiday_closure_proposals AS p ON c.\"proposal_id\" = p.\"id\" WHERE (p.tenant_id = p_school_id) AND NOT COALESCE((c.tenant_id = p_school_id), FALSE)\n    UNION ALL\n"
	holidayClosureErasureSignatureOld   = "3c5470c3e959fd028116c405a08023e05d22daf1ebad0da585b09d63b49440f8"
	holidayClosureErasureSignatureNew   = "45ce144272f6f3f674c2f3d15c534584a6051f576bf53d83ea0d3a218c8081b7"
	holidayClosureErasureDeleteAnchor   = "   SELECT count(*) INTO v_count FROM calendar.parent_conference_bookings AS r"
	holidayClosureErasurePreviewAnchor  = " v_counts := v_counts || jsonb_build_object('calendar.parent_conference_bookings',"
	holidayClosureErasureResidualAnchor = "    SELECT 1 FROM calendar.parent_conference_bookings AS r WHERE r.tenant_id = p_school_id\n"
)

var holidayClosureErasureNames = []string{"calendar.holiday_closure_events", "calendar.holiday_closure_proposals", "calendar.holiday_closure_revisions"}

func holidayClosureErasureDeletes() string {
	var b strings.Builder
	for _, name := range holidayClosureErasureNames {
		b.WriteString("   SELECT count(*) INTO v_count FROM " + name + " AS r WHERE r.tenant_id = p_school_id;\n")
		b.WriteString("   DELETE FROM " + name + " AS r WHERE r.tenant_id = p_school_id;\n")
		b.WriteString("   v_counts := v_counts || jsonb_build_object('" + name + "', v_count);\n")
	}
	return b.String()
}

func holidayClosureErasurePreviews() string {
	var b strings.Builder
	for _, name := range holidayClosureErasureNames {
		b.WriteString(" v_counts := v_counts || jsonb_build_object('" + name + "',(SELECT count(*) FROM " + name + " AS r WHERE r.tenant_id = p_school));\n")
	}
	return b.String()
}

func holidayClosureErasureResiduals() string {
	var b strings.Builder
	for _, name := range holidayClosureErasureNames {
		b.WriteString("    SELECT 1 FROM " + name + " AS r WHERE r.tenant_id = p_school_id\n    UNION ALL\n")
	}
	return b.String()
}

var (
	expectedHolidayClosureErasurePlanSQL = strings.Replace(attendanceRetentionMarkErasurePlanSQL, holidayClosureErasurePlanAnchor,
		holidayClosureErasureTables+holidayClosureErasurePlanAnchor, 1)
	expectedHolidayClosureErasurePreflightSQL = strings.Replace(strings.Replace(strings.Replace(emailDeliveryEventErasurePreflightSQL,
		holidayClosureErasureLockAnchor, holidayClosureErasureLock+holidayClosureErasureLockAnchor, 1),
		holidayClosureErasureScopeAnchor, holidayClosureErasureScope+holidayClosureErasureScopeAnchor, 1),
		holidayClosureErasureSignatureOld, holidayClosureErasureSignatureNew, 1)
	expectedHolidayClosureErasureDeleteSQL = strings.Replace(attendanceRetentionMarkErasureDeleteSQL, holidayClosureErasureDeleteAnchor,
		holidayClosureErasureDeletes()+holidayClosureErasureDeleteAnchor, 1)
	expectedHolidayClosureErasurePreviewSQL = strings.Replace(attendanceRetentionMarkErasurePreviewSQL, holidayClosureErasurePreviewAnchor,
		holidayClosureErasurePreviews()+holidayClosureErasurePreviewAnchor, 1)
	expectedHolidayClosureErasureResidualSQL = strings.Replace(attendanceRetentionMarkErasureResidualSQL, holidayClosureErasureResidualAnchor,
		holidayClosureErasureResiduals()+holidayClosureErasureResidualAnchor, 1)
)

// The school erasure of 1.15.919 is the erasure of 1.15.811 with the three
// tables of the holiday closure notices (ADR 0196), and nothing else changed.
func TestHolidayClosureErasureIsTheErasureOf811(t *testing.T) {
	t.Parallel()
	for name, pair := range map[string][2]string{
		"plan":      {expectedHolidayClosureErasurePlanSQL, holidayClosureErasurePlanSQL},
		"preflight": {expectedHolidayClosureErasurePreflightSQL, holidayClosureErasurePreflightSQL},
		"delete":    {expectedHolidayClosureErasureDeleteSQL, holidayClosureErasureDeleteSQL},
		"preview":   {expectedHolidayClosureErasurePreviewSQL, holidayClosureErasurePreviewSQL},
		"residual":  {expectedHolidayClosureErasureResidualSQL, holidayClosureErasureResidualSQL},
	} {
		if name == "delete" {
			assert.Equal(t, strings.Fields(pair[0]), strings.Fields(pair[1]), name)
		} else {
			assert.Equal(t, pair[0], pair[1], name)
		}
		assert.Contains(t, pair[1], "calendar.holiday_closure_events", name)
	}
	assert.NotContains(t, holidayClosureErasurePreflightSQL, holidayClosureErasureSignatureOld, "the preflight names the new foreign keys")
}
