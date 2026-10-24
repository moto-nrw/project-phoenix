package analytics

import (
	"maps"
	"strconv"
	"strings"
)

// groupIdentifyEvent names a PostHog group and sets its properties.
const groupIdentifyEvent = "$groupidentify"

// schoolGroupType is the PostHog group type of a school. The presence events
// already link it (`$groups`), so it holds group type index 0.
const schoolGroupType = "school"

// School is one school as the usage dashboards name it.
type School struct {
	ID   int64
	Name string
}

// IdentifySchools names every school for the usage dashboards
// (scripts/posthog-dashboards.mjs): one group per school with its display
// name. A school name is organisation data, no personal data. The tracker
// qualifies each group key with the deployment, so the demo and a local
// instance never rename a production school.
func IdentifySchools(tracker interface {
	Capture(distinctID, event string, props map[string]any)
}, schools []School) int {
	identified := 0
	for _, school := range schools {
		name := strings.TrimSpace(school.Name)
		if school.ID <= 0 || name == "" {
			continue
		}
		tracker.Capture("", groupIdentifyEvent, map[string]any{
			"$group_type": schoolGroupType,
			"$group_key":  strconv.FormatInt(school.ID, 10),
			"$group_set":  map[string]any{"name": name},
		})
		identified++
	}
	return identified
}

// qualifySchoolGroup turns a school's $groupidentify into the group
// `<deployment>:<school_id>`, the key the dashboards look a name up by. The
// event goes out under the group's own distinct ID, as PostHog's SDKs send
// it. It reports false for any other $groupidentify, which the tracker drops:
// the backend names schools only.
func qualifySchoolGroup(properties map[string]any, deployment string) (string, bool) {
	key, _ := properties["$group_key"].(string)
	set, _ := properties["$group_set"].(map[string]any)
	if properties["$group_type"] != schoolGroupType || key == "" || set == nil {
		return "", false
	}
	key = deployment + ":" + key
	properties["$group_key"] = key
	groupSet := make(map[string]any, len(set)+1)
	maps.Copy(groupSet, set)
	groupSet["deployment"] = deployment
	properties["$group_set"] = groupSet
	// A group is no person: the flag stays unset, as in PostHog's SDKs.
	delete(properties, "$process_person_profile")
	return "$" + schoolGroupType + "_" + key, true
}
