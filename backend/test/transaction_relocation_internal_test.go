package test

import (
	"os"
	"path/filepath"
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
			err := transactionScopeSubset(current, previous, moves, fingerprint, nil)
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

func TestTransactionEngineReceiverRelocationsStayStrict(t *testing.T) {
	t.Parallel()
	const pkg = "modules/fleet/internal/application"
	const old = pkg + "/engine.go:Engine.FindDevice:e.service.deps.Tx.AdminScope#1"
	const moved = pkg + "/queries.go:*Service.FindDevice:s.deps.Tx.AdminScope#1"
	const reason = "existing discovery"
	for _, tc := range []struct {
		name, key, reason, retained string
		oldCount, newCount          int
		copy, pass                  bool
	}{
		{name: "same operation and scope", pass: true},
		{name: "multiple occurrences", oldCount: 2, newCount: 2, pass: true},
		{name: "renamed operation", key: strings.Replace(moved, "FindDevice", "LookupDevice", 1)},
		{name: "changed scope", key: strings.Replace(moved, "AdminScope", "RunWrite", 1)},
		{name: "different package", key: strings.Replace(moved, "/fleet/", "/other/", 1)},
		{name: "extra scope", newCount: 2},
		{name: "dropped scope", oldCount: 2},
		{name: "changed occurrence", key: strings.Replace(moved, "#1", "#2", 1)},
		{name: "changed reason", reason: "different discovery"},
		{name: "value service receiver", key: strings.Replace(moved, "*Service", "Service", 1)},
		{name: "other receiver", key: strings.Replace(moved, "*Service", "*Worker", 1)},
		{name: "Engine method retained in another file", retained: "func (e Engine) FindDevice() {}"},
		{name: "pointer Engine method retained", retained: "func (e *Engine) FindDevice() {}"},
		{name: "other Engine method retained", retained: "func (e Engine) Other() {}", pass: true},
		{name: "copied allowance", copy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, pkg)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			key := tc.key
			if key == "" {
				key = moved
			}
			file, operation, _ := strings.Cut(key, ":")
			method := strings.Split(operation, ":")[0]
			receiver, name, _ := strings.Cut(method, ".")
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0o755); err != nil {
				t.Fatal(err)
			}
			code := "package application\nfunc (s " + receiver + ") " + name + "() { s.deps.Tx.AdminScope(ctx, work) }\n"
			if err := os.WriteFile(filepath.Join(root, file), []byte(code), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.retained != "" {
				if err := os.WriteFile(filepath.Join(dir, "retained.go"), []byte("package application\n"+tc.retained), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			previous := map[string]string{old: reason}
			currentReason := reason
			if tc.reason != "" {
				currentReason = tc.reason
			}
			current := map[string]string{key: currentReason}
			if tc.oldCount == 2 {
				previous[strings.Replace(old, "#1", "#2", 1)] = reason
			}
			if tc.newCount == 2 {
				current[strings.Replace(key, "#1", "#2", 1)] = currentReason
			}
			if tc.copy {
				current[old] = reason
			}
			fingerprint := transactionFingerprints(root, "base", func(...string) ([]byte, error) {
				return []byte("package application\nfunc (e Engine) FindDevice() { e.service.deps.Tx.AdminScope(ctx, work) }"), nil
			})
			err := transactionScopeSubset(current, previous, nil, fingerprint, removedTransactionReceiverMethods(root))
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%t, got %v", tc.pass, err)
			}
		})
	}
}
