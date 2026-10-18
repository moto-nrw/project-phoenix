package timetable

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateFilters(t *testing.T) {
	t.Parallel()
	isTemplate, groupID := true, int64(52)
	offeringIDs := []int64{61, 62}
	assert.Equal(t, GroupFilter{IsTemplate: &isTemplate, ActiveOnly: true, SeriesForGroupID: &groupID, OrderByID: true}, ActiveTemplateSeries(groupID))
	assert.Equal(t, GroupFilter{IsTemplate: &isTemplate, ActiveOnly: true, HasOfferingSource: true, OrderByID: true}, ActiveOfferingSourcedTemplates())
	assert.Equal(t, GroupFilter{IsTemplate: &isTemplate, ActiveOnly: true, SourceOfferingIDs: offeringIDs, OrderByID: true}, ActiveTemplatesSourcedFrom(offeringIDs))
}
