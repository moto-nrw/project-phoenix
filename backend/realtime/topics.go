package realtime

import "strconv"

// EducationGroupTopic is the topic of an educational group's live updates
// ("edu:<id>"): staff of the group subscribe to it, and attendance changes of
// its children are broadcast to it. A session topic is the session id alone.
func EducationGroupTopic(educationGroupID int64) string {
	return "edu:" + strconv.FormatInt(educationGroupID, 10)
}
