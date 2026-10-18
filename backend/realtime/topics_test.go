package realtime

import "testing"

func TestEducationGroupTopicPrefixesTheGroupID(t *testing.T) {
	t.Parallel()
	groupID := int64(42)
	if got := EducationGroupTopic(groupID); got != "edu:42" {
		t.Errorf("EducationGroupTopic(%d) = %q", groupID, got)
	}
}
