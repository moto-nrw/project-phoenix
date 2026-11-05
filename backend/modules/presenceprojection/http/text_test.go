package presenceprojectionhttp

import "testing"

// The student list search finds "Özdemir" for "ozdem" (#4949).
func TestContainsFoldIgnoresAccents(t *testing.T) {
	t.Parallel()
	if !containsFold("Zoë Özdemir", "zoe ozdem") {
		t.Error(`containsFold("Zoë Özdemir", "zoe ozdem") = false`)
	}
	if containsFold("Kim Becker", "ozdem") {
		t.Error(`containsFold("Kim Becker", "ozdem") = true`)
	}
}
