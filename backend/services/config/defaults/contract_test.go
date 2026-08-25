package defaults_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contractKeys is every vertrag.* setting, in registry order.
var contractKeys = []string{
	config.KeyContractTier,
	config.KeyContractBookedChildren,
	config.KeyContractPricePerChildCents,
	config.KeyContractBillingCycle,
	config.KeyContractTermStart,
	config.KeyContractTermEnd,
	config.KeyContractInvoiceRecipient,
	config.KeyContractCustomerNumber,
	config.KeyContractSupportEmail,
	config.KeyContractNote,
}

// TestContractSettings_AllRegisteredOnVertragTab pins the shape of the demo
// contract surface (#1459): every key exists, sits on the "vertrag" tab, and
// carries German UI text.
func TestContractSettings_AllRegisteredOnVertragTab(t *testing.T) {
	t.Parallel()

	for _, key := range contractKeys {
		def := config.GetDefinition(key)
		require.NotNilf(t, def, "setting %q should be registered", key)
		assert.Equal(t, "vertrag", def.Tab, key)
		assert.NotEmpty(t, def.Label, key)
		assert.NotEmpty(t, def.Description, key)
		assert.NotEmpty(t, def.Category, key)
	}
}

// TestContractSettings_AreOperatorOnly is the security assertion of this
// feature: a school admin must not be able to raise their own tier or child
// contingent. AccessOperatorOnly hides the keys from the tenant schema and
// makes api/config reject direct writes.
func TestContractSettings_AreOperatorOnly(t *testing.T) {
	t.Parallel()

	for _, key := range contractKeys {
		def := config.GetDefinition(key)
		require.NotNil(t, def, key)
		assert.Equalf(t, config.AccessOperatorOnly, def.AccessPolicy,
			"setting %q must stay operator-only — a school could otherwise change its own contract", key)
	}
}

// TestContractSettings_Permissions keeps the write tier at config:manage,
// matching the other commercially sensitive settings.
func TestContractSettings_Permissions(t *testing.T) {
	t.Parallel()

	for _, key := range contractKeys {
		def := config.GetDefinition(key)
		require.NotNil(t, def, key)
		assert.Equal(t, "config:read", def.ReadPermission, key)
		assert.Equal(t, "config:manage", def.WritePermission, key)
	}
}

// TestContractSettings_Types pins each field type, because the frontend
// renders purely from the schema.
func TestContractSettings_Types(t *testing.T) {
	t.Parallel()

	expected := map[string]config.FieldType{
		config.KeyContractTier:               config.FieldSelect,
		config.KeyContractBookedChildren:     config.FieldNumber,
		config.KeyContractPricePerChildCents: config.FieldNumber,
		config.KeyContractBillingCycle:       config.FieldSelect,
		config.KeyContractTermStart:          config.FieldDate,
		config.KeyContractTermEnd:            config.FieldDate,
		config.KeyContractInvoiceRecipient:   config.FieldText,
		config.KeyContractCustomerNumber:     config.FieldText,
		config.KeyContractSupportEmail:       config.FieldText,
		config.KeyContractNote:               config.FieldTextarea,
	}

	for key, wantType := range expected {
		def := config.GetDefinition(key)
		require.NotNil(t, def, key)
		assert.Equal(t, wantType, def.Type, key)
	}
}

// TestContractSettings_DefaultsAreEmpty encodes the deliberate decision that
// an unconfigured school shows "noch nicht hinterlegt" rather than an invented
// tier or price — the same reasoning as the DATEV Lohnarten.
func TestContractSettings_DefaultsAreEmpty(t *testing.T) {
	t.Parallel()

	emptyStringKeys := []string{
		config.KeyContractTier,
		config.KeyContractBillingCycle,
		config.KeyContractTermStart,
		config.KeyContractTermEnd,
		config.KeyContractInvoiceRecipient,
		config.KeyContractCustomerNumber,
		config.KeyContractSupportEmail,
		config.KeyContractNote,
	}
	for _, key := range emptyStringKeys {
		def := config.GetDefinition(key)
		require.NotNil(t, def, key)
		assert.Equalf(t, "", def.Default, "setting %q must default to empty", key)
	}

	for _, key := range []string{config.KeyContractBookedChildren, config.KeyContractPricePerChildCents} {
		def := config.GetDefinition(key)
		require.NotNil(t, def, key)
		assert.Equalf(t, 0, def.Default, "setting %q must default to 0", key)
	}
}

// TestContractSettings_SelectOptionsAreCanonical keeps the stored values in
// sync with the constants the service maps to labels.
func TestContractSettings_SelectOptionsAreCanonical(t *testing.T) {
	t.Parallel()

	tier := config.GetDefinition(config.KeyContractTier)
	require.NotNil(t, tier)
	require.NotNil(t, tier.Options)
	assert.Equal(t, []any{
		config.ContractTierUnset,
		config.ContractTierTest,
		config.ContractTierBasis,
		config.ContractTierPlus,
		config.ContractTierPremium,
	}, optionValues(tier.Options.Static))

	cycle := config.GetDefinition(config.KeyContractBillingCycle)
	require.NotNil(t, cycle)
	require.NotNil(t, cycle.Options)
	assert.Equal(t, []any{
		config.ContractCycleUnset,
		config.ContractCycleMonthly,
		config.ContractCycleQuarterly,
		config.ContractCycleYearly,
	}, optionValues(cycle.Options.Static))
}

func optionValues(options []config.SelectOption) []any {
	values := make([]any, 0, len(options))
	for _, option := range options {
		values = append(values, option.Value)
	}
	return values
}

// TestContractSettings_NumericBounds guards against a fat-fingered contingent
// or price that the UI would then render as fact.
func TestContractSettings_NumericBounds(t *testing.T) {
	t.Parallel()

	children := config.GetDefinition(config.KeyContractBookedChildren)
	require.NotNil(t, children)
	require.NotNil(t, children.Validation)
	require.NotNil(t, children.Validation.Min)
	require.NotNil(t, children.Validation.Max)
	assert.Equal(t, 0.0, *children.Validation.Min)
	assert.Equal(t, 10000.0, *children.Validation.Max)

	price := config.GetDefinition(config.KeyContractPricePerChildCents)
	require.NotNil(t, price)
	require.NotNil(t, price.Validation)
	require.NotNil(t, price.Validation.Min)
	assert.Equal(t, 0.0, *price.Validation.Min)
}

// TestContractSettings_EmailFieldsAllowEmpty makes sure an unset recipient is
// a legal state — the pattern must not fire on "".
func TestContractSettings_EmailFieldsAllowEmpty(t *testing.T) {
	t.Parallel()

	for _, key := range []string{config.KeyContractInvoiceRecipient, config.KeyContractSupportEmail} {
		def := config.GetDefinition(key)
		require.NotNil(t, def, key)
		require.NotNil(t, def.Validation, key)
		require.NotNil(t, def.Validation.Pattern, key)
		assert.Truef(t, def.Validation.AllowEmpty, "setting %q must accept an empty value", key)

		require.NotNil(t, def.Validation.CompiledPattern, key)
		assert.True(t, def.Validation.CompiledPattern.MatchString("buchhaltung@schule-am-berg.de"), key)
		assert.False(t, def.Validation.CompiledPattern.MatchString("keine-email"), key)
	}
}

// TestContractSettings_CustomerNumberPattern rejects the characters that would
// break a CSV export of the accounting numbers later.
func TestContractSettings_CustomerNumberPattern(t *testing.T) {
	t.Parallel()

	def := config.GetDefinition(config.KeyContractCustomerNumber)
	require.NotNil(t, def)
	require.NotNil(t, def.Validation)
	require.NotNil(t, def.Validation.CompiledPattern)
	assert.True(t, def.Validation.AllowEmpty)

	assert.True(t, def.Validation.CompiledPattern.MatchString("K-10023"))
	assert.True(t, def.Validation.CompiledPattern.MatchString("2026/0042"))
	assert.False(t, def.Validation.CompiledPattern.MatchString("K;10023"))
	assert.False(t, def.Validation.CompiledPattern.MatchString(""),
		"empty is handled by AllowEmpty, not by the pattern")
}

// TestContractSettings_TabIsNotAbrechnung is the twin-heading guard from
// .claude/rules/verstaendlichkeit.md: "Abrechnung" already names the DATEV
// staff-payroll surface. Two tabs with that stem would be read as duplicates.
func TestContractSettings_TabIsNotAbrechnung(t *testing.T) {
	t.Parallel()

	for _, key := range contractKeys {
		def := config.GetDefinition(key)
		require.NotNil(t, def, key)
		assert.NotEqualf(t, "abrechnung", def.Tab,
			"setting %q must not share the payroll tab", key)
	}
}
