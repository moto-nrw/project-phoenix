package authorize

import (
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// Guardian permission checks and grants are authorization concerns, not data
// facts on the model entities. They lived on PersonGuardian / StudentGuardian
// (HasPermission / GrantPermission / RevokePermission); they move here per
// issue #586, Rule 12. The PersonGuardian variants operate on the model's JSON
// `Permissions` string; the StudentGuardian variant operates on its decoded map.

// PersonGuardianHasPermission reports whether the guardian relationship grants
// the named permission. An empty or invalid permissions JSON yields false.
func PersonGuardianHasPermission(pg *users.PersonGuardian, permission string) bool {
	if pg == nil || pg.Permissions == "" {
		return false
	}
	perms := map[string]bool{}
	if err := json.Unmarshal([]byte(pg.Permissions), &perms); err != nil {
		return false
	}
	return perms[permission]
}

// PersonGuardianGrantPermission grants the named permission on the guardian
// relationship and re-serializes the JSON `Permissions` field. Invalid existing
// JSON is reset to a fresh map before the grant is applied.
func PersonGuardianGrantPermission(pg *users.PersonGuardian, permission string) error {
	if pg == nil {
		return nil
	}
	perms := map[string]bool{}
	if pg.Permissions != "" {
		if err := json.Unmarshal([]byte(pg.Permissions), &perms); err != nil {
			perms = map[string]bool{}
		}
	}
	perms[permission] = true
	bytes, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	pg.Permissions = string(bytes)
	return nil
}

// PersonGuardianRevokePermission removes the named permission from the guardian
// relationship and re-serializes the JSON `Permissions` field. A missing or
// empty permission set is a no-op.
func PersonGuardianRevokePermission(pg *users.PersonGuardian, permission string) error {
	if pg == nil || pg.Permissions == "" {
		return nil
	}
	perms := map[string]bool{}
	if err := json.Unmarshal([]byte(pg.Permissions), &perms); err != nil {
		return nil
	}
	delete(perms, permission)
	bytes, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	pg.Permissions = string(bytes)
	return nil
}

// StudentGuardianHasPermission reports whether the student-guardian relationship
// grants the named permission. A boolean value is returned directly; any other
// non-nil value is treated as granted, matching the prior model behavior.
func StudentGuardianHasPermission(sg *users.StudentGuardian, permission string) bool {
	if sg == nil {
		return false
	}
	value, exists := sg.GetPermissions()[permission]
	if !exists {
		return false
	}
	if boolValue, ok := value.(bool); ok {
		return boolValue
	}
	return value != nil
}
