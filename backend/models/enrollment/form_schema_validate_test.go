package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FormSchema.Validate is the last guard the schema service runs before
// it writes a new row. The duplicate-key + per-field validation here
// is what stops a broken schema from poisoning every parent submission.

func validSchema() *FormSchema {
	return &FormSchema{
		Name:      "Schuljahr",
		Version:   1,
		CreatedBy: 4321,
		Fields: []FormField{
			{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		},
	}
}

func TestFormSchema_Validate_HappyPath(t *testing.T) {
	assert.NoError(t, validSchema().Validate())
}

func TestFormSchema_Validate_RequiresName(t *testing.T) {
	s := validSchema()
	s.Name = ""
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestFormSchema_Validate_RequiresPositiveVersion(t *testing.T) {
	s := validSchema()
	s.Version = 0
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestFormSchema_Validate_RejectsNegativeVersion(t *testing.T) {
	s := validSchema()
	s.Version = -1
	err := s.Validate()
	require.Error(t, err)
}

func TestFormSchema_Validate_RequiresCreatedBy(t *testing.T) {
	s := validSchema()
	s.CreatedBy = 0
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_by")
}

func TestFormSchema_Validate_RejectsNegativeCreatedBy(t *testing.T) {
	s := validSchema()
	s.CreatedBy = -1
	err := s.Validate()
	require.Error(t, err)
}

func TestFormSchema_Validate_EmptyFieldsOK(t *testing.T) {
	// A "no extra fields" schema is legal — admins might publish a
	// schema with only the standard fields enabled.
	s := validSchema()
	s.Fields = nil
	assert.NoError(t, s.Validate())
}

func TestFormSchema_Validate_RejectsUnknownCoreRequirement(t *testing.T) {
	s := validSchema()
	s.CoreRequirements = CoreRequirements{"guardian_fax": true}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown core requirement")
}

func TestFormSchema_Validate_AcceptsLegalBlocks(t *testing.T) {
	s := validSchema()
	s.LegalBlocks = []FormLegalBlock{
		{
			Key:      ConsentKeyDataProcessing,
			Kind:     LegalBlockKindPrivacyNotice,
			Title:    "Datenschutzinformation",
			Label:    "Ich habe die Datenschutzinformation zur Kenntnis genommen.",
			Required: true,
			Enabled:  true,
			Source:   LegalBlockSourceStandard,
		},
		{
			Key:      "custom_pool",
			Kind:     LegalBlockKindConsent,
			Title:    "Schwimmbad",
			Label:    "Mein Kind darf am Schwimmbad-Ausflug teilnehmen.",
			Required: false,
			Enabled:  true,
			Source:   LegalBlockSourceCustom,
		},
	}

	require.NoError(t, s.Validate())
}

func TestFormSchema_Validate_AcceptsAGBPDFLegalBlock(t *testing.T) {
	s := validSchema()
	s.LegalBlocks = []FormLegalBlock{
		{
			Key:         ConsentKeyAGB,
			Kind:        LegalBlockKindTerms,
			Title:       "AGB / Teilnahmebedingungen",
			Label:       "Ich akzeptiere die AGB.",
			Required:    true,
			Enabled:     true,
			Source:      LegalBlockSourceStandard,
			DisplayMode: LegalBlockDisplayModePDF,
			DocumentURL: "/uploads/enrollment-legal-documents/1_terms.pdf",
		},
	}

	require.NoError(t, s.Validate())
	assert.Equal(t, LegalBlockDisplayModePDF, s.LegalBlocks[0].DisplayMode)
}

func TestFormSchema_Validate_AcceptsDisabledDataProcessingWithEnabledBlocks(t *testing.T) {
	// Deliberately supported: pilot schools without a standalone
	// Datenschutzinformation run their consent via the Elternbrief/AGB
	// block, so a template may enable blocks while keeping
	// data_processing disabled. The editor shows a non-blocking hint.
	s := validSchema()
	s.LegalBlocks = []FormLegalBlock{
		{
			Key:      ConsentKeyDataProcessing,
			Kind:     LegalBlockKindPrivacyNotice,
			Title:    "Datenschutzinformation",
			Label:    "Ich habe die Datenschutzinformation zur Kenntnis genommen.",
			Required: true,
			Enabled:  false,
			Source:   LegalBlockSourceStandard,
		},
		{
			Key:     "custom_pool",
			Kind:    LegalBlockKindConsent,
			Title:   "Schwimmbad",
			Label:   "Mein Kind darf am Schwimmbad-Ausflug teilnehmen.",
			Enabled: true,
		},
	}

	require.NoError(t, s.Validate())
}

func TestFormSchema_Validate_AcceptsAllDisabledLegalBlocks(t *testing.T) {
	// An all-disabled snapshot stays valid — the submission service falls
	// back to the tenant-wide legal settings for those templates.
	s := validSchema()
	s.LegalBlocks = []FormLegalBlock{
		{
			Key:     ConsentKeyDataProcessing,
			Kind:    LegalBlockKindPrivacyNotice,
			Title:   "Datenschutzinformation",
			Label:   "Ich habe die Datenschutzinformation zur Kenntnis genommen.",
			Enabled: false,
		},
		{
			Key:     "custom_pool",
			Kind:    LegalBlockKindConsent,
			Title:   "Schwimmbad",
			Label:   "Mein Kind darf am Schwimmbad-Ausflug teilnehmen.",
			Enabled: false,
		},
	}

	require.NoError(t, s.Validate())
}

func TestFormSchema_Validate_RejectsDuplicateLegalBlockKey(t *testing.T) {
	s := validSchema()
	s.LegalBlocks = []FormLegalBlock{
		{Key: "custom_pool", Kind: LegalBlockKindConsent, Title: "A", Label: "A", Enabled: true},
		{Key: "custom_pool", Kind: LegalBlockKindConsent, Title: "B", Label: "B", Enabled: true},
	}

	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate legal block key")
}

func TestFormSchema_Validate_RejectsInvalidLegalBlockShape(t *testing.T) {
	tests := []struct {
		name  string
		block FormLegalBlock
		want  string
	}{
		{
			name:  "bad key",
			block: FormLegalBlock{Key: "Bad Key", Kind: LegalBlockKindConsent, Title: "T", Label: "L", Enabled: true},
			want:  "lowercase letters",
		},
		{
			name:  "custom reserves standard key",
			block: FormLegalBlock{Key: ConsentKeyPhoto, Kind: LegalBlockKindConsent, Title: "T", Label: "L", Enabled: true, Source: LegalBlockSourceCustom},
			want:  "reserved",
		},
		{
			name:  "standard unknown key",
			block: FormLegalBlock{Key: "custom_pool", Kind: LegalBlockKindConsent, Title: "T", Label: "L", Enabled: true, Source: LegalBlockSourceStandard},
			want:  "not recognized",
		},
		{
			name:  "unknown kind",
			block: FormLegalBlock{Key: "custom_pool", Kind: "whatever", Title: "T", Label: "L", Enabled: true},
			want:  "unknown kind",
		},
		{
			name:  "enabled needs title",
			block: FormLegalBlock{Key: "custom_pool", Kind: LegalBlockKindConsent, Label: "L", Enabled: true},
			want:  "requires a title",
		},
		{
			name:  "notice cannot be required",
			block: FormLegalBlock{Key: "custom_notice", Kind: LegalBlockKindNotice, Title: "T", Label: "L", Required: true, Enabled: true},
			want:  "cannot be required",
		},
		{
			name:  "pdf mode only allowed for agb",
			block: FormLegalBlock{Key: ConsentKeyPhoto, Kind: LegalBlockKindConsent, Title: "T", Label: "L", Enabled: true, Source: LegalBlockSourceStandard, DisplayMode: LegalBlockDisplayModePDF, DocumentURL: "/uploads/enrollment-legal-documents/1_terms.pdf"},
			want:  "cannot use PDF display mode",
		},
		{
			name:  "enabled pdf mode needs document",
			block: FormLegalBlock{Key: ConsentKeyAGB, Kind: LegalBlockKindTerms, Title: "T", Label: "L", Enabled: true, Source: LegalBlockSourceStandard, DisplayMode: LegalBlockDisplayModePDF},
			want:  "requires a PDF document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSchema()
			s.LegalBlocks = []FormLegalBlock{tt.block}

			err := s.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestFormSchema_Validate_RejectsNonScalarVisibleWhenValue(t *testing.T) {
	// JSON decodes arrays/objects as []any / map[string]any. Comparing two
	// such values at evaluation time panics, so a non-scalar condition value
	// must be rejected at schema-save time with a clean error.
	for name, value := range map[string]any{
		"array":  []any{"1", "2"},
		"object": map[string]any{"k": "v"},
	} {
		t.Run(name, func(t *testing.T) {
			s := validSchema()
			s.Fields = []FormField{{
				Key:         "x",
				Label:       "X",
				Type:        FormFieldText,
				AppliesToCh: true,
				SortOrder:   0,
				VisibleWhen: &VisibilityCondition{
					Source:   ConditionSourceGradeLevel,
					Operator: ConditionOpEquals,
					Value:    value,
				},
			}}
			err := s.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "string, number, or boolean")
		})
	}
}

func TestFormSchema_Validate_AcceptsScalarVisibleWhenValue(t *testing.T) {
	s := validSchema()
	s.Fields = []FormField{{
		Key:         "x",
		Label:       "X",
		Type:        FormFieldText,
		AppliesToCh: true,
		SortOrder:   0,
		VisibleWhen: &VisibilityCondition{
			Source:   ConditionSourceGradeLevel,
			Operator: ConditionOpEquals,
			Value:    float64(2), // JSON numbers decode to float64
		},
	}}
	assert.NoError(t, s.Validate())
}

func TestFormSchema_Validate_RejectsDuplicateKey(t *testing.T) {
	s := validSchema()
	s.Fields = []FormField{
		{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		{Key: "allergies", Label: "Doppelt", Type: FormFieldText, SortOrder: 1},
	}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "allergies")
}

func TestFormSchema_Validate_RejectsDuplicateReservedTarget(t *testing.T) {
	// Two fields pointing at the same Stammdaten target produce an undefined
	// last-field-wins outcome on approval — the departure plan can silently
	// collapse back into self-goer semantics (#1694), so the schema is rejected.
	s := validSchema()
	s.Fields = []FormField{
		{Key: "heimweg_a", Label: "Erlaubte Heimwege", Type: FormFieldWeekdayMultiMode, Target: TargetStudentAllowedDepartureModes, AppliesToCh: true, SortOrder: 0},
		{Key: "heimweg_b", Label: "Heimwege (Kopie)", Type: FormFieldWeekdayMultiMode, Target: TargetStudentAllowedDepartureModes, AppliesToCh: true, SortOrder: 1},
	}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate field target")
	assert.Contains(t, err.Error(), TargetStudentAllowedDepartureModes)
	assert.Contains(t, err.Error(), "heimweg_a")
	assert.Contains(t, err.Error(), "heimweg_b")
}

func TestFormSchema_Validate_AllowsRepeatedEmptyTarget(t *testing.T) {
	// Free custom fields have no target and may repeat freely.
	s := validSchema()
	s.Fields = []FormField{
		{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		{Key: "diet", Label: "Diät", Type: FormFieldText, SortOrder: 1},
	}
	assert.NoError(t, s.Validate())
}

func TestFormSchema_Validate_AllowsDistinctLegacyDepartureTargets(t *testing.T) {
	// The legacy split — Buskind and Abholregelung — are DIFFERENT targets and
	// legitimately coexist; only the same target twice is rejected.
	s := validSchema()
	s.Fields = []FormField{
		{Key: "bus", Label: "Buskind", Type: FormFieldWeekdayBoolean, Target: TargetStudentBus, AppliesToCh: true, SortOrder: 0},
		{Key: "abhol", Label: "Abholregelung", Type: FormFieldWeekdayBoolean, Target: TargetStudentPickupStatus, AppliesToCh: true, SortOrder: 1},
	}
	assert.NoError(t, s.Validate())
}

func TestFormSchema_Validate_PropagatesFieldErrorWithIndex(t *testing.T) {
	// One bad field should fail validation with the offending index in
	// the error so the admin can find it in the editor list.
	s := validSchema()
	s.Fields = []FormField{
		{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		{Key: "", Label: "Missing key", Type: FormFieldText, SortOrder: 1},
	}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field 1", "error must name the failing index")
}

func TestFormSchema_Validate_AcceptsMultipleDistinctFields(t *testing.T) {
	s := validSchema()
	s.Fields = []FormField{
		{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		{Key: "diet", Label: "Diät", Type: FormFieldText, SortOrder: 1},
		{Key: "notes", Label: "Notizen", Type: FormFieldTextarea, SortOrder: 2},
	}
	assert.NoError(t, s.Validate())
}

// ---- Heimweg-Beschränkung (single_mode_grades, #2381) ---------------------

func singleModeField() FormField {
	return FormField{
		Key: "heimwege", Label: "Erlaubte Heimwege",
		Type: FormFieldWeekdayMultiMode, AppliesToCh: true,
		Target:           TargetStudentAllowedDepartureModes,
		SingleModeGrades: []int{1},
	}
}

func TestFormField_Validate_SingleModeGradesHappyPath(t *testing.T) {
	f := singleModeField()
	assert.NoError(t, f.Validate())
	assert.Equal(t, []int{1}, f.SingleModeGrades)
}

func TestFormField_Validate_SingleModeGradesWrongTarget(t *testing.T) {
	f := FormField{
		Key: "pickup_times", Label: "Abholzeiten",
		Type: FormFieldWeekdaySchedule, AppliesToCh: true,
		Target:           TargetSchedulePickup,
		SingleModeGrades: []int{1},
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single_mode_grades")
}

func TestFormField_Validate_SingleModeGradesRequiresChildField(t *testing.T) {
	f := singleModeField()
	f.AppliesToCh = false
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single_mode_grades")
}

func TestFormField_Validate_SingleModeGradesOutOfRange(t *testing.T) {
	f := singleModeField()
	f.SingleModeGrades = []int{0}
	require.Error(t, f.Validate())
	f = singleModeField()
	f.SingleModeGrades = []int{14}
	require.Error(t, f.Validate())
}

func TestFormField_Validate_SingleModeGradesDeduped(t *testing.T) {
	f := singleModeField()
	f.SingleModeGrades = []int{1, 2, 1}
	require.NoError(t, f.Validate())
	assert.Equal(t, []int{1, 2}, f.SingleModeGrades)
}

func TestFormField_Validate_InfoFieldRejectsSingleModeGrades(t *testing.T) {
	f := FormField{
		Key: "hinweis", Type: FormFieldInfo, Content: "Text",
		SingleModeGrades: []int{1},
	}
	err := f.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single_mode_grades")
}

func TestFormField_SingleModeAppliesTo(t *testing.T) {
	f := singleModeField()
	one, two := int16(1), int16(2)
	assert.True(t, f.SingleModeAppliesTo(&one))
	assert.False(t, f.SingleModeAppliesTo(&two))
	assert.False(t, f.SingleModeAppliesTo(nil), "children without a target grade are never restricted")
}

func TestWeekdayMultiMode_ValidateSingleSelection(t *testing.T) {
	ok := WeekdayMultiMode{"mon": {"bus"}, "tue": {"pickup"}}
	assert.NoError(t, ok.ValidateSingleSelection(),
		"one mode per day is fine, even different modes across days")

	bad := WeekdayMultiMode{"mon": {"bus", "pickup"}}
	err := bad.ValidateSingleSelection()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one departure mode")

	malformed := WeekdayMultiMode{"mon": {"jetpack"}}
	require.Error(t, malformed.ValidateSingleSelection())
}
