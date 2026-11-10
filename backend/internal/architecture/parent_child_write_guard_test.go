package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const parentChildWriteModule = "github.com/moto-nrw/project-phoenix"

func parentFlowsProject(t *testing.T) string {
	t.Helper()
	project, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func checkParentFlows(t *testing.T, project string, overlay map[string][]byte) []Violation {
	t.Helper()
	violations, err := checkParentChildWriteFlows(project, parentChildWriteModule, overlay)
	if err != nil {
		t.Fatal(err)
	}
	return violations
}

func TestParentChildWritesRunBehindTheGuard(t *testing.T) {
	t.Parallel()
	if violations := checkParentFlows(t, parentFlowsProject(t), nil); len(violations) > 0 {
		t.Fatalf("parent child writes outside the guard:\n%v", FormatViolations(violations))
	}
}

// A sick note that opens its unit without the guard is reported, with every
// owner write it reaches.
func TestParentChildWriteGuardRuleReportsABypass(t *testing.T) {
	t.Parallel()
	project := parentFlowsProject(t)
	file := filepath.Join(project, flowPackages[0], "absences.go")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	guarded := "s.WithChildWrite(ctx, report.accountID, report.studentID, child, sickNoteWriteAccess, func(txCtx context.Context) error {"
	if !strings.Contains(string(source), guarded) {
		t.Fatal("the sick note no longer opens its unit as this test expects; update the bypass")
	}
	bypass := strings.Replace(string(source), guarded, "s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {", 1)
	violations := checkParentFlows(t, project, map[string][]byte{file: []byte(bypass)})
	if len(violations) != 1 || violations[0].Rule != "parent-child-writes.unguarded" || violations[0].Source != "care.Service.SubmitSickNote" ||
		!strings.Contains(violations[0].Target, "care.GuardianAbsenceReports.ReportGuardianAbsence") {
		t.Fatalf("want the unguarded sick note, got %+v", violations)
	}
}

// A port the register does not classify is reported before it can carry an
// unguarded write.
func TestParentChildWriteGuardRuleReportsAnUnclassifiedPort(t *testing.T) {
	t.Parallel()
	project := parentFlowsProject(t)
	file := filepath.Join(project, flowPackages[0], "zz_new_port.go")
	violations := checkParentFlows(t, project, map[string][]byte{file: []byte(`package care

import "context"

type NewPort interface{ Store(context.Context) error }

func (s *Service) StoreSomething(ctx context.Context, port NewPort) error { return port.Store(ctx) }
`)})
	if len(violations) != 1 || violations[0].Rule != "parent-child-writes.unclassified-port" || violations[0].Target != "care.NewPort.Store" {
		t.Fatalf("want the unclassified port, got %+v", violations)
	}
}
