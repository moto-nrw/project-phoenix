package timetable

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
)

// These tests pin the presence contract of the offering-source fields on the
// template PUT (#2147 review round 12): a client that omits the fields — the
// full pre-#2137 update body — must keep the stored source untouched; only an
// explicit null clears it.

func decodeUpdateSourceFields(t *testing.T, raw string) *updateTemplateRequest {
	t.Helper()
	req := &updateTemplateRequest{}
	require.NoError(t, json.Unmarshal([]byte(raw), req))
	return req
}

func TestUpdateTemplateRequestSourceFieldPresence(t *testing.T) {
	t.Parallel()

	t.Run("omitted fields are unset", func(t *testing.T) {
		req := decodeUpdateSourceFields(t, `{"name":"Tpl"}`)
		assert.False(t, req.SourceCareOfferingIDs.Set)
		assert.False(t, req.SourceGradeLevels.Set)
	})

	t.Run("explicit null is set with nil value", func(t *testing.T) {
		req := decodeUpdateSourceFields(t, `{"source_care_offering_ids":null,"source_grade_levels":null}`)
		assert.True(t, req.SourceCareOfferingIDs.Set)
		assert.Nil(t, req.SourceCareOfferingIDs.Value)
		assert.True(t, req.SourceGradeLevels.Set)
		assert.Nil(t, req.SourceGradeLevels.Value)
	})

	t.Run("explicit values are set", func(t *testing.T) {
		req := decodeUpdateSourceFields(t, `{"source_care_offering_ids":[7,9],"source_grade_levels":[2,3]}`)
		require.True(t, req.SourceCareOfferingIDs.Set)
		assert.Equal(t, []int64{7, 9}, req.SourceCareOfferingIDs.Value)
		assert.True(t, req.SourceGradeLevels.Set)
		assert.Equal(t, []int{2, 3}, req.SourceGradeLevels.Value)
	})

	t.Run("explicit empty filter list is set and empty", func(t *testing.T) {
		req := decodeUpdateSourceFields(t, `{"source_grade_levels":[]}`)
		assert.True(t, req.SourceGradeLevels.Set)
		assert.Empty(t, req.SourceGradeLevels.Value)
	})
}

// validUpdateBodyJSON builds a minimal body that passes the presence checks
// in updateTemplateRequest.Bind; the fragment is spliced in per-case.
func validUpdateBodyJSON(fragment string) string {
	extra := ""
	if fragment != "" {
		extra = "," + fragment
	}
	return `{"name":"Tpl","type":"care","weekdays":[1],"start_time":"14:00",` +
		`"end_time":"15:00","room_id":3,"category_id":2` + extra + `}`
}

// A filter-only edit of a sourced template omits source_care_offering_id and
// still submits source_grade_levels; the stored source merges in AFTER bind
// (applyOfferingSourcePresence), so bind must not validate the filter against
// the not-yet-merged nil source (#2147 review round 14). Submitted source
// fields keep failing fast at bind.
func TestUpdateTemplateBind_DefersSourceValidationUntilMerge(t *testing.T) {
	t.Parallel()

	t.Run("filter-only body binds and keeps the filter for the merge", func(t *testing.T) {
		req := decodeUpdateSourceFields(t, validUpdateBodyJSON(
			`"target_group_type":"angebot","source_grade_levels":[2]`))
		require.NoError(t, req.Bind(nil))
		assert.False(t, req.SourceCareOfferingIDs.Set)
		assert.Equal(t, []int{2}, req.SourceGradeLevels.Value)
	})

	t.Run("explicit null source next to a filter fails at bind", func(t *testing.T) {
		req := decodeUpdateSourceFields(t, validUpdateBodyJSON(
			`"target_group_type":"angebot","source_care_offering_ids":null,"source_grade_levels":[2]`))
		require.ErrorContains(t, req.Bind(nil), "source_grade_levels requires source_care_offering_ids")
	})

	t.Run("submitted source on a non-angebot type fails at bind", func(t *testing.T) {
		req := decodeUpdateSourceFields(t, validUpdateBodyJSON(
			`"target_group_type":"klasse","target_school_class":"1a","source_care_offering_ids":[7]`))
		require.ErrorContains(t, req.Bind(nil), "requires target group type 'angebot'")
	})
}

func sourcedExistingTemplate(offeringIDs []int64, gradeLevels []int) templateResponse {
	return templateResponse{
		TargetGroupType:       activitiesModel.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: offeringIDs,
		SourceGradeLevels:     gradeLevels,
	}
}

func TestApplyOfferingSourcePresence(t *testing.T) {
	t.Parallel()

	storedOfferingIDs := []int64{41, 43}

	t.Run("omitted fields carry the stored sources and filter", func(t *testing.T) {
		req := &updateTemplateRequest{TargetGroupType: activitiesModel.TargetGroupTypeAngebot}
		applyOfferingSourcePresence(req, sourcedExistingTemplate(storedOfferingIDs, []int{1, 2}))
		assert.Equal(t, storedOfferingIDs, req.SourceCareOfferingIDs.Value)
		assert.Equal(t, []int{1, 2}, req.SourceGradeLevels.Value)
	})

	t.Run("explicit null clears sources and drags the omitted filter along", func(t *testing.T) {
		req := &updateTemplateRequest{
			TargetGroupType:       activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: nullableInt64Slice{Set: true, Value: nil},
		}
		applyOfferingSourcePresence(req, sourcedExistingTemplate(storedOfferingIDs, []int{1, 2}))
		assert.Nil(t, req.SourceCareOfferingIDs.Value)
		assert.Nil(t, req.SourceGradeLevels.Value,
			"a source cleared by explicit null must not leave the stored filter behind (DB CHECK)")
	})

	t.Run("provided values win over the stored ones", func(t *testing.T) {
		req := &updateTemplateRequest{
			TargetGroupType:       activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: nullableInt64Slice{Set: true, Value: []int64{44}},
			SourceGradeLevels:     nullableIntSlice{Set: true, Value: []int{4}},
		}
		applyOfferingSourcePresence(req, sourcedExistingTemplate(storedOfferingIDs, []int{1, 2}))
		assert.Equal(t, []int64{44}, req.SourceCareOfferingIDs.Value)
		assert.Equal(t, []int{4}, req.SourceGradeLevels.Value)
	})

	t.Run("omitted filter follows a provided source", func(t *testing.T) {
		req := &updateTemplateRequest{
			TargetGroupType:       activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: nullableInt64Slice{Set: true, Value: []int64{44}},
		}
		applyOfferingSourcePresence(req, sourcedExistingTemplate(storedOfferingIDs, []int{1, 2}))
		assert.Equal(t, []int{1, 2}, req.SourceGradeLevels.Value)
	})

	t.Run("leaving the angebot Zielgruppe drops the rule instead of carrying it", func(t *testing.T) {
		req := &updateTemplateRequest{TargetGroupType: activitiesModel.TargetGroupTypeKlasse}
		applyOfferingSourcePresence(req, sourcedExistingTemplate(storedOfferingIDs, []int{1, 2}))
		assert.Nil(t, req.SourceCareOfferingIDs.Value,
			"a source cannot exist outside 'angebot' (DB CHECK); the type switch removes it like a split does")
		assert.Nil(t, req.SourceGradeLevels.Value)
	})

	t.Run("no stored source stays no source", func(t *testing.T) {
		req := &updateTemplateRequest{TargetGroupType: activitiesModel.TargetGroupTypeAngebot}
		applyOfferingSourcePresence(req, templateResponse{TargetGroupType: activitiesModel.TargetGroupTypeAngebot})
		assert.Nil(t, req.SourceCareOfferingIDs.Value)
		assert.Nil(t, req.SourceGradeLevels.Value)
	})
}

// #2482: the Klassenfilter follows the same presence rules as the
// Jahrgangsfilter, plus the mutual exclusion — submitting one non-empty
// filter must DROP a stored other one instead of colliding with it.
func TestApplyOfferingSourcePresenceClassFilter(t *testing.T) {
	t.Parallel()

	stored := templateResponse{
		SourceCareOfferingIDs: []int64{44},
		SourceGradeLevels:     []int{1},
	}
	storedClasses := templateResponse{
		SourceCareOfferingIDs: []int64{44},
		SourceSchoolClasses:   []string{"1b"},
	}

	t.Run("omitted class filter inherits the stored one", func(t *testing.T) {
		req := &updateTemplateRequest{TargetGroupType: activitiesModel.TargetGroupTypeAngebot}
		applyOfferingSourcePresence(req, storedClasses)
		assert.Equal(t, []string{"1b"}, req.SourceSchoolClasses.Value)
		assert.Empty(t, req.SourceGradeLevels.Value)
	})

	t.Run("submitted class filter drops the stored grade filter", func(t *testing.T) {
		req := &updateTemplateRequest{
			TargetGroupType:     activitiesModel.TargetGroupTypeAngebot,
			SourceSchoolClasses: nullableStringSlice{Set: true, Value: []string{"1b"}},
		}
		applyOfferingSourcePresence(req, stored)
		assert.Equal(t, []string{"1b"}, req.SourceSchoolClasses.Value)
		assert.Empty(t, req.SourceGradeLevels.Value,
			"switching to a Klassenfilter must not be blocked by a filter the client never sent")
	})

	t.Run("submitted grade filter drops the stored class filter", func(t *testing.T) {
		req := &updateTemplateRequest{
			TargetGroupType:   activitiesModel.TargetGroupTypeAngebot,
			SourceGradeLevels: nullableIntSlice{Set: true, Value: []int{2}},
		}
		applyOfferingSourcePresence(req, storedClasses)
		assert.Equal(t, []int{2}, req.SourceGradeLevels.Value)
		assert.Empty(t, req.SourceSchoolClasses.Value)
	})

	t.Run("clearing the source clears both filters", func(t *testing.T) {
		req := &updateTemplateRequest{
			TargetGroupType:       activitiesModel.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: nullableInt64Slice{Set: true, Value: nil},
		}
		applyOfferingSourcePresence(req, storedClasses)
		assert.Empty(t, req.SourceSchoolClasses.Value)
		assert.Empty(t, req.SourceGradeLevels.Value)
	})

	t.Run("submitted class filter without a source remains for validation", func(t *testing.T) {
		req := &updateTemplateRequest{
			TargetGroupType:     activitiesModel.TargetGroupTypeAngebot,
			SourceSchoolClasses: nullableStringSlice{Set: true, Value: []string{"1b"}},
		}
		applyOfferingSourcePresence(req, templateResponse{TargetGroupType: activitiesModel.TargetGroupTypeAngebot})
		assert.Equal(t, []string{"1b"}, req.SourceSchoolClasses.Value,
			"the model must reject a submitted class filter without an offering source")
	})
}
