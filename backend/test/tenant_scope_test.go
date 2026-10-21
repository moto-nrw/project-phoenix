package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// iterationTB stands in for one run of a top-level test: the same name, a
// fresh testing.T instance per -count iteration, and cleanups run on demand.
type iterationTB struct {
	testing.TB
	name     string
	cleanups []func()
}

func (tb *iterationTB) Name() string     { return tb.name }
func (tb *iterationTB) Cleanup(f func()) { tb.cleanups = append(tb.cleanups, f) }

func (tb *iterationTB) finish() {
	for i := len(tb.cleanups) - 1; i >= 0; i-- {
		tb.cleanups[i]()
	}
	tb.cleanups = nil
}

// go test -count=N reruns a top-level test under the same name. Each run must
// own a fresh tenant: the rows of the previous run stay in the package clone,
// and without hand-written cleanup deletes a fixed-name fixture would collide
// with them.
func TestTenantIsFreshForEveryIterationOfATopLevelTest(t *testing.T) {
	t.Parallel()
	SetupTestDB(t)
	name := t.Name() + "Probe"

	first := &iterationTB{TB: t, name: name}
	firstTenant := Tenant(first)
	assert.Equal(t, firstTenant, Tenant(&iterationTB{TB: t, name: name + "/sub"}),
		"a subtest shares its parent's tenant")
	first.finish()
	assert.Equal(t, firstTenant, Tenant(first),
		"a cleanup of the finished run still sees the run's own tenant")

	second := &iterationTB{TB: t, name: name}
	secondTenant := Tenant(second)
	assert.NotEqual(t, firstTenant, secondTenant, "a rerun must not inherit the previous run's tenant")
	assert.Equal(t, secondTenant, Tenant(&iterationTB{TB: t, name: name + "/sub"}),
		"subtests of the rerun follow the rerun's tenant")
	second.finish()
}
