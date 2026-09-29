package users

import (
	"strings"
)

// The masking and diff helpers of the guardian payment audit (#2608). They
// began as the staff Stammdaten helpers and stayed here when the staff
// personnel record moved to Workforce (#3752): the guardian bank data is masked
// the same way, but its audit trail is a separate one.

// stammdatenChange is one field diff destined for the audit trail.
type stammdatenChange struct {
	field    string
	oldValue string
	newValue string
}

func normalizeCompact(v *string) *string {
	if v == nil {
		return nil
	}
	compact := strings.ReplaceAll(strings.TrimSpace(*v), " ", "")
	if compact == "" {
		return nil
	}
	return &compact
}

func strPtrAuditValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func maskedAuditValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// maskedChange builds a financial audit diff from masked values. When the
// plaintext changed but both masks render identically (same last-4, or the
// fixed full mask), the new value is suffixed so the audit row still shows a
// change and passes the old!=new validation.
func maskedChange(field string, oldMasked, newMasked *string) stammdatenChange {
	oldValue := maskedAuditValue(oldMasked)
	newValue := maskedAuditValue(newMasked)
	if oldValue == newValue {
		newValue += " (geändert)"
	}
	return stammdatenChange{field: field, oldValue: oldValue, newValue: newValue}
}

// maskTailPtr masks all but the last visible characters: "•••• 1234".
func maskTailPtr(v *string, visible int) *string {
	if v == nil || *v == "" {
		return nil
	}
	value := *v
	if len(value) <= visible {
		masked := strings.Repeat("•", len(value))
		return &masked
	}
	masked := "•••• " + value[len(value)-visible:]
	return &masked
}

// maskAllPtr returns a fixed-length mask so not even the value length leaks.
func maskAllPtr(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	masked := "••••••••"
	return &masked
}

func normalizeOptional(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// ibanChecksumValid runs the ISO 13616 mod-97 check.
func ibanChecksumValid(iban string) bool {
	rearranged := iban[4:] + iban[:4]
	remainder := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		case r >= 'A' && r <= 'Z':
			remainder = (remainder*100 + int(r-'A') + 10) % 97
		default:
			return false
		}
	}
	return remainder == 1
}
