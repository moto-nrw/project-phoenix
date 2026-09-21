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

// The simulation serves only demo schools in use (#3464): a school that falls
// out of use loses its ticker, and a returning visitor gets a new one.
func TestDemoTickersFollowTheSchoolsInUse(t *testing.T) {
	t.Parallel()
	tickers := newDemoTickers()
	stopped := map[string]bool{}
	start := func(slugs []string) {
		for _, slug := range tickers.keepOnly(slugs) {
			tickers.add(slug, func() { stopped[slug] = true })
		}
	}

	assert.Equal(t, []string{"a", "b"}, tickers.keepOnly([]string{"a", "b"}), "every school in use needs a ticker")
	start([]string{"a", "b"})
	assert.Empty(t, tickers.keepOnly([]string{"a", "b"}), "a running ticker is not started twice")

	assert.Empty(t, tickers.keepOnly([]string{"b"}))
	assert.True(t, stopped["a"], "a school out of use gets no more ticks")
	assert.False(t, stopped["b"])

	assert.Equal(t, []string{"a"}, tickers.keepOnly([]string{"a", "b"}), "a returning visitor brings the ticker back")
	assert.Empty(t, tickers.keepOnly(nil))
	assert.True(t, stopped["b"])
}

// A stopped ticker may end after its successor started; ending must not
// forget the successor.
func TestDemoTickerEndingKeepsItsSuccessor(t *testing.T) {
	t.Parallel()
	tickers := newDemoTickers()
	first := tickers.add("a", func() {})
	tickers.keepOnly(nil)
	second := tickers.add("a", func() {})

	tickers.remove("a", first)
	assert.Empty(t, tickers.keepOnly([]string{"a"}), "the successor still runs")
	tickers.remove("a", second)
	assert.Equal(t, []string{"a"}, tickers.keepOnly([]string{"a"}))
}
