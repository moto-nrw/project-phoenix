package test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/pgdriver"
)

// PostgreSQL rejects a query message containing an embedded NUL. The injected DROP is a real
// failing SQL statement, but cannot name or remove any existing database.
func TestIsolatedCloneDropReportsSQLFailure(t *testing.T) {
	t.Parallel()
	db := SetupIsolatedTestDB(t)
	var name string
	require.NoError(t, db.NewRaw(`SELECT current_database()`).Scan(t.Context(), &name))
	entry := isolatedTestDatabase{clone: &testdb.CloneHandle{Name: name + "\x00"}}
	err := entry.drop()
	require.Error(t, err, "a failed DROP must not become a successful disposal")
	var sqlErr pgdriver.Error
	require.ErrorAs(t, err, &sqlErr)
	assert.Equal(t, "08P01", sqlErr.Field('C'))
	assert.Equal(t, err, entry.drop(), "repeated disposal retains the first failure")
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM pg_database WHERE datname = ?)`, name).Scan(t.Context(), &exists))
	assert.True(t, exists, "the valid clone was untouched by the fault")
}

// Exercise the actual t.Cleanup registration in a child test process: a
// disposal function returning an error is not enough if cleanup discards it.
func TestIsolatedCloneDropFailureFailsTestProcess(t *testing.T) {
	t.Parallel()
	const childFlag = "PHX_TEST_DROP_FAILURE_CHILD"
	if os.Getenv(childFlag) == "1" {
		// Reuse this already-required child and its clone for the setup-failure
		// regression: a bad template must not poison the following root setup.
		setupPackageTestDB(t)
		func() {
			good := packageCloneCfg
			defer func() { packageCloneCfg = good }()
			packageCloneCfg = good.WithTemplate("phx_missing_s3a_template")
			expected := &isolatedSetupFailureTest{T: t}
			require.PanicsWithValue(t, "expected isolated setup failure", func() { SetupIsolatedTestDB(expected) })
			require.Contains(t, expected.message, "phx_missing_s3a_template")
		}()
		require.False(t, t.Failed(), "only the later DROP failure must fail this child")
		_, cached := isolatedTestDatabases.Load(topLevelTestName(t))
		require.False(t, cached, "failed setup must remove its own cache entry")
		// A free slot also works without a package deadline (debugger/-timeout 0).
		// Reuse this clone so the regression adds no successful DB lifecycle.
		db := SetupIsolatedTestDB(isolatedNoDeadlineTest{t})
		fmt.Println("S3A_SETUP_RECOVERED")
		var name string
		require.NoError(t, db.NewRaw(`SELECT current_database()`).Scan(t.Context(), &name))
		fmt.Printf("Q7_DROP_FIXTURE=%s\n", name)
		value, ok := isolatedTestDatabases.Load(topLevelTestName(t))
		require.True(t, ok)
		value.(*isolatedTestDatabase).clone.Name = name + "\x00"
		return // the registered cleanup, not this body, must fail the process
	}
	SetupTestDB(t)
	output, exitCode, err := testdb.RunTestProcess(t.Context(), "TestIsolatedCloneDropFailureFailsTestProcess",
		childFlag+"=1", testdb.RunIDEnv+"="+testdb.SanitizeRunID(fmt.Sprintf("drop-%d", UniqueTestTenantID(t))), testdb.TemplateEnv+"="+packageCloneCfg.TemplateName())
	var clone string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Q7_DROP_FIXTURE=") {
			clone = strings.TrimPrefix(line, "Q7_DROP_FIXTURE=")
		}
	}
	// This exact clone belongs to the child we just started. Its deliberately
	// broken DROP cannot remove it; restore disposal here even when assertions fail.
	require.True(t, strings.HasPrefix(clone, testdb.ClonePrefix), "%s", output)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, testdb.DropClone(cleanupCtx, packageCloneCfg, clone))
	require.NoError(t, err, "%s", output)
	assert.Equal(t, 1, exitCode, "%s", output)
	assert.Contains(t, string(output), "S3A_SETUP_RECOVERED", "setup failure did not poison the following setup")
	assert.Contains(t, string(output), "drop isolated test database:")
	assert.Contains(t, string(output), "08P01", "the real PostgreSQL protocol fault reaches test cleanup")
}

// Intercept only the expected setup Fatalf. The real testing.T remains green
// until the existing DROP fault, preserving the process-exit regression.
type isolatedSetupFailureTest struct {
	*testing.T
	message string
}

func (t *isolatedSetupFailureTest) Fatalf(format string, args ...any) {
	t.message = fmt.Sprintf(format, args...)
	panic("expected isolated setup failure")
}
