// IoT error-string golden — issue #3419.
//
// PyrePortal (ERROR_MESSAGE_MAPPINGS and STAFF_CLOCK_MESSAGES in
// PyrePortal/src/services/apiErrors.ts, snapshot in
// src/services/apiErrors.test.ts) substring-matches the error text of
// /api/iot/* responses and renders a German message. A backend rename makes
// the match fail silently, so testdata/iot_error_strings.golden pins every
// mapped literal to the Go declaration that emits it.
//
// Each line is derived from the source: the test parses the named file,
// finds the first string literal (never a comment or a log message) that
// contains the PyrePortal pattern, and records the constant, variable or
// function that declares it. Renaming the literal turns its line into
// "NOT EMITTED"; moving it to another declaration changes the symbol. Both
// are golden diffs. api/iot.TestPyrePortalErrorStringsGuard reconciles its
// own lists against the same golden.
package api

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// iotErrorContract is one PyrePortal pattern and the backend file (relative to
// the backend module root) whose declaration emits it on an /api/iot route.
type iotErrorContract struct {
	pattern string
	file    string
}

const (
	deviceAuthErrors   = "auth/device/errors.go"
	checkinCapacity    = "api/iot/checkin/capacity_errors.go"
	checkinAttendance  = "api/iot/checkin/attendance_types.go"
	iotSessionHandlers = "api/iot/sessions/handlers.go"
	iotSessionTypes    = "api/iot/sessions/types.go"
	iotDataHandlers    = "api/iot/data/handlers.go"
	iotRFIDTypes       = "api/iot/data/rfid_types.go"
	iotFeedbackTypes   = "api/iot/data/feedback_types.go"
	iotFeedbackHandler = "api/iot/data/feedback_handlers.go"
	staffClockErrors   = "api/iot/staffclock/errors.go"
	presenceOperations = "modules/studentpresence/operations.go"
	deviceScanErrors   = "modules/devicescan/devicescan.go"
	deviceScanMessages = "modules/devicescan/scan.go"
	deviceScanSessions = "modules/devicescan/internal/application/session_lifecycle.go"
	deviceScanTags     = "modules/devicescan/internal/application/tag_commands.go"
)

// iotErrorMessages are the business patterns of ERROR_MESSAGE_MAPPINGS
// (apiErrors.ts). The generic HTTP fallbacks at its end are no backend
// contract and stay out.
var iotErrorMessages = []iotErrorContract{
	{"ACTIVITY_CAPACITY_EXCEEDED", checkinCapacity},
	{"activity capacity exceeded", checkinCapacity},
	{"Activity capacity exceeded", checkinCapacity},
	{"ROOM_CAPACITY_EXCEEDED", checkinCapacity},
	{"room capacity exceeded", checkinCapacity},
	{"Room capacity exceeded", checkinCapacity},
	{"STUDENT_ALREADY_ACTIVE", checkinCapacity},
	{"student already has an active visit", presenceOperations},

	{"invalid device API key", deviceAuthErrors},
	{"device API key is required", deviceAuthErrors},
	{"invalid API key format", deviceAuthErrors},
	{"invalid staff PIN", deviceAuthErrors},
	{"staff PIN is required", deviceAuthErrors},
	{"device is not active", deviceAuthErrors},

	{"device is already running an activity session", presenceOperations},
	{"no active session to end", deviceScanSessions},
	{"no active session", deviceScanMessages},
	{"no room available", presenceOperations},
	{"invalid session ID", iotSessionHandlers},
	{"activity_id is required", iotSessionTypes},
	{"at least one supervisor", iotSessionTypes},

	{"RFID tag not found", deviceScanMessages},
	{"RFID tag not assigned", deviceScanMessages},
	{"student RFID tag required for pickup query", deviceScanMessages},
	{"RFID parameter is required", deviceScanMessages},
	{"invalid RFID tag", deviceScanErrors},
	{"RFID tag is inactive", deviceScanErrors},
	{"RFID tag is not assigned to staff", deviceScanErrors},

	{"student has no attendance record for today", presenceOperations},
	{"person is not a student", deviceScanMessages},
	{"no active groups in specified room", deviceScanMessages},

	{"invalid staff ID", iotRFIDTypes},
	{"staff not found", deviceScanTags},
	{"staff has no RFID tag assigned", deviceScanTags},

	{"student_id is required", iotFeedbackTypes},
	{"value is required", iotFeedbackTypes},
	{"student not found", iotFeedbackHandler},

	{"room_id is required for check-in", deviceScanMessages},
	{"tagId parameter is required", iotDataHandlers},
	{"destination must be 'zuhause' or 'unterwegs'", checkinAttendance},
	{"destination is required for confirm_daily_checkout", checkinAttendance},

	{"schulhof activity not configured", deviceScanMessages},
	{"WC activity not configured", deviceScanMessages},
	{"WC activity auto-create requires staff context", deviceScanMessages},
	{"failed to create Schulhof session", deviceScanMessages},
	{"failed to create WC session", deviceScanMessages},
	{"failed to create session", deviceScanMessages},
	{"failed to create visit record", deviceScanMessages},
	{"failed to end visit record", deviceScanMessages},
	{"failed to get room information", deviceScanMessages},
	{"failed to check room capacity", deviceScanMessages},
	{"failed to get activity information", deviceScanMessages},
	{"failed to check activity capacity", deviceScanMessages},
	{"error finding active groups in room", deviceScanMessages},
	{"failed to get person data for staff", deviceScanTags},
}

// iotErrorCodes are the stable codes of STAFF_CLOCK_MESSAGES (apiErrors.ts).
var iotErrorCodes = []iotErrorContract{
	{"invalid_staff_clock_request", staffClockErrors},
	{"invalid_rfid_tag", staffClockErrors},
	{"rfid_tag_not_found", staffClockErrors},
	{"rfid_tag_inactive", staffClockErrors},
	{"rfid_tag_not_staff", staffClockErrors},
	{"planned_start_not_reached", staffClockErrors},
	{"deviation_reason_required", staffClockErrors},
	{"invalid_staff_clock_state", staffClockErrors},
}

// iotErrorExclusions are PyrePortal mappings the backend deliberately does not
// emit. Each is a decision on the record: the PyrePortal side is dead code and
// a cleanup candidate there, not a backend contract.
var iotErrorExclusions = []struct{ kind, pattern, reason string }{
	{"message", "locked", "no emitter; PyrePortal's generic lock fallback"},
	{"message", "staff account is locked due to failed PIN attempts", "never emitted (audit A5)"},
	{"message", "maximum PIN attempts exceeded", "never emitted (audit A5)"},
	{"message", "device is offline", "never emitted (audit A5)"},
	{"message", "staff RFID authentication must be done via session management", "verified absent 2026-07-12"},
	{"message", "no active visit found for student", "verified absent 2026-07-12"},
	{"code", "reopen_status_conflict", "retired with #2402; a repeated check-in starts a new work block"},
}

const iotErrorStringsHeader = `# PyrePortal error-string contract for /api/iot/* (issue #3419).
# Consumer: ../PyrePortal/src/services/apiErrors.ts (ERROR_MESSAGE_MAPPINGS,
# STAFF_CLOCK_MESSAGES), snapshot-guarded by src/services/apiErrors.test.ts.
# Changing a line here is a coordinated change in the backend and PyrePortal repos.
# Generated by TestFullProductionRouterGolden/contracts/IoT_error_strings;
# reconciled by api/iot.TestPyrePortalErrorStringsGuard.
`

func checkIoTErrorStringsGolden(t *testing.T) {
	t.Parallel()

	files := map[string]*declaredLiterals{}
	resolve := func(c iotErrorContract) string {
		decls, ok := files[c.file]
		if !ok {
			decls = parseDeclaredLiterals(t, c.file)
			files[c.file] = decls
		}
		if symbol := decls.find(c.pattern); symbol != "" {
			return fmt.Sprintf("%s.%s (%s)", path.Dir(c.file), symbol, path.Base(c.file))
		}
		return fmt.Sprintf("NOT EMITTED by %s", c.file)
	}

	var lines []string
	for _, c := range iotErrorMessages {
		lines = append(lines, fmt.Sprintf("message: %s -> %s", c.pattern, resolve(c)))
	}
	for _, c := range iotErrorCodes {
		lines = append(lines, fmt.Sprintf("code: %s -> %s", c.pattern, resolve(c)))
	}
	for _, e := range iotErrorExclusions {
		lines = append(lines, fmt.Sprintf("excluded %s: %s -> %s", e.kind, e.pattern, e.reason))
	}
	sort.Strings(lines)

	compareGolden(t, filepath.Join("testdata", "iot_error_strings.golden"), iotErrorStringsHeader+strings.Join(lines, "\n")+"\n",
		"a PyrePortal-mapped /api/iot error string moved or changed. The kiosk substring-matches it in "+
			"PyrePortal/src/services/apiErrors.ts and falls back to raw English on a mismatch. Revert the "+
			"string, or change apiErrors.ts in PyrePortal in step and regenerate with -update-goldens")
}

// declaredLiterals holds the string literals of one Go file with the name of
// the constant, variable or function that declares each.
type declaredLiterals struct {
	literals []string
	symbols  []string
}

func (d *declaredLiterals) find(pattern string) string {
	for i, literal := range d.literals {
		if strings.Contains(literal, pattern) {
			return d.symbols[i]
		}
	}
	return ""
}

func parseDeclaredLiterals(t *testing.T, rel string) *declaredLiterals {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
	require.NoErrorf(t, err, "parse %s — if the file moved, update the IoT error contract table in the same commit", rel)

	decls := &declaredLiterals{}
	collect := func(symbol string, node ast.Node) {
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && isLogCall(call) {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			require.NoError(t, err)
			decls.literals = append(decls.literals, value)
			decls.symbols = append(decls.symbols, symbol)
			return true
		})
	}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			collect(funcSymbol(decl), decl)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, expr := range value.Values {
					collect(value.Names[min(i, len(value.Names)-1)].Name, expr)
				}
			}
		}
	}
	return decls
}

// isLogCall reports slog-style calls whose message never reaches the client.
func isLogCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch selector.Sel.Name {
	case "DebugContext", "InfoContext", "WarnContext", "ErrorContext":
		return true
	}
	return false
}

func funcSymbol(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	recv := decl.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if index, ok := recv.(*ast.IndexExpr); ok {
		recv = index.X
	}
	if ident, ok := recv.(*ast.Ident); ok {
		return ident.Name + "." + decl.Name.Name
	}
	return decl.Name.Name
}
