// Package iot — PyrePortal error-string contract guard.
//
// This is a regression tripwire, not a behavioral test. PyrePortal maps
// backend error substrings to German kiosk messages in
// PyrePortal/src/services/apiErrors.ts (ERROR_MESSAGE_MAPPINGS). If Phoenix
// changes an emitted substring without updating PyrePortal, the kiosk silently
// falls through to a generic or raw error message.
//
// Each surface below lists only production files that can emit its mapped
// strings. Keeping the source lists explicit prevents test fixtures or an
// unrelated package from satisfying the contract by accident. Generic client
// fallbacks such as HTTP status names are deliberately excluded: they are not
// exact Phoenix error-string contracts.
package iot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pyrePortalErrorSurface struct {
	name    string
	strings []string
	sources []string
}

var pyrePortalErrorSurfaces = []pyrePortalErrorSurface{
	{
		name: "device authentication",
		strings: []string{
			"invalid device API key",
			"device API key is required",
			"invalid API key format",
			"invalid staff PIN",
			"staff PIN is required",
			"device is not active",
		},
		sources: []string{"../../auth/device/errors.go"},
	},
	{
		name: "checkin",
		strings: []string{
			"ROOM_CAPACITY_EXCEEDED",
			"room capacity exceeded",
			"Room capacity exceeded",
			"ACTIVITY_CAPACITY_EXCEEDED",
			"activity capacity exceeded",
			"Activity capacity exceeded",
			"STUDENT_ALREADY_ACTIVE",
			"student already has an active visit",
			"RFID tag not found",
			"RFID tag not assigned",
			"student RFID tag required for pickup query",
			"no active session",
			"failed to end visit record",
			"failed to create visit record",
			"failed to get room information",
			"failed to check room capacity",
			"failed to get activity information",
			"failed to check activity capacity",
			"error finding active groups in room",
			"room_id is required for check-in",
			"schulhof activity not configured",
			"WC activity not configured",
			"WC activity auto-create requires staff context",
			"no active groups in specified room",
			"failed to create Schulhof session",
			"failed to create WC session",
			"failed to create session",
		},
		sources: []string{
			"common/errors.go",
			"checkin/workflow.go",
			"checkin/handlers.go",
			"checkin/helpers.go",
			"checkin/schulhof.go",
			"checkin/wc.go",
			"checkin/types.go",
			"checkin/resource.go",
		},
	},
	{
		name: "sessions",
		strings: []string{
			"device is already running an activity session",
			"no active session to end",
			"no active session",
			"invalid session ID",
			"activity_id is required",
			"at least one supervisor",
		},
		sources: []string{
			"sessions/handlers.go",
			"sessions/helpers.go",
			"sessions/types.go",
			"../../services/active/errors.go",
			"../../services/active/session_service.go",
		},
	},
	{
		name: "attendance",
		strings: []string{
			"RFID parameter is required",
			"RFID tag not found",
			"RFID tag not assigned",
			"student has no attendance record for today",
			"person is not a student",
			"destination must be 'zuhause' or 'unterwegs'",
			"destination is required for confirm_daily_checkout",
		},
		sources: []string{
			"attendance/handlers.go",
			"attendance/types.go",
			"common/errors.go",
		},
	},
	{
		name: "RFID lookup and staff assignment",
		strings: []string{
			"tagId parameter is required",
			"invalid staff ID",
			"staff not found",
			"staff has no RFID tag assigned",
			"failed to get person data for staff",
		},
		sources: []string{
			"data/handlers.go",
			"rfid/handlers.go",
			"../common/errors.go",
		},
	},
	{
		name: "feedback",
		strings: []string{
			"student_id is required",
			"value is required",
			"student not found",
		},
		sources: []string{
			"feedback/handlers.go",
			"feedback/types.go",
		},
	},
}

// TestPyrePortalIoTErrorStringsGuard asserts that every backend-specific
// substring mapped by PyrePortal remains in the production source for the IoT
// surface that emits it.
func TestPyrePortalIoTErrorStringsGuard(t *testing.T) {
	for _, surface := range pyrePortalErrorSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			corpus := readGuardSources(t, surface.sources)
			for _, want := range surface.strings {
				assert.Containsf(t, corpus, want,
					"PyrePortal ERROR_MESSAGE_MAPPINGS depends on substring %q from the %s IoT surface. "+
						"Changing it silently breaks the kiosk's German error translation. If the change is "+
						"intentional, update PyrePortal/src/services/apiErrors.ts and this guard together.",
					want,
					surface.name,
				)
			}
		})
	}
}

func readGuardSources(t *testing.T, sources []string) string {
	t.Helper()

	var blob strings.Builder
	for _, source := range sources {
		path, err := filepath.Abs(source)
		if err != nil {
			t.Fatalf("resolving guard source %s: %v", source, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading guard source %s: %v", path, err)
		}
		blob.Write(data)
		blob.WriteByte('\n')
	}
	return blob.String()
}
