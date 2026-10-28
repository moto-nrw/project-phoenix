package domain

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMFAPrivilegedPermissionsCannotBeExempted(t *testing.T) {
	t.Parallel()
	for _, grant := range []string{"users:manage", "roles:update", "permissions:create", "auth:manage", "config:manage", "system:manage", "staff_documents:health", "student_documents:health", "staff:financial", "staff:stammdaten", "staff:documents", "time_tracking:manage", "vacation:approve", "enrollment_uploads:read", "enrollment_uploads:*", "enrollment*:read", "admin:*", "*:*", "*:health", "*:manage", "user*:manage", "staff*:h*"} {
		for _, policy := range []MFAPolicy{MFAPolicyForMode(MFAModeOff), MFAPolicyForMode(MFAModeRequiredAdmins), MFAPolicyForced(false)} {
			assert.True(t, policy.RequiredFor([]string{grant}), "grant %q must require MFA even with off/force_off", grant)
		}
	}
	for _, policy := range []MFAPolicy{MFAPolicyForMode(MFAModeOff), MFAPolicyForMode(MFAModeRequiredAdmins), MFAPolicyForced(false)} {
		assert.False(t, policy.RequiredFor([]string{"users:read"}), "child health_info stays readable without imposing the privileged MFA floor")
	}
	assert.False(t, MFAPolicyForMode(MFAModeOff).RequiredFor([]string{"users:read", "users:create", "users:update", "users:delete"}), "person-record editing does not administer accounts or grant roles")
	assert.False(t, MFAPolicyForMode(MFAModeRequiredAdmins).RequiredFor([]string{"admin"}), "role names are not permissions")
}

// Every catalog change requires an explicit risk classification, not just
// additions whose names happen to contain "admin". This catches new resources.
func TestMFAPermissionCatalogIsClassified(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../../auth/authorize/permissions/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog []struct {
		Resource string
		Action   string
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	classified := map[string]bool{
		"*:*":                                true,
		"activities:assign":                  false,
		"activities:create":                  false,
		"activities:delete":                  false,
		"activities:enroll":                  false,
		"activities:list":                    false,
		"activities:manage":                  false,
		"activities:manage_categories":       false,
		"activities:read":                    false,
		"activities:update":                  false,
		"admin:*":                            true,
		"auth:manage":                        true,
		"calendar:manage":                    false,
		"calendar:own":                       false,
		"calendar:school":                    false,
		"class_day:absence_report_write":     false,
		"class_day:all_classes":              false,
		"class_day:absence_report_write":     false,
		"class_day:arrival_exception_write":  false,
		"class_day:class_trip_write":         false,
		"class_day:day_hint_write":           false,
		"class_day:handover_write":           false,
		"class_day:homework_write":           false,
		"class_day:parent_letter_write":      false,
		"class_day:morning_attendance_write": false,
		"class_day:read":                     false,
		"config:manage":                      true,
		"config:read":                        false,
		"config:update":                      false,
		"display:manage":                     false,
		"display:read":                       false,
		"enrollment_uploads:read":            true,
		"feedback:create":                    false,
		"feedback:delete":                    false,
		"feedback:list":                      false,
		"feedback:manage":                    false,
		"feedback:read":                      false,
		"files:manage":                       false,
		"grade_transitions:apply":            false,
		"grade_transitions:create":           false,
		"grade_transitions:delete":           false,
		"grade_transitions:read":             false,
		"grade_transitions:update":           false,
		"groups:assign":                      false,
		"groups:create":                      false,
		"groups:delete":                      false,
		"groups:list":                        false,
		"groups:manage":                      false,
		"groups:read":                        false,
		"groups:update":                      false,
		"guardians:financial":                false,
		"iot:manage":                         false,
		"iot:read":                           false,
		"iot:update":                         false,
		"meals:manage":                       false,
		"meals:orders":                       false,
		"meals:read":                         false,
		"meals:register":                     false,
		"permissions:create":                 true,
		"permissions:delete":                 true,
		"permissions:read":                   true,
		"permissions:update":                 true,
		"roles:create":                       true,
		"roles:delete":                       true,
		"roles:read":                         true,
		"roles:update":                       true,
		"rooms:create":                       false,
		"rooms:delete":                       false,
		"rooms:list":                         false,
		"rooms:manage":                       false,
		"rooms:read":                         false,
		"rooms:update":                       false,
		"schedules:create":                   false,
		"schedules:delete":                   false,
		"schedules:list":                     false,
		"schedules:manage":                   false,
		"schedules:read":                     false,
		"schedules:update":                   false,
		"staff:documents":                    true,
		"staff:financial":                    true,
		"staff:manage":                       true,
		"staff:stammdaten":                   true,
		"staff_documents:health":             true,
		"staff_notices:read":                 false,
		"student_documents:health":           true,
		"student_documents:legal":            false,
		"students:read":                      false,
		"substitutions:create":               false,
		"substitutions:delete":               false,
		"substitutions:list":                 false,
		"substitutions:manage":               false,
		"substitutions:read":                 false,
		"substitutions:update":               false,
		"supervision:own":                    false,
		"surveys:manage":                     false,
		"system:manage":                      true,
		"time_tracking:manage":               true,
		"time_tracking:own":                  false,
		"users:absence":                      false,
		"users:checkin":                      false,
		"users:create":                       false,
		"users:delete":                       false,
		"users:list":                         false,
		"users:manage":                       true,
		"users:read":                         false,
		"users:update":                       false,
		"vacation:approve":                   true,
		"visits:create":                      false,
		"visits:delete":                      false,
		"visits:list":                        false,
		"visits:manage":                      false,
		"visits:read":                        false,
		"visits:update":                      false,
	}
	for _, entry := range catalog {
		name := entry.Resource + ":" + entry.Action
		required, found := classified[name]
		if !found {
			t.Errorf("classify the MFA risk of new permission %q", name)
			continue
		}
		assert.Equal(t, required, HasPrivilegedMFAPermission([]string{name}), name)
		if required {
			assert.Contains(t, PrivilegedMFAPermissions, name, "classify concrete targets explicitly so matching wildcard grants cannot bypass new rights")
		}
		delete(classified, name)
	}
	assert.Empty(t, classified, "remove stale classifications")
}

func TestMFABestandQueryUsesTheSamePrivilegeFloor(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../../../docs/operations/sql/mfa-pflicht-3841-counts.sql")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(data), "VALUES")
	end := strings.Index(string(data), "), targets")
	if start < 0 || end < start {
		t.Fatal("missing inventory privilege values")
	}
	var listed []string
	for _, line := range strings.Split(string(data)[start+len("VALUES"):end], "\n") {
		value := strings.TrimSpace(line)
		if value != "" {
			listed = append(listed, strings.Trim(value, " (),\n'"))
		}
	}
	assert.ElementsMatch(t, PrivilegedMFAPermissions, listed)
}
