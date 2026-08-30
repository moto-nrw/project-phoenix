// Deliberately NOT parallel (whole package): the setting registry these tests
// exercise is a package-global map, and they register and clear entries in it
// (#2419).
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStringPtr(value string) *string { return &value }

func setup(t *testing.T) {
	t.Helper()
	ResetRegistry()
	t.Cleanup(func() { ResetRegistry() })
}

func validDefinition(key string) Definition {
	return Definition{
		Key:      key,
		Label:    "Test Setting",
		Type:     FieldText,
		Default:  "default",
		Tab:      "general",
		Category: "test",
	}
}

func TestRegister_Success(t *testing.T) {
	setup(t)

	Register(validDefinition("test.setting"))

	def := GetDefinition("test.setting")
	require.NotNil(t, def)
	assert.Equal(t, "test.setting", def.Key)
	assert.Equal(t, FieldText, def.Type)
	assert.Equal(t, "default", def.Default)
}

func TestRegister_DuplicateKeyPanics(t *testing.T) {
	setup(t)
	Register(validDefinition("test.dup"))
	assert.Panics(t, func() {
		Register(validDefinition("test.dup"))
	})
}

func TestRegister_InvalidDefinitionPanics(t *testing.T) {
	setup(t)

	// Missing key
	assert.Panics(t, func() {
		Register(Definition{
			Type:     FieldText,
			Tab:      "general",
			Category: "test",
		})
	})

	// Unknown field type
	assert.Panics(t, func() {
		Register(Definition{
			Key:      "test.bad_type",
			Type:     "unknown",
			Tab:      "general",
			Category: "test",
		})
	})

	// Select without options
	assert.Panics(t, func() {
		Register(Definition{
			Key:      "test.select_no_opts",
			Type:     FieldSelect,
			Tab:      "general",
			Category: "test",
		})
	})
}

func TestGetDefinition_NotFound(t *testing.T) {
	setup(t)
	assert.Nil(t, GetDefinition("nonexistent"))
}

func TestRegister_DefaultsAccessPolicyToShared(t *testing.T) {
	setup(t)
	Register(validDefinition("test.no_policy"))

	def := GetDefinition("test.no_policy")
	require.NotNil(t, def)
	assert.Equal(t, AccessShared, def.AccessPolicy,
		"Register() should backfill empty AccessPolicy to AccessShared")
}

func TestRegister_AcceptsAdminAndOperatorOnly(t *testing.T) {
	setup(t)

	adminOnly := validDefinition("test.admin_only")
	adminOnly.AccessPolicy = AccessAdminOnly
	Register(adminOnly)
	assert.Equal(t, AccessAdminOnly, GetDefinition("test.admin_only").AccessPolicy)

	operatorOnly := validDefinition("test.operator_only")
	operatorOnly.AccessPolicy = AccessOperatorOnly
	Register(operatorOnly)
	assert.Equal(t, AccessOperatorOnly, GetDefinition("test.operator_only").AccessPolicy)
}

func TestRegister_InvalidAccessPolicyPanics(t *testing.T) {
	setup(t)

	def := validDefinition("test.bogus_policy")
	def.AccessPolicy = AccessPolicy("not_a_valid_policy")
	assert.Panics(t, func() { Register(def) })
}

func TestAllDefinitions(t *testing.T) {
	setup(t)

	Register(validDefinition("test.one"))
	Register(validDefinition("test.two"))

	all := AllDefinitions()
	assert.Len(t, all, 2)
	assert.NotNil(t, all["test.one"])
	assert.NotNil(t, all["test.two"])
}

func TestDefinitionValidate_OptionsOnNonSelect(t *testing.T) {
	t.Parallel()

	def := Definition{
		Key:      "test.opts_on_text",
		Type:     FieldText,
		Tab:      "general",
		Category: "test",
		Options: &SelectOptions{
			Static: []SelectOption{{Label: "A", Value: "a"}},
		},
	}
	assert.Error(t, def.Validate())
}

func TestDefinitionValidate_SelectWithOptions(t *testing.T) {
	t.Parallel()

	def := Definition{
		Key:      "test.valid_select",
		Type:     FieldSelect,
		Tab:      "general",
		Category: "test",
		Options: &SelectOptions{
			Static: []SelectOption{{Label: "A", Value: "a"}},
		},
	}
	assert.NoError(t, def.Validate())
}

func TestResetRegistry(t *testing.T) {
	setup(t)
	Register(validDefinition("test.reset"))
	assert.NotNil(t, GetDefinition("test.reset"))

	ResetRegistry()
	assert.Nil(t, GetDefinition("test.reset"))
}

func TestRegister_WithDependency(t *testing.T) {
	setup(t)

	Register(Definition{
		Key:      "test.parent",
		Type:     FieldBoolean,
		Default:  true,
		Tab:      "general",
		Category: "test",
	})

	Register(Definition{
		Key:      "test.child",
		Type:     FieldText,
		Default:  "value",
		Tab:      "general",
		Category: "test",
		DependsOn: &Dependency{
			Key:       "test.parent",
			Condition: "eq",
			Value:     true,
		},
	})

	child := GetDefinition("test.child")
	require.NotNil(t, child)
	require.NotNil(t, child.DependsOn)
	assert.Equal(t, "test.parent", child.DependsOn.Key)
}

func TestRegister_AllFieldTypes(t *testing.T) {
	setup(t)

	for _, ft := range []FieldType{
		FieldBoolean, FieldNumber, FieldTime,
		FieldText, FieldPassword,
	} {
		def := validDefinition("test." + string(ft))
		def.Type = ft
		Register(def)
	}

	Register(Definition{
		Key:      "test.select",
		Type:     FieldSelect,
		Tab:      "general",
		Category: "test",
		Options:  &SelectOptions{Static: []SelectOption{{Label: "A", Value: "a"}}},
	})

	assert.Len(t, AllDefinitions(), 6)
}

func TestValidate_InvalidPattern(t *testing.T) {
	t.Parallel()

	def := Definition{
		Key:        "test.bad_pattern",
		Type:       FieldPassword,
		Tab:        "security",
		Category:   "auth",
		Validation: &ValidationRules{Pattern: testStringPtr("[invalid(regex")},
	}
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid validation pattern")
}

func TestValidate_ValidPattern(t *testing.T) {
	t.Parallel()

	def := Definition{
		Key:        "test.good_pattern",
		Type:       FieldPassword,
		Tab:        "security",
		Category:   "auth",
		Validation: &ValidationRules{Pattern: testStringPtr(`^\d{4}$`)},
	}
	err := def.Validate()
	require.NoError(t, err)
	assert.NotNil(t, def.Validation.CompiledPattern)
}

func TestValidate_DefaultBelowMin(t *testing.T) {
	t.Parallel()

	minVal := 10.0
	def := Definition{
		Key:        "test.below_min",
		Type:       FieldNumber,
		Default:    5,
		Tab:        "general",
		Category:   "test",
		Validation: &ValidationRules{Min: &minVal},
	}
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below minimum")
}

func TestValidate_DefaultExceedsMax(t *testing.T) {
	t.Parallel()

	maxVal := 100.0
	def := Definition{
		Key:        "test.above_max",
		Type:       FieldNumber,
		Default:    150,
		Tab:        "general",
		Category:   "test",
		Validation: &ValidationRules{Max: &maxVal},
	}
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestValidate_DefaultWithinRange(t *testing.T) {
	t.Parallel()

	minVal := 1.0
	maxVal := 100.0
	def := Definition{
		Key:        "test.in_range",
		Type:       FieldNumber,
		Default:    50,
		Tab:        "general",
		Category:   "test",
		Validation: &ValidationRules{Min: &minVal, Max: &maxVal},
	}
	require.NoError(t, def.Validate())
}

func TestAllDefinitions_DeepCopy(t *testing.T) {
	setup(t)
	Register(validDefinition("test.copy"))

	defs := AllDefinitions()
	defs["test.copy"].Label = "MUTATED"

	// Original in registry should be unchanged
	orig := GetDefinition("test.copy")
	assert.Equal(t, "Test Setting", orig.Label)
}
