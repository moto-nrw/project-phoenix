package test

import "testing"

func TestSideEffectNoticeRegisterAcceptsOnlyUnchangedPathRelocations(t *testing.T) {
	t.Parallel()
	const operation = ":Scheduler.afterCommit:RegisterAfterCommit@operation#1"
	oldKey := "services/scheduler/tenant_runtime.go" + operation
	newKey := "modules/schedulerruntime/internal/application/tenant_runtime.go" + operation
	retainedKey := "modules/example/http/handler.go:serve:go@retained#1"
	notice := losableSideEffectNotice{Reason: "process-local cache hint", Fingerprint: "unchanged body"}
	retained := losableSideEffectNotice{Reason: "realtime invalidation", Fingerprint: "retained body"}
	base := map[string]losableSideEffectNotice{oldKey: notice, retainedKey: retained}
	changedBody := notice
	changedBody.Fingerprint = "durable write"
	changedReason := notice
	changedReason.Reason = "a different loss case"

	cases := []struct {
		name    string
		current map[string]losableSideEffectNotice
		wantErr bool
	}{
		{"path only", map[string]losableSideEffectNotice{newKey: notice, retainedKey: retained}, false},
		{"path and body", map[string]losableSideEffectNotice{newKey: changedBody, retainedKey: retained}, true},
		{"path and reason", map[string]losableSideEffectNotice{newKey: changedReason, retainedKey: retained}, true},
		{"path and operation", map[string]losableSideEffectNotice{newKey + "changed": notice, retainedKey: retained}, true},
		{"additional entry", map[string]losableSideEffectNotice{oldKey: notice, newKey: notice, retainedKey: retained}, true},
		{"reuse moved entry", map[string]losableSideEffectNotice{newKey: notice, "other/location.go" + operation: notice}, true},
		{"retained reason changed", map[string]losableSideEffectNotice{oldKey: changedReason, retainedKey: retained}, true},
		{"retirement and relocation", map[string]losableSideEffectNotice{newKey: notice}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := sideEffectRegisterSubset(tc.current, base)
			if (err != nil) != tc.wantErr {
				t.Fatalf("relocation error = %v, want error %t", err, tc.wantErr)
			}
		})
	}
}
