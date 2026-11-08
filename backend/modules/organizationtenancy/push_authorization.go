package organizationtenancy

import "context"

// ReleasePushCandidate carries only schools/groups of currently permitted
// children from the stored announcement's audience, never arbitrary members.
type ReleasePushCandidate struct {
	Key       int64   `json:"key"`
	ReleaseID int64   `json:"release_id"`
	SchoolID  int64   `json:"school_id"`
	GroupIDs  []int64 `json:"group_ids"`
}

type ReleasePushValidity interface {
	ValidReleasePushes(context.Context, int64, []ReleasePushCandidate) (map[int64]bool, error)
}
