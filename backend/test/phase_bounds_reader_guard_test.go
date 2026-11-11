package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Phase bounds written into bookings come from Enrollment's one phase bounds
// reader (#6657): enrollment.PhaseBoundsReader takes the recurrence gate the
// phase edit holds and then reads the phase, and only it builds an
// enrollment.LockedPhase. TestPhaseBoundsWritersUseTheLockedReader keeps
// writers from reading the bounds around it:
//
//  1. No code outside the reader builds a LockedPhase.
//  2. An Enrollment application function that builds care booking inputs
//     (CareBookingInput) takes their window from a LockedPhase, not from
//     a phase's ServiceStartDate/ServiceEndDate.
//  3. The booking writers of the intake and the change requests take a
//     LockedPhase parameter, so a phase read elsewhere cannot reach them.
//  4. The care exit workflow reads phases only through the reader.
//
// Read-only paths (forms, status page, reports) keep reading the phase
// directly; they write no bound.

const phaseBoundsReaderFile = "modules/enrollment/phase_bounds_reader.go"

// phaseBoundsExemption is a booking writer that may take its window from a
// phase directly, with the reason no edit can run in between.
type phaseBoundsExemption struct {
	function string
	reason   string
}

var phaseBoundsExemptions = []phaseBoundsExemption{
	{
		function: "modules/enrollment/internal/application/rollover_requests.go:rolloverOfferingCopies",
		reason:   "the rollover creates the target phase in the same transaction; no other transaction sees it before the commit",
	},
}

// lockedPhaseWriters must take an enrollment LockedPhase parameter.
var lockedPhaseWriters = []string{
	"modules/enrollment/internal/application/intake_offerings.go:recordOfferingSubmission",
	"modules/enrollment/internal/application/change_request_apply.go:changeCareBookings",
}

func TestPhaseBoundsWritersUseTheLockedReader(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatalf("Could not find backend root: %v", err)
	}
	files, err := phaseBoundsGuardSources(backendRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range phaseBoundsViolations(files, phaseBoundsExemptions, lockedPhaseWriters) {
		t.Error(violation)
	}
}

// The guard recognises every kind of bypass it is meant to stop.
func TestPhaseBoundsGuardDetectsBypasses(t *testing.T) {
	t.Parallel()

	reader := `package enrollment
type LockedPhase struct{ phase *Phase }
func (r PhaseBoundsReader) LockedPhase(phase *Phase) (LockedPhase, error) { return LockedPhase{phase: phase}, nil }`
	clean := map[string]string{
		phaseBoundsReaderFile: reader,
		"modules/enrollment/internal/application/intake_offerings.go": `package application
func (s *Intake) failed() (enrollmentNative.LockedPhase, error) { return enrollmentNative.LockedPhase{}, errFailed }
func (s *Intake) recordOfferingSubmission(phase enrollmentNative.LockedPhase) []enrollmentNative.CareBookingInput {
	from, until, _ := phase.BookingWindow()
	return []enrollmentNative.CareBookingInput{{ValidFrom: &from, ValidUntil: &until}}
}`,
		"workflows/careexit/bookings.go": `package careexit
func (b Bookings) read(ctx context.Context) { _, _ = b.phases.LockedPhase(ctx, 1); _ = locked.Phase() }`,
	}
	writers := []string{"modules/enrollment/internal/application/intake_offerings.go:recordOfferingSubmission"}
	if got := phaseBoundsViolations(parsePhaseBoundsFixtures(t, clean), nil, writers); len(got) != 0 {
		t.Fatalf("clean sources report %v", got)
	}

	for name, bypass := range map[string]map[string]string{
		"a LockedPhase built outside the reader": {
			"modules/enrollment/phase_precheck.go": `package enrollment
func precheckedPhase(phase *Phase) LockedPhase { return LockedPhase{phase: phase} }`,
		},
		"a booking window from the pre-check phase": {
			"modules/enrollment/internal/application/change_request_apply.go": `package application
func bookings(phase *enrollmentNative.Phase) enrollmentNative.CareBookingInput {
	from := calendar.Date(phase.ServiceStartDate)
	return enrollmentNative.CareBookingInput{ValidFrom: &from}
}`,
		},
		"a booking writer without a LockedPhase": {
			"modules/enrollment/internal/application/intake_offerings.go": `package application
func (s *Intake) recordOfferingSubmission(from, through enrollmentNative.Date) {}`,
		},
		"an unlocked phase read in the care exit": {
			"workflows/careexit/bookings.go": `package careexit
func (b Bookings) read(ctx context.Context) { _, _ = b.phaseRows.Phase(ctx, 1) }`,
		},
	} {
		sources := map[string]string{phaseBoundsReaderFile: reader}
		for path, source := range bypass {
			sources[path] = source
		}
		if got := phaseBoundsViolations(parsePhaseBoundsFixtures(t, sources), nil, writers); len(got) == 0 {
			t.Errorf("%s is not reported", name)
		}
	}
}

func parsePhaseBoundsFixtures(t *testing.T, sources map[string]string) map[string]*ast.File {
	t.Helper()
	files := make(map[string]*ast.File, len(sources))
	for path, source := range sources {
		file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files[path] = file
	}
	return files
}

// phaseBoundsGuardSources parses the production sources the guard reads:
// all of modules/ and workflows/ for rule 1, the Enrollment application and
// the care exit workflow for the others.
func phaseBoundsGuardSources(backendRoot string) (map[string]*ast.File, error) {
	files := map[string]*ast.File{}
	for _, dir := range []string{"modules", "workflows", "roots"} {
		err := filepath.WalkDir(filepath.Join(backendRoot, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(backendRoot, path)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = file
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func phaseBoundsViolations(files map[string]*ast.File, exemptions []phaseBoundsExemption, writers []string) []string {
	exempt := map[string]bool{}
	for _, exemption := range exemptions {
		exempt[exemption.function] = true
	}
	declared, exemptUsed := map[string]bool{}, map[string]bool{}
	var violations []string
	for path, file := range files {
		if path != phaseBoundsReaderFile && buildsLockedPhase(file) {
			violations = append(violations, path+": builds an enrollment.LockedPhase outside the phase bounds reader")
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key := path + ":" + fn.Name.Name
			declared[key] = true
			switch {
			case strings.HasPrefix(path, "modules/enrollment/internal/application/"):
				if buildsCareBookingInput(fn) && readsServiceWindow(fn) {
					if !exempt[key] {
						violations = append(violations, key+": builds care bookings from a phase's service window instead of a LockedPhase")
					}
					exemptUsed[key] = true
				}
			case strings.HasPrefix(path, "workflows/careexit/"):
				if readsPhaseUnlocked(fn) {
					violations = append(violations, key+": reads a phase around the phase bounds reader")
				}
			}
		}
	}
	for _, exemption := range exemptions {
		if !exemptUsed[exemption.function] {
			violations = append(violations, exemption.function+": stale exemption; remove it from phaseBoundsExemptions")
		}
	}
	for _, writer := range writers {
		path, name, _ := strings.Cut(writer, ":")
		file, ok := files[path]
		if !ok || !declared[writer] {
			violations = append(violations, writer+": booking writer not found; update lockedPhaseWriters")
			continue
		}
		if !takesLockedPhase(file, name) {
			violations = append(violations, writer+": takes no enrollment.LockedPhase parameter")
		}
	}
	sort.Strings(violations)
	return violations
}

func typeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.StarExpr:
		return typeName(e.X)
	}
	return ""
}

func buildsLockedPhase(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		// The zero value carries no phase and its bounds fail; only a
		// literal with a phase is a second reader.
		if lit, ok := node.(*ast.CompositeLit); ok && lit.Type != nil && typeName(lit.Type) == "LockedPhase" && len(lit.Elts) > 0 {
			found = true
		}
		return !found
	})
	return found
}

func buildsCareBookingInput(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if lit, ok := node.(*ast.CompositeLit); ok && lit.Type != nil {
			if array, isArray := lit.Type.(*ast.ArrayType); isArray {
				found = found || typeName(array.Elt) == "CareBookingInput"
			}
			found = found || typeName(lit.Type) == "CareBookingInput"
		}
		return !found
	})
	return found
}

func readsServiceWindow(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if sel, ok := node.(*ast.SelectorExpr); ok && (sel.Sel.Name == "ServiceStartDate" || sel.Sel.Name == "ServiceEndDate") {
			found = true
		}
		return !found
	})
	return found
}

// readsPhaseUnlocked reports a call X.Phase(...) with arguments: a phase row
// read. LockedPhase.Phase() takes none.
func readsPhaseUnlocked(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && len(call.Args) > 0 {
			if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel && sel.Sel.Name == "Phase" {
				found = true
			}
		}
		return !found
	})
	return found
}

func takesLockedPhase(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		for _, field := range fn.Type.Params.List {
			if typeName(field.Type) == "LockedPhase" {
				return true
			}
		}
	}
	return false
}
