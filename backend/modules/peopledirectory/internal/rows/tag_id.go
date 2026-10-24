package users

import "strings"

// NormalizeTagID formats a person's card reference: trimmed, separators
// removed, upper case. Card validity remains the Identity Access owner's rule.
func NormalizeTagID(tagID string) string {
	tagID = strings.TrimSpace(tagID)
	tagID = strings.NewReplacer(":", "", "-", "", " ", "").Replace(tagID)
	return strings.ToUpper(tagID)
}
