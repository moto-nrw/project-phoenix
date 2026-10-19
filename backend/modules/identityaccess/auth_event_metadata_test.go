package identityaccess_test

import (
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func TestAuthEventMetadata(t *testing.T) {
	t.Parallel()

	schoolID, operatorID, pendingID := int64(4082), int64(2580), int64(3754)
	access := &identityaccess.TenantAccessEvidence{
		SchoolID: schoolID, SchoolName: "OGS Nord", Role: "admin", RemovedRoles: []string{"betreuer"},
		AccountDeactivated: true, OperatorID: operatorID,
	}
	accessBase := []identityaccess.AuthEventMetadataEntry{
		{Key: "school_id", Value: schoolID}, {Key: "school_name", Value: "OGS Nord"}, {Key: "operator_id", Value: operatorID},
	}
	tests := []struct {
		name  string
		event identityaccess.AuthEvent
		want  []identityaccess.AuthEventMetadataEntry
	}{
		{name: "no evidence", event: identityaccess.AuthEvent{Type: identityaccess.AuthEventLogin}},
		{
			name:  "access granted",
			event: identityaccess.AuthEvent{Type: identityaccess.AuthEventTenantAccessGranted, TenantAccess: access},
			want:  append(append([]identityaccess.AuthEventMetadataEntry{}, accessBase...), identityaccess.AuthEventMetadataEntry{Key: "role", Value: "admin"}),
		},
		{
			name:  "role changed",
			event: identityaccess.AuthEvent{Type: identityaccess.AuthEventTenantRoleChanged, TenantAccess: access},
			want: append(append([]identityaccess.AuthEventMetadataEntry{}, accessBase...),
				identityaccess.AuthEventMetadataEntry{Key: "role", Value: "admin"}, identityaccess.AuthEventMetadataEntry{Key: "removed_roles", Value: []string{"betreuer"}}),
		},
		{
			name:  "access revoked",
			event: identityaccess.AuthEvent{Type: identityaccess.AuthEventTenantAccessRevoked, TenantAccess: access},
			want:  append(append([]identityaccess.AuthEventMetadataEntry{}, accessBase...), identityaccess.AuthEventMetadataEntry{Key: "account_deactivated", Value: true}),
		},
		{
			name: "revoked sessions and pending wipe",
			event: identityaccess.AuthEvent{
				Type:            identityaccess.AuthEventTokenRevoked,
				RevokedSessions: &identityaccess.RevokedSessionsEvidence{PortalScope: "parent", FamilyFingerprint: "fp", Reason: "logout_all", Count: 3},
				PendingWipe:     &identityaccess.PendingWipeEvidence{Reason: "password_reset"},
			},
			want: []identityaccess.AuthEventMetadataEntry{
				{Key: "portal_scope", Value: "parent"}, {Key: "family_fingerprint", Value: "fp"}, {Key: "reason", Value: "logout_all"},
				{Key: "revoked_token_count", Value: 3}, {Key: "reason", Value: "password_reset"}, {Key: "pending_account_wide_wipe", Value: true},
			},
		},
		{
			name:  "completed wipe",
			event: identityaccess.AuthEvent{Type: identityaccess.AuthEventAccountWideWipeCompleted, CompletedWipe: &identityaccess.CompletedWipeEvidence{PendingEventID: pendingID}},
			want:  []identityaccess.AuthEventMetadataEntry{{Key: "pending_event_id", Value: pendingID}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.event.Metadata(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Metadata() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
