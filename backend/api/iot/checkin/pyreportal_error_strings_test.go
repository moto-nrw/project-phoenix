// Package checkin — PyrePortal error-string guard (WP-B10).
//
// This is a regression tripwire, not a behavioral test. The PyrePortal
// kiosk app (see ../../../../PyrePortal/src/services/api.ts, the
// ERROR_MESSAGE_MAPPINGS const) hard-codes every error substring it
// expects from the IoT checkin boundary and maps each to a German UI
// translation. If the backend mutates one of these strings, PyrePortal
// will silently fall back to displaying the raw English error — the
// German mapping breaks with no compile-time signal.
//
// The guard below reads each checkin-relevant source file as plain text
// and asserts every mapped substring is still present. A future PR that
// changes a wording ("failed to create visit record" → "could not
// create visit") will fail this test loudly, flagging the PyrePortal
// coupling that must be updated in lockstep.
//
// Enumeration methodology: scanned PyrePortal/src/services/api.ts
// end-to-end against the four checkin-boundary routes:
//
//	POST /api/iot/checkin
//	POST /api/iot/pickup-query
//	POST /api/iot/ping
//	GET  /api/iot/status
//
// All four go through device auth middleware (device.DeviceAuthenticator)
// which can emit the 401/403 strings in auth/device/errors.go. The checkin
// handlers/workflow add the rest. Strings that only fire on other IoT
// subtrees (sessions, attendance, rfid, feedback) are OUT of scope for
// this guard even though they appear in PyrePortal's mapping.
package checkin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// pyreportalCheckinErrorStrings are the substrings PyrePortal's mapping
// matches on for errors emitted by the four checkin-boundary routes.
//
// Source rules:
//   - Lift from ERROR_MESSAGE_MAPPINGS in PyrePortal/src/services/api.ts.
//   - Only include a string if it can surface from /checkin, /pickup-query,
//     /ping, or /status — not strings unique to /session, /attendance,
//     /rfid/*, or /feedback.
//   - Use PyrePortal's SUBSTRING (not the full emitted phrase) so refactors
//     that keep the substring (e.g. "RFID tag not assigned to X" →
//     "RFID tag not assigned to Y") don't trip the guard needlessly.
var pyreportalCheckinErrorStrings = []string{
	// Device auth errors — fire on every checkin-boundary route
	"invalid device API key",
	"device API key is required",
	"invalid API key format",
	"invalid staff PIN",
	"staff PIN is required",
	"staff account is locked due to failed PIN attempts",
	"maximum PIN attempts exceeded",
	"device is not active",
	"device is offline",

	// Capacity errors (POST /checkin)
	"ROOM_CAPACITY_EXCEEDED",
	"room capacity exceeded",
	"ACTIVITY_CAPACITY_EXCEEDED",
	"activity capacity exceeded",

	// Duplicate active visit (POST /checkin) — 409 Conflict path added in
	// migration 1.15.47. PyrePortal substring-matches the code to surface
	// "Kinder*in ist bereits angemeldet" instead of the generic conflict
	// fallback. The English message phrasing is the canonical
	// active-service error from services/active/errors.go and is
	// surfaced both as the response Message and as the typed Error()
	// string so existing PyrePortal builds without the new code mapping
	// still display a sensible message.
	"STUDENT_ALREADY_ACTIVE",
	"student already has an active visit",

	// RFID lookup (POST /checkin, POST /pickup-query)
	"RFID tag not found",
	"RFID tag not assigned",
	"student RFID tag required for pickup query",

	// Supervisor scan (POST /checkin)
	"no active session",

	// Visit / room / activity (POST /checkin)
	"failed to end visit record",
	"failed to create visit record",
	"failed to get room information",
	"failed to check room capacity",
	"failed to get activity information",
	"failed to check activity capacity",
	"error finding active groups in room",
	"room_id is required for check-in",

	// Special rooms (POST /checkin via schulhof/wc autocreate)
	"schulhof activity not configured",
	"WC activity not configured",
	"WC activity auto-create requires staff context",
	"no active groups in specified room",
	"failed to create Schulhof session",
	"failed to create WC session",
	"failed to create session",
}

// guardSources are the non-test source files the guard scans. Staying
// narrow (explicit file list, no wildcards) prevents false positives from
// test fixtures that happen to contain the same literal.
//
// Paths are relative to this test file (api/iot/checkin/).
//
// Scope limitation: the guard only covers strings that originate inside
// this file list (the IoT checkin package + device auth + iot/common
// errors). If the checkin flow begins to surface error strings that come
// from a deeper layer — for example a new active/schedule service error
// bubbled up through the handler and translated by PyrePortal — add that
// source file here. Otherwise a wording change there silently breaks the
// mapping and this guard will not catch it.
var guardSources = []string{
	// Checkin package itself
	"workflow.go",
	"handlers.go",
	"helpers.go",
	"schulhof.go",
	"wc.go",
	"types.go",
	"resource.go",
	// IoT common errors (capacity, shared RFID message constant)
	"../common/errors.go",
	// Device auth errors — middleware on every checkin route
	"../../../auth/device/errors.go",
}

// TestPyrePortalCheckinErrorStringsGuard asserts every PyrePortal-mapped
// substring is still present in the sources enumerated above. The test
// name deliberately contains "PyrePortal" so failures in CI logs land the
// reader directly on this file — and the file-top comment explains the
// coupling.
func TestPyrePortalCheckinErrorStringsGuard(t *testing.T) {
	// Read every guard source into one concatenated blob. We don't care
	// which file a substring appears in — only that it appears SOMEWHERE
	// in the IoT checkin + device auth surface.
	var blob strings.Builder
	for _, rel := range guardSources {
		abs, err := filepath.Abs(rel)
		if err != nil {
			t.Fatalf("resolving %s: %v", rel, err)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			t.Fatalf("reading %s: %v", abs, err)
		}
		blob.Write(data)
		blob.WriteByte('\n')
	}
	corpus := blob.String()

	for _, want := range pyreportalCheckinErrorStrings {
		assert.Containsf(t, corpus, want,
			"PyrePortal error mapping depends on the substring %q being present in the "+
				"IoT checkin or device auth sources. It is missing — changing this string "+
				"silently breaks PyrePortal's German translation for this error. "+
				"If the change is intentional, update PyrePortal/src/services/api.ts "+
				"ERROR_MESSAGE_MAPPINGS and this test in the same PR.",
			want,
		)
	}
}
