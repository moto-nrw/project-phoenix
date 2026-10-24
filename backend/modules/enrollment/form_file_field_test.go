package enrollment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fileField() FormField {
	return FormField{
		Key: "nachweis", Label: "Einkommensnachweis", Type: FormFieldFile,
		FileOptions: &FormFileOptions{AllowedTypes: []string{" PDF ", "jpg", "pdf"}, MaxSizeMB: 10, Multiple: true},
	}
}

func TestFormField_FileValidatesAndNormalizesOptions(t *testing.T) {
	t.Parallel()

	field := fileField()
	require.NoError(t, field.Validate())
	assert.Equal(t, []string{"pdf", "jpg"}, field.FileOptions.AllowedTypes, "types are trimmed, lowercased and deduplicated")
	assert.Equal(t, MaxFilesPerField, field.FileOptions.MaxFiles())
	assert.Equal(t, int64(10*1024*1024), field.FileOptions.MaxBytes())
	assert.True(t, field.FileOptions.AllowsContentType("application/pdf"))
	assert.False(t, field.FileOptions.AllowsContentType("image/png"))

	single := FormFileOptions{AllowedTypes: []string{"png"}, MaxSizeMB: 1}
	assert.Equal(t, 1, single.MaxFiles())
}

func TestFormField_FileRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*FormField){
		"missing options": func(f *FormField) { f.FileOptions = nil },
		"no type":         func(f *FormField) { f.FileOptions.AllowedTypes = nil },
		"unknown type":    func(f *FormField) { f.FileOptions.AllowedTypes = []string{"docx"} },
		"size zero":       func(f *FormField) { f.FileOptions.MaxSizeMB = 0 },
		"size over limit": func(f *FormField) { f.FileOptions.MaxSizeMB = MaxFileFieldSizeMB + 1 },
		"per child":       func(f *FormField) { f.AppliesToCh = true },
		"with target":     func(f *FormField) { f.Target = TargetStudentExtraInfo },
		"with validation": func(f *FormField) { f.Validation = &FormFieldValidation{} },
		"options on text": func(f *FormField) { f.Type = FormFieldText },
		"options on info": func(f *FormField) { f.Type = FormFieldInfo; f.Content = "Hinweis" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			field := fileField()
			mutate(&field)
			err := field.Validate()
			require.Error(t, err)
			var rejection *InvalidInputError
			if errors.As(err, &rejection) {
				assert.Contains(t, []string{CodeFormFieldFileOptionsInvalid, CodeFormFieldTargetInvalid}, rejection.Code)
			}
		})
	}
}

func TestFormSchema_FileFieldCannotControlVisibility(t *testing.T) {
	t.Parallel()

	schema := validSchema()
	schema.Fields = append(schema.Fields, fileField(), FormField{
		Key: "hinweis", Label: "Hinweis", Type: FormFieldText, SortOrder: 2,
		VisibleWhen: &VisibilityCondition{Source: ConditionSourceField, Field: "nachweis", Operator: ConditionOpNotEmpty},
	})
	require.Error(t, schema.Validate())
}

func TestRequestUpload_AnswerCarriesNoToken(t *testing.T) {
	t.Parallel()

	upload := RequestUpload{ID: 42, Filename: "Bescheid.pdf", SizeBytes: 12, ContentType: "application/pdf", TokenHash: "hash"}
	assert.Equal(t, FileAnswer{UploadID: "42", Filename: "Bescheid.pdf", SizeBytes: 12, ContentType: "application/pdf"}, upload.Answer())
}
