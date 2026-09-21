package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The register endpoint rejects usernames over 30 characters, so the account
// scope cannot be the slug with its OGS name of up to 30 characters.
func TestDemoAccountScopeFitsTheUsernameLimit(t *testing.T) {
	t.Parallel()
	slug := "offene-ganztagsschule-an-der-g-k3m9xp"

	assert.Equal(t, "k3m9xp", demoAccountScope(slug, 1))
	assert.Equal(t, "k3m9xp-2", demoAccountScope(slug, 2), "a repetition cannot reuse the accounts of the abandoned school")
	assert.LessOrEqual(t, len(fmt.Sprintf("demo20-%s", demoAccountScope(slug, 2))), 30)
}
