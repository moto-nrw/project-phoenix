// Package secrets generates the random values operator provisioning hands
// out: device API keys and the suffix that keeps generated usernames unique.
package secrets

import "github.com/moto-nrw/project-phoenix/internal/randstr"

const usernameSuffixLength = 6

// Generator implements the provisioning secrets port over the shared
// cryptographically secure generator.
type Generator struct{}

// APIKey returns a fresh device API key.
func (Generator) APIKey() (string, error) { return randstr.APIKey() }

// UsernameSuffix returns six random lowercase letters and digits. A failing
// random source yields an empty suffix; the username stays unique through
// the account's other name parts and the registration's conflict check.
func (Generator) UsernameSuffix() string {
	suffix, _ := randstr.String(usernameSuffixLength, randstr.LowerAlphanumeric)
	return suffix
}
