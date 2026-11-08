package records

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStringPtr(value string) *string { return &value }

func setup(t *testing.T) *Registry {
	t.Helper()
	return NewRegistry()
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
	t.Parallel()
	registry := setup(t)

	registry.Register(validDefinition("test.setting"))

	def := registry.GetDefinition("test.setting")
	require.NotNil(t, def)
	assert.Equal(t, "test.setting", def.Key)
	assert.Equal(t, FieldText, def.Type)
	assert.Equal(t, "default", def.Default)
}

func TestRegister_DuplicateKeyPanics(t *testing.T) {
	t.Parallel()
	registry := setup(t)
	registry.Register(validDefinition("test.dup"))
	assert.Panics(t, func() {
		registry.Register(validDefinition("test.dup"))
	})
}

func TestRegister_InvalidDefinitionPanics(t *testing.T) {
	t.Parallel()
	registry := setup(t)

	// Missing key
	assert.Panics(t, func() {
		registry.Register(Definition{
			Type:     FieldText,
			Tab:      "general",
			Category: "test",
		})
	})

	// Unknown field type
	assert.Panics(t, func() {
		registry.Register(Definition{
			Key:      "test.bad_type",
			Type:     "unknown",
			Tab:      "general",
			Category: "test",
		})
	})

	// Select without options
	assert.Panics(t, func() {
		registry.Register(Definition{
			Key:      "test.select_no_opts",
			Type:     FieldSelect,
			Tab:      "general",
			Category: "test",
		})
	})
}

func TestGetDefinition_NotFound(t *testing.T) {
	t.Parallel()
	registry := setup(t)
	assert.Nil(t, registry.GetDefinition("nonexistent"))
}

func TestRegister_DefaultsAccessPolicyToShared(t *testing.T) {
	t.Parallel()
	registry := setup(t)
	registry.Register(validDefinition("test.no_policy"))

	def := registry.GetDefinition("test.no_policy")
	require.NotNil(t, def)
	assert.Equal(t, AccessShared, def.AccessPolicy,
		"registry.Register() should backfill empty AccessPolicy to AccessShared")
}

func TestRegister_AcceptsAdminAndOperatorOnly(t *testing.T) {
	t.Parallel()
	registry := setup(t)

	adminOnly := validDefinition("test.admin_only")
	adminOnly.AccessPolicy = AccessAdminOnly
	registry.Register(adminOnly)
	assert.Equal(t, AccessAdminOnly, registry.GetDefinition("test.admin_only").AccessPolicy)

	operatorOnly := validDefinition("test.operator_only")
	operatorOnly.AccessPolicy = AccessOperatorOnly
	registry.Register(operatorOnly)
	assert.Equal(t, AccessOperatorOnly, registry.GetDefinition("test.operator_only").AccessPolicy)
}

func TestRegister_InvalidAccessPolicyPanics(t *testing.T) {
	t.Parallel()
	registry := setup(t)

	def := validDefinition("test.bogus_policy")
	def.AccessPolicy = AccessPolicy("not_a_valid_policy")
	assert.Panics(t, func() { registry.Register(def) })
}

func TestAllDefinitions(t *testing.T) {
	t.Parallel()
	registry := setup(t)

	registry.Register(validDefinition("test.one"))
	registry.Register(validDefinition("test.two"))

	all := registry.AllDefinitions()
	assert.Len(t, all, 2)
	assert.NotNil(t, all["test.one"])
	assert.NotNil(t, all["test.two"])
}

func TestDefinitionValidate_OptionsOnNonSelect(t *testing.T) {
	t.Parallel()

	// Text fields take presets since #6042 (TestDefinitionValidate_TextPresets);
	// every other non-select type still refuses options.
	def := Definition{
		Key:      "test.opts_on_number",
		Type:     FieldNumber,
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

func colorDefinition(presets ...SelectOption) Definition {
	pattern := `^#[0-9A-Fa-f]{6}$`
	return Definition{
		Key:        "test.color",
		Type:       FieldColor,
		Default:    "#111827",
		Tab:        "general",
		Category:   "test",
		Validation: &ValidationRules{Pattern: &pattern},
		Options:    &SelectOptions{Static: presets},
	}
}

func TestDefinitionValidate_ColorPresets(t *testing.T) {
	t.Parallel()

	valid := colorDefinition(SelectOption{Label: "Schwarz", Value: "#111827"})
	assert.NoError(t, valid.Validate())

	for name, def := range map[string]Definition{
		"empty":            colorDefinition(),
		"no label":         colorDefinition(SelectOption{Value: "#111827"}),
		"not a string":     colorDefinition(SelectOption{Label: "Schwarz", Value: 17}),
		"pattern mismatch": colorDefinition(SelectOption{Label: "Rot", Value: "red"}),
	} {
		assert.Error(t, def.Validate(), name)
	}
}

func TestNewRegistryIsEmpty(t *testing.T) {
	t.Parallel()
	registry := setup(t)
	registry.Register(validDefinition("test.reset"))
	assert.NotNil(t, registry.GetDefinition("test.reset"))

	assert.Nil(t, NewRegistry().GetDefinition("test.reset"))
}

func TestRegister_WithDependency(t *testing.T) {
	t.Parallel()
	registry := setup(t)

	registry.Register(Definition{
		Key:      "test.parent",
		Type:     FieldBoolean,
		Default:  true,
		Tab:      "general",
		Category: "test",
	})

	registry.Register(Definition{
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

	child := registry.GetDefinition("test.child")
	require.NotNil(t, child)
	require.NotNil(t, child.DependsOn)
	assert.Equal(t, "test.parent", child.DependsOn.Key)
}

func TestRegister_AllFieldTypes(t *testing.T) {
	t.Parallel()
	registry := setup(t)

	for _, ft := range []FieldType{
		FieldBoolean, FieldNumber, FieldTime,
		FieldText, FieldColor, FieldPassword,
	} {
		def := validDefinition("test." + string(ft))
		def.Type = ft
		registry.Register(def)
	}

	registry.Register(Definition{
		Key:      "test.select",
		Type:     FieldSelect,
		Tab:      "general",
		Category: "test",
		Options:  &SelectOptions{Static: []SelectOption{{Label: "A", Value: "a"}}},
	})

	assert.Len(t, registry.AllDefinitions(), 7)
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

// A length limit belongs to a text field and is positive (#5622).
func TestValidate_MaxLength(t *testing.T) {
	t.Parallel()

	length := func(n int) *int { return &n }
	for name, tc := range map[string]struct {
		fieldType FieldType
		max       int
		wantErr   string
	}{
		"textarea":       {fieldType: FieldTextarea, max: 600},
		"text":           {fieldType: FieldText, max: 35},
		"number refused": {fieldType: FieldNumber, max: 10, wantErr: "only be set for text fields"},
		"zero refused":   {fieldType: FieldTextarea, max: 0, wantErr: "must be positive"},
	} {
		def := Definition{
			Key: "test.max_length", Type: tc.fieldType, Tab: "system", Category: "general",
			Validation: &ValidationRules{MaxLength: length(tc.max)},
		}
		err := def.Validate()
		if tc.wantErr == "" {
			assert.NoError(t, err, name)
			continue
		}
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.wantErr, name)
	}
}

// An example text belongs to a textarea, the one input that shows it (#5693).
func TestValidate_Placeholder(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		fieldType FieldType
		wantErr   string
	}{
		"textarea":       {fieldType: FieldTextarea},
		"text refused":   {fieldType: FieldText, wantErr: "only be set for textarea fields"},
		"number refused": {fieldType: FieldNumber, wantErr: "only be set for textarea fields"},
	} {
		def := Definition{
			Key: "test.placeholder", Type: tc.fieldType, Tab: "system", Category: "general",
			Placeholder: "Musterstraße 1",
		}
		err := def.Validate()
		if tc.wantErr == "" {
			assert.NoError(t, err, name)
			continue
		}
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.wantErr, name)
	}
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
	t.Parallel()
	registry := setup(t)
	registry.Register(validDefinition("test.copy"))

	defs := registry.AllDefinitions()
	defs["test.copy"].Label = "MUTATED"

	// Original in registry should be unchanged
	orig := registry.GetDefinition("test.copy")
	assert.Equal(t, "Test Setting", orig.Label)
}

func textPresetDefinition(presets ...SelectOption) Definition {
	pattern := `^[A-Z]+$`
	return Definition{
		Key:        "test.text_presets",
		Type:       FieldText,
		Default:    "OGS",
		Tab:        "general",
		Category:   "test",
		Validation: &ValidationRules{Pattern: &pattern},
		Options:    &SelectOptions{Static: presets},
	}
}

// A text field may offer labelled presets like a colour field (#6042): the
// presets are quick picks, never a restriction of the value.
func TestDefinitionValidate_TextPresets(t *testing.T) {
	t.Parallel()

	valid := textPresetDefinition(SelectOption{Label: "OGS", Value: "OGS"}, SelectOption{Label: "GTS", Value: "GTS"})
	assert.NoError(t, valid.Validate())

	for name, def := range map[string]Definition{
		"empty":            textPresetDefinition(),
		"no label":         textPresetDefinition(SelectOption{Value: "OGS"}),
		"not a string":     textPresetDefinition(SelectOption{Label: "OGS", Value: 17}),
		"pattern mismatch": textPresetDefinition(SelectOption{Label: "Ogs", Value: "ogs"}),
	} {
		assert.Error(t, def.Validate(), name)
	}

	textarea := textPresetDefinition(SelectOption{Label: "OGS", Value: "OGS"})
	textarea.Type = FieldTextarea
	assert.Error(t, textarea.Validate(), "presets stay limited to single-line text and colour fields")
}
