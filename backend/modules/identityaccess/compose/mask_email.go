package compose

import "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"

// MaskEmail renders a recipient for a log line as `j***@example.com`. It is
// the module's masking rule, not a copy of it, so a log line and the MFA hint
// can never disagree about how much of an address shows (#2108).
func MaskEmail(address string) string {
	return domain.MaskEmail(address)
}
