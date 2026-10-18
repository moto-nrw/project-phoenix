package messaging

import "encoding/json"

// PermissionValues interprets a stored relationship permission, never membership.
type PermissionValues interface {
	PermissionGranted(any) bool
	PermissionGrantedJSON(json.RawMessage, string) bool
}

// ContentFingerprints identifies the canonical declaration bytes through Security.
type ContentFingerprints interface{ Content([]byte) string }
