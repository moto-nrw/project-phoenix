package education

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The lookup resolves exact (trimmed) forms first — the condition students
// and Klassenlehrer rows move under — and falls back to the normalized class
// identity entries are matched by everywhere else (#2399 review round 9).
func TestClassListEntryRenameLookup(t *testing.T) {
	t.Parallel()

	lookup := newClassListEntryRenameLookup(map[string]string{
		"1a": "2a",
		"4a": "", // graduates
	})

	target, hit := lookup.target(" 1a ")
	assert.True(t, hit)
	assert.Equal(t, "2a", target, "exact trimmed form resolves directly")

	target, hit = lookup.target("1A")
	assert.True(t, hit)
	assert.Equal(t, "2a", target, "case-divergent form resolves via the normalized fallback")

	target, hit = lookup.target("4A")
	assert.True(t, hit)
	assert.Empty(t, target, "graduation resolves via the fallback too")

	_, hit = lookup.target("1b")
	assert.False(t, hit, "an unmapped class stays out of the transition")
}

// Two mappings collapsing onto one normalized key with different targets are
// ambiguous: the exact forms still resolve, but no third display form may
// pick one of the two targets arbitrarily.
func TestClassListEntryRenameLookupAmbiguousNormalizedKey(t *testing.T) {
	t.Parallel()

	lookup := newClassListEntryRenameLookup(map[string]string{
		"1ab": "2a",
		"1AB": "2b",
	})

	target, hit := lookup.target("1ab")
	assert.True(t, hit)
	assert.Equal(t, "2a", target)

	target, hit = lookup.target("1AB")
	assert.True(t, hit)
	assert.Equal(t, "2b", target)

	_, hit = lookup.target("1Ab")
	assert.False(t, hit, "an ambiguous normalized key is excluded from the fallback")
}
