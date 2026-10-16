package schoolstructure

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/stretchr/testify/assert"
)

// TestNormalizeClassFollowsTheClassNameRule pins the public class-name
// identity to School Structure's internal rule, so the two cannot drift.
func TestNormalizeClassFollowsTheClassNameRule(t *testing.T) {
	t.Parallel()

	for _, class := range []string{"", "1a", " 1a ", "1A", "\t3B\n", "Klasse 4c", "ÄB"} {
		assert.Equal(t, schoolclass.Normalize(class), NormalizeClass(class), "class %q", class)
	}
	assert.Equal(t, "1a", NormalizeClass(" 1A "))
}
