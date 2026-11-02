package test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSideEffectNoticeRegisterUsesTheBranchBase(t *testing.T) {
	t.Parallel()
	key := "modules/example/http/handler.go:serve:AfterCommit@operation#1"
	original := losableSideEffectNotice{Reason: "realtime hint", Fingerprint: "original body"}
	upstream := losableSideEffectNotice{Reason: original.Reason, Fingerprint: "upstream body"}
	missingBase := false
	run := func(args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify origin/development^{commit}":
			if missingBase {
				return nil, errors.New("missing fetched base")
			}
			return []byte("upstream\n"), nil
		case "merge-base HEAD upstream":
			return []byte("fork\n"), nil
		case "ls-tree --name-only fork -- backend/test/durable_side_effect_notices.json", "ls-tree --name-only upstream -- backend/test/durable_side_effect_notices.json":
			return []byte("backend/test/durable_side_effect_notices.json\n"), nil
		case "show fork:backend/test/durable_side_effect_notices.json":
			return json.Marshal(map[string]losableSideEffectNotice{key: original})
		case "show upstream:backend/test/durable_side_effect_notices.json":
			return json.Marshal(map[string]losableSideEffectNotice{key: upstream})
		default:
			t.Fatalf("unexpected git command: %v", args)
			return nil, nil
		}
	}
	current := map[string]losableSideEffectNotice{key: original}
	if err := sideEffectRegisterAgainstBase(run, current); err != nil {
		t.Fatalf("unchanged branch notice rejected after upstream advanced: %v", err)
	}
	changed := map[string]losableSideEffectNotice{key: {Reason: original.Reason, Fingerprint: "candidate body"}}
	if err := sideEffectRegisterAgainstBase(run, changed); err == nil {
		t.Fatal("a changed candidate body must still fail against its branch base")
	}
	added := map[string]losableSideEffectNotice{key: original, "modules/example/http/other.go:serve:go@new#1": original}
	if err := sideEffectRegisterAgainstBase(run, added); err == nil {
		t.Fatal("an additional candidate allowance must still fail")
	}
	missingBase = true
	if err := sideEffectRegisterAgainstBase(run, current); err == nil {
		t.Fatal("a missing fetched base must not bypass the ratchet")
	}
}

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
