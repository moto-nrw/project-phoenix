package repositories_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

// TestNewFactory pins the factory's construction contract: it binds the
// owner capabilities it reads through — People Directory, School Structure,
// School Membership, Facilities, School Calendar, Workforce and Timetable — at
// construction (#2743) and refuses to build without them. The composed graph
// itself is exercised by services' TestNewRepositoryFactory, the composition
// that owns the People Directory.
func TestNewFactory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	assert.PanicsWithValue(t, "repository factory: timetable and projection dependencies are required", func() {
		repositories.NewFactory(db, repositories.TimetableDependencies{})
	})
}
