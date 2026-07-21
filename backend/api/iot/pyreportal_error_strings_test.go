// Package iot — PyrePortal error-string guard.
//
// This is a regression tripwire, not a behavioral test. The PyrePortal
// kiosk app (ERROR_MESSAGE_MAPPINGS in PyrePortal/src/services/apiErrors.ts)
// hard-codes every error substring it expects from the /api/iot/* boundary
// and maps each to a German UI translation. If the backend mutates one of
// these strings, PyrePortal silently falls back to displaying the raw
// English error — the German mapping breaks with no compile-time signal.
//
// The guard reads every non-test .go file under api/iot/ (all sub-packages,
// so it survives package moves/merges) plus the auth/device, services/active
// and api/common error sources whose strings surface through IoT routes, and
// asserts every mapped substring is still present somewhere in that corpus.
//
// History: this guard originally lived in api/iot/checkin/ with a hardcoded
// file list covering only the four checkin-boundary routes (POST /checkin,
// POST /pickup-query, POST /ping, GET /status). Issue #575 B0 moved it here
// and extended it to the session / attendance / data / rfid / feedback
// routes so the IoT package consolidation cannot silently orphan a string.
package iot

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pyreportalErrorStrings are the substrings PyrePortal's mapping matches on
// for errors emitted by /api/iot/* routes.
//
// Source rules:
//   - Lift from ERROR_MESSAGE_MAPPINGS in PyrePortal/src/services/apiErrors.ts.
//   - Only include a string the backend actually emits. PyrePortal carries a
//     few orphaned mappings for strings no production code has ever emitted
//     ("staff account is locked due to failed PIN attempts", "maximum PIN
//     attempts exceeded", "device is offline" — audit A5; "staff RFID
//     authentication must be done via session management" and "no active
//     visit found for student" — verified absent 2026-07-12). Those are
//     PyrePortal cleanup candidates, not backend contracts.
//   - Use PyrePortal's SUBSTRING (not the full emitted phrase) so refactors
//     that keep the substring don't trip the guard needlessly.
var pyreportalErrorStrings = []string{
	// Device auth errors — fire on every device-authenticated route
	// (origin: auth/device/errors.go)
	"invalid device API key",
	"device API key is required",
	"invalid API key format",
	"invalid staff PIN",
	"staff PIN is required",
	"device is not active",

	// Capacity errors (POST /checkin)
	"ROOM_CAPACITY_EXCEEDED",
	"room capacity exceeded",
	"Room capacity exceeded",
	"ACTIVITY_CAPACITY_EXCEEDED",
	"activity capacity exceeded",
	"Activity capacity exceeded",

	// Duplicate active visit (POST /checkin) — 409 Conflict. The canonical
	// phrasing is services/active.ErrStudentAlreadyActive; the api/iot/common
	// response type references that sentinel so the two layers cannot drift.
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
	"person is not a student",

	// Special rooms (POST /checkin via schulhof/wc autocreate)
	"schulhof activity not configured",
	"WC activity not configured",
	"WC activity auto-create requires staff context",
	"no active groups in specified room",
	"failed to create Schulhof session",
	"failed to create WC session",
	"failed to create session",

	// Sessions (POST/GET/PUT /session*) — extended coverage, issue #575 B0
	"device is already running an activity session",
	"no active session to end",
	"invalid session ID",
	"activity_id is required",
	"at least one supervisor",

	// Attendance (POST /attendance/toggle, GET /attendance/status/{rfid})
	"RFID parameter is required",
	"student has no attendance record for today",
	"destination must be 'zuhause' or 'unterwegs'",
	"destination is required for confirm_daily_checkout",

	// Data queries (GET /rfid/{tagId})
	"tagId parameter is required",

	// Staff RFID (mounted at /staff)
	"invalid staff ID",
	"staff not found",
	"staff has no RFID tag assigned",
	"failed to get person data for staff",

	// Feedback (POST /feedback)
	"student_id is required",
	"value is required",
	"student not found",
}

// Staff-clock screens branch on stable codes rather than English prose. Keep
// those codes under the same cross-repository tripwire as legacy substrings.
var pyreportalErrorCodes = []string{
	"invalid_staff_clock_request",
	"invalid_rfid_tag",
	"rfid_tag_not_found",
	"rfid_tag_inactive",
	"rfid_tag_not_staff",
	"reopen_status_conflict",
	"planned_start_not_reached",
	"deviation_reason_required",
	"invalid_staff_clock_state",
}

// extraGuardSources are files OUTSIDE api/iot whose error strings surface
// through IoT routes: device auth middleware, active-service sentinels
// bubbled by the checkin/session workflows, and the api/common message
// constants referenced by IoT handlers.
//
// Paths are relative to the backend module root.
var extraGuardSources = []string{
	"auth/device/errors.go",
	"services/active/errors.go",
	"api/common/errors.go",
	// Issue #575 B8 extracted the RFID check-in business logic into
	// services/iot/checkin.CheckinService. Its internal-error and
	// not-found strings (formerly in api/iot/checkin/workflow.go) now live
	// in this file's message constants; keep it in the guard corpus.
	"services/iot/checkin/checkin_errors.go",
}

// TestPyrePortalErrorStringsGuard asserts every PyrePortal-mapped substring
// is still present in the IoT-facing sources. The test name deliberately
// contains "PyrePortal" so failures in CI logs land the reader directly on
// this file — and the file-top comment explains the coupling.
func TestPyrePortalErrorStringsGuard(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed — cannot locate guard sources")
	iotDir := filepath.Dir(thisFile)                  // backend/api/iot
	backendRoot := filepath.Dir(filepath.Dir(iotDir)) // backend/

	var blob strings.Builder

	// Every non-test .go file under api/iot/ — package-layout agnostic, so
	// the issue #575 sub-package consolidation cannot silently blind the
	// guard the way the old hardcoded file list would have.
	walkErr := filepath.WalkDir(iotDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path) // #nosec G304 -- repo-local source scan
		if readErr != nil {
			return readErr
		}
		blob.Write(data)
		blob.WriteByte('\n')
		return nil
	})
	require.NoError(t, walkErr, "scanning api/iot sources")

	for _, rel := range extraGuardSources {
		data, err := os.ReadFile(filepath.Join(backendRoot, rel)) // #nosec G304 -- repo-local source scan
		require.NoErrorf(t, err, "reading guard source %s — if the file moved, update extraGuardSources in the same commit", rel)
		blob.Write(data)
		blob.WriteByte('\n')
	}
	corpus := blob.String()

	for _, want := range pyreportalErrorStrings {
		assert.Containsf(t, corpus, want,
			"PyrePortal error mapping depends on the substring %q being present in the "+
				"IoT-facing sources. It is missing — changing this string silently breaks "+
				"PyrePortal's German translation for this error. If the change is "+
				"intentional, update PyrePortal/src/services/apiErrors.ts "+
				"ERROR_MESSAGE_MAPPINGS and this test in the same PR.",
			want,
		)
	}
	for _, want := range pyreportalErrorCodes {
		assert.Containsf(t, corpus, want,
			"PyrePortal branches on the stable IoT error code %q. Update the backend and "+
				"PyrePortal mapping together if this contract changes.",
			want,
		)
	}
}
