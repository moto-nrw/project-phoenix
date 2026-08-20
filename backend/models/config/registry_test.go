// Deliberately NOT parallel (whole package): the setting registry these tests
// exercise is a package-global map, and they register and clear entries in it
// (#2419).
package config_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) {
	t.Helper()
	config.ResetRegistry()
	t.Cleanup(func() { config.ResetRegistry() })
}

func validDefinition(key string) config.Definition {
	return config.Definition{
		Key:      key,
		Label:    "Test Setting",
		Type:     config.FieldText,
		Default:  "default",
		Tab:      "general",
		Category: "test",
	}
}

func TestRegister_Success(t *testing.T) {
	setup(t)

	config.Register(validDefinition("test.setting"))

	def := config.GetDefinition("test.setting")
	require.NotNil(t, def)
	assert.Equal(t, "test.setting", def.Key)
	assert.Equal(t, config.FieldText, def.Type)
	assert.Equal(t, "default", def.Default)
}

func TestRegister_DuplicateKeyPanics(t *testing.T) {
	setup(t)
	config.Register(validDefinition("test.dup"))
	assert.Panics(t, func() {
		config.Register(validDefinition("test.dup"))
	})
}

func TestRegister_InvalidDefinitionPanics(t *testing.T) {
	setup(t)

	// Missing key
	assert.Panics(t, func() {
		config.Register(config.Definition{
			Type:     config.FieldText,
			Tab:      "general",
			Category: "test",
		})
	})

	// Unknown field type
	assert.Panics(t, func() {
		config.Register(config.Definition{
			Key:      "test.bad_type",
			Type:     "unknown",
			Tab:      "general",
			Category: "test",
		})
	})

	// Select without options
	assert.Panics(t, func() {
		config.Register(config.Definition{
			Key:      "test.select_no_opts",
			Type:     config.FieldSelect,
			Tab:      "general",
			Category: "test",
		})
	})
}

func TestGetDefinition_NotFound(t *testing.T) {
	setup(t)
	assert.Nil(t, config.GetDefinition("nonexistent"))
}

func TestRegister_DefaultsAccessPolicyToShared(t *testing.T) {
	setup(t)
	config.Register(validDefinition("test.no_policy"))

	def := config.GetDefinition("test.no_policy")
	require.NotNil(t, def)
	assert.Equal(t, config.AccessShared, def.AccessPolicy,
		"Register() should backfill empty AccessPolicy to AccessShared")
}

func TestRegister_AcceptsAdminAndOperatorOnly(t *testing.T) {
	setup(t)

	adminOnly := validDefinition("test.admin_only")
	adminOnly.AccessPolicy = config.AccessAdminOnly
	config.Register(adminOnly)
	assert.Equal(t, config.AccessAdminOnly, config.GetDefinition("test.admin_only").AccessPolicy)

	operatorOnly := validDefinition("test.operator_only")
	operatorOnly.AccessPolicy = config.AccessOperatorOnly
	config.Register(operatorOnly)
	assert.Equal(t, config.AccessOperatorOnly, config.GetDefinition("test.operator_only").AccessPolicy)
}

func TestRegister_InvalidAccessPolicyPanics(t *testing.T) {
	setup(t)

	def := validDefinition("test.bogus_policy")
	def.AccessPolicy = config.AccessPolicy("not_a_valid_policy")
	assert.Panics(t, func() { config.Register(def) })
}

func TestAllDefinitions(t *testing.T) {
	setup(t)

	config.Register(validDefinition("test.one"))
	config.Register(validDefinition("test.two"))

	all := config.AllDefinitions()
	assert.Len(t, all, 2)
	assert.NotNil(t, all["test.one"])
	assert.NotNil(t, all["test.two"])
}

func TestDefinitionValidate_OptionsOnNonSelect(t *testing.T) {
	t.Parallel()

	def := config.Definition{
		Key:      "test.opts_on_text",
		Type:     config.FieldText,
		Tab:      "general",
		Category: "test",
		Options: &config.SelectOptions{
			Static: []config.SelectOption{{Label: "A", Value: "a"}},
		},
	}
	assert.Error(t, def.Validate())
}

func TestDefinitionValidate_SelectWithOptions(t *testing.T) {
	t.Parallel()

	def := config.Definition{
		Key:      "test.valid_select",
		Type:     config.FieldSelect,
		Tab:      "general",
		Category: "test",
		Options: &config.SelectOptions{
			Static: []config.SelectOption{{Label: "A", Value: "a"}},
		},
	}
	assert.NoError(t, def.Validate())
}

func TestResetRegistry(t *testing.T) {
	setup(t)
	config.Register(validDefinition("test.reset"))
	assert.NotNil(t, config.GetDefinition("test.reset"))

	config.ResetRegistry()
	assert.Nil(t, config.GetDefinition("test.reset"))
}

func TestRegister_WithDependency(t *testing.T) {
	setup(t)

	config.Register(config.Definition{
		Key:      "test.parent",
		Type:     config.FieldBoolean,
		Default:  true,
		Tab:      "general",
		Category: "test",
	})

	config.Register(config.Definition{
		Key:      "test.child",
		Type:     config.FieldText,
		Default:  "value",
		Tab:      "general",
		Category: "test",
		DependsOn: &config.Dependency{
			Key:       "test.parent",
			Condition: "eq",
			Value:     true,
		},
	})

	child := config.GetDefinition("test.child")
	require.NotNil(t, child)
	require.NotNil(t, child.DependsOn)
	assert.Equal(t, "test.parent", child.DependsOn.Key)
}

func TestRegister_AllFieldTypes(t *testing.T) {
	setup(t)

	for _, ft := range []config.FieldType{
		config.FieldBoolean, config.FieldNumber, config.FieldTime,
		config.FieldText, config.FieldPassword,
	} {
		def := validDefinition("test." + string(ft))
		def.Type = ft
		config.Register(def)
	}

	config.Register(config.Definition{
		Key:      "test.select",
		Type:     config.FieldSelect,
		Tab:      "general",
		Category: "test",
		Options:  &config.SelectOptions{Static: []config.SelectOption{{Label: "A", Value: "a"}}},
	})

	assert.Len(t, config.AllDefinitions(), 6)
}

func TestValidate_InvalidPattern(t *testing.T) {
	t.Parallel()

	def := config.Definition{
		Key:        "test.bad_pattern",
		Type:       config.FieldPassword,
		Tab:        "security",
		Category:   "auth",
		Validation: &config.ValidationRules{Pattern: testpkg.StrPtr("[invalid(regex")},
	}
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid validation pattern")
}

func TestValidate_ValidPattern(t *testing.T) {
	t.Parallel()

	def := config.Definition{
		Key:        "test.good_pattern",
		Type:       config.FieldPassword,
		Tab:        "security",
		Category:   "auth",
		Validation: &config.ValidationRules{Pattern: testpkg.StrPtr(`^\d{4}$`)},
	}
	err := def.Validate()
	require.NoError(t, err)
	assert.NotNil(t, def.Validation.CompiledPattern)
}

func TestValidate_DefaultBelowMin(t *testing.T) {
	t.Parallel()

	minVal := 10.0
	def := config.Definition{
		Key:        "test.below_min",
		Type:       config.FieldNumber,
		Default:    5,
		Tab:        "general",
		Category:   "test",
		Validation: &config.ValidationRules{Min: &minVal},
	}
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below minimum")
}

func TestValidate_DefaultExceedsMax(t *testing.T) {
	t.Parallel()

	maxVal := 100.0
	def := config.Definition{
		Key:        "test.above_max",
		Type:       config.FieldNumber,
		Default:    150,
		Tab:        "general",
		Category:   "test",
		Validation: &config.ValidationRules{Max: &maxVal},
	}
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestValidate_DefaultWithinRange(t *testing.T) {
	t.Parallel()

	minVal := 1.0
	maxVal := 100.0
	def := config.Definition{
		Key:        "test.in_range",
		Type:       config.FieldNumber,
		Default:    50,
		Tab:        "general",
		Category:   "test",
		Validation: &config.ValidationRules{Min: &minVal, Max: &maxVal},
	}
	require.NoError(t, def.Validate())
}

func TestAllDefinitions_DeepCopy(t *testing.T) {
	setup(t)
	config.Register(validDefinition("test.copy"))

	defs := config.AllDefinitions()
	defs["test.copy"].Label = "MUTATED"

	// Original in registry should be unchanged
	orig := config.GetDefinition("test.copy")
	assert.Equal(t, "Test Setting", orig.Label)
}
