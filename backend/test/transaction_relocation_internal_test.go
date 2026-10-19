package test

import (
	"strings"
	"testing"
)

func TestTransactionRegisterRelocationsStayStrict(t *testing.T) {
	t.Parallel()
	const old = "old/worker.go:run:runtime.WithinAdmin#1"
	const moved = "new/worker.go:run:runtime.WithinAdmin#1"
	for _, tc := range []struct {
		name                                string
		key                                 string
		reason, body                        string
		copy, extra, unverified, otherScope bool
		pass                                bool
	}{
		{name: "pure path move", key: moved, pass: true},
		{name: "changed reason", key: moved, reason: "wider work"},
		{name: "changed body", key: moved, body: "changed"},
		{name: "changed operation", key: strings.Replace(moved, "WithinAdmin", "WithinTenant", 1)},
		{name: "changed function", key: strings.Replace(moved, ":run:", ":other:", 1)},
		{name: "changed filename", key: strings.Replace(moved, "worker.go", "other.go", 1)},
		{name: "unverified move", key: moved, unverified: true},
		{name: "copied allowance", key: moved, copy: true},
		{name: "additional entry", key: moved, extra: true},
		{name: "different scope", key: moved, otherScope: true},
		{name: "retained reason change", key: old, reason: "different"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason := tc.reason
			if reason == "" {
				reason = "existing command"
			}
			previous := map[string]string{old: "existing command"}
			current := map[string]string{tc.key: reason}
			moves := map[string]string{"old": "new"}
			if tc.unverified {
				moves = nil
			}
			if tc.copy {
				current[old] = "existing command"
			}
			if tc.extra {
				current[strings.Replace(moved, "#1", "#2", 1)] = reason
			}
			if tc.otherScope {
				previous = nil
			}
			fingerprint := func(_ string, before bool) (string, error) {
				if !before && tc.body != "" {
					return tc.body, nil
				}
				return "unchanged body", nil
			}
			err := transactionScopeSubset(current, previous, moves, fingerprint)
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%t, got %v", tc.pass, err)
			}
		})
	}
}

func TestTransactionBodyFingerprintDoesNotIncludeFilePath(t *testing.T) {
	t.Parallel()
	const code = "package fixture\nfunc run(){ runtime.WithinAdmin(ctx, work) }\n"
	old, err := transactionFunctionFingerprints("old/worker.go", []byte(code))
	if err != nil {
		t.Fatal(err)
	}
	moved, err := transactionFunctionFingerprints("new/worker.go", []byte(code))
	if err != nil {
		t.Fatal(err)
	}
	if old["run"] != moved["run"] {
		t.Fatal("path-only move changed the body fingerprint")
	}
	changed, err := transactionFunctionFingerprints("new/worker.go", []byte(strings.Replace(code, "work)", "widerWork)", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if old["run"] == changed["run"] {
		t.Fatal("changed operation body kept its fingerprint")
	}
}
