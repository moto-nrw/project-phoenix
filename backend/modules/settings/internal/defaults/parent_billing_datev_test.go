package defaults

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The DATEV settings (#4694) wait for the customer's values: no number or
// account is preset, and the patterns follow the DATEV header.
func TestParentBillingDatevSettings(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		config.KeyParentBillingDatevConsultantNumber, config.KeyParentBillingDatevClientNumber,
		config.KeyParentBillingDatevAccountLength, config.KeyParentBillingDatevChartOfAccounts,
		config.KeyParentBillingDatevRevenueAccount,
	} {
		definition := config.GetDefinition(key)
		require.NotNil(t, definition, key)
		assert.Equal(t, "", definition.Default, "%s has no preset", key)
		assert.Equal(t, config.LevelTraeger, definition.Levels, key)
	}
	assert.Equal(t, "1", config.GetDefinition(config.KeyParentBillingDatevFiscalYearStart).Default)
	assert.Equal(t, false, config.GetDefinition(config.KeyParentBillingDatevLocked).Default)

	patterns := map[string]struct{ valid, invalid []string }{
		config.KeyParentBillingDatevConsultantNumber: {
			valid:   []string{"1001", "1009", "1010", "1099", "1100", "1999", "2000", "9999", "10000", "29098", "100000", "1000000", "9999999"},
			invalid: []string{"999", "1000", "0123", "12345678", "12 34"},
		},
		config.KeyParentBillingDatevClientNumber:   {valid: []string{"1", "55003", "99999"}, invalid: []string{"0", "100000", "1a"}},
		config.KeyParentBillingDatevRevenueAccount: {valid: []string{"85", "4400", "44000000"}, invalid: []string{"440000000", "44-00"}},
	}
	for key, values := range patterns {
		pattern := regexp.MustCompile(*config.GetDefinition(key).Validation.Pattern)
		for _, value := range values.valid {
			assert.True(t, pattern.MatchString(value), "%s admits %q", key, value)
		}
		for _, value := range values.invalid {
			assert.False(t, pattern.MatchString(value), "%s refuses %q", key, value)
		}
	}
}
