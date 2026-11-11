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
	cgo := false
	patterns := make([]string, 0, len(flowPackages))
	for _, path := range flowPackages {
		patterns = append(patterns, "./"+path)
	}
	loaded, err := loadTypedPatterns(project, Build{GOOS: "linux", GOARCH: "amd64", CGOEnabled: &cgo}, overlay, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	return parentChildWriteGuardViolations(project, parentChildWriteModule, loaded)
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

// A poll answer written without the guard is reported with the answer write
// (#6638).
func TestParentChildWriteGuardRuleReportsAnUnguardedPollAnswer(t *testing.T) {
	t.Parallel()
	project := parentFlowsProject(t)
	file := filepath.Join(project, flowPackages[1], "parent_announcement_service.go")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	guarded := "err = s.Children.WithChildWrite(ctx, accountID, studentID, child, access, func(txCtx context.Context) error {"
	if !strings.Contains(string(source), guarded) {
		t.Fatal("the poll answer no longer opens its unit as this test expects; update the bypass")
	}
	bypass := strings.Replace(string(source), guarded, "_ = access\n\terr = s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {", 1)
	violations := checkParentFlows(t, project, map[string][]byte{file: []byte(bypass)})
	if len(violations) != 1 || violations[0].Rule != "parent-child-writes.unguarded" || violations[0].Source != "messaging.Service.RespondToAnnouncement" ||
		!strings.Contains(violations[0].Target, "messaging.AnnouncementFeedAccess.SetResponse") {
		t.Fatalf("want the unguarded poll answer, got %+v", violations)
	}
}

func checkRenewal(t *testing.T, overlay map[string][]byte) []Violation {
	t.Helper()
	cgo := false
	project := parentFlowsProject(t)
	loaded, err := loadTypedPatterns(project, Build{GOOS: "linux", GOARCH: "amd64", CGOEnabled: &cgo}, overlay, "./"+renewalPackage)
	if err != nil {
		t.Fatal(err)
	}
	return parentRenewalGuardViolations(project, parentChildWriteModule, loaded)
}

func renewalSource(t *testing.T, name string) (string, string) {
	t.Helper()
	file := filepath.Join(parentFlowsProject(t), renewalPackage, name)
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return file, string(source)
}

func TestParentRenewalsRunBehindTheGuard(t *testing.T) {
	t.Parallel()
	if violations := checkRenewal(t, nil); len(violations) > 0 {
		t.Fatalf("parent renewals outside the guard:\n%v", FormatViolations(violations))
	}
}

// A submission that asks the renewal permission without locks is reported.
func TestParentRenewalGuardRuleReportsTheUnlockedCheck(t *testing.T) {
	t.Parallel()
	file, source := renewalSource(t, "intake_submit.go")
	guarded := "if err := s.holdGuardianReEnrollment(ctx, submitter, matchedStudentID, req.TenantID, i); err != nil {"
	if !strings.Contains(source, guarded) {
		t.Fatal("the submission no longer holds the renewal as this test expects; update the bypass")
	}
	bypass := strings.Replace(source, guarded, "if err := s.assertGuardianMayReEnrollStudent(ctx, submitter, matchedStudentID, req.TenantID, i); err != nil {", 1)
	violations := checkRenewal(t, map[string][]byte{file: []byte(bypass)})
	rules := map[string]string{}
	for _, v := range violations {
		if v.Source == "application.Intake.Submit" {
			rules[v.Rule] = v.Target
		}
	}
	if !strings.Contains(rules["parent-renewal.unlocked-check"], "application.Intake.assertGuardianMayReEnrollStudent") {
		t.Fatalf("want the unlocked renewal check of the submission, got %+v", violations)
	}
}

// Removing the hold from the shared renewal check reports every renewing
// entry as unguarded.
func TestParentRenewalGuardRuleReportsARemovedHold(t *testing.T) {
	t.Parallel()
	file, source := renewalSource(t, "intake_eligibility.go")
	guarded := "granted, err := submitterHoldsRenewal(ctx, s.deps.GuardianHold, submitter, *matchedStudentID, tenantID)"
	if !strings.Contains(source, guarded) {
		t.Fatal("the renewal no longer asks the hold as this test expects; update the bypass")
	}
	bypass := strings.Replace(source, guarded, "granted, err := true, error(nil)", 1)
	violations := checkRenewal(t, map[string][]byte{file: []byte(bypass)})
	unguarded := map[string]bool{}
	for _, v := range violations {
		if v.Rule == "parent-renewal.unguarded" {
			unguarded[v.Source] = true
		}
	}
	if !unguarded["application.Intake.Submit"] || !unguarded["application.Intake.ReplaceEditable"] {
		t.Fatalf("want submission and edit reported as unguarded, got %+v", violations)
	}
}
