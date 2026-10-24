package enrollment

import (
	"fmt"
	"strings"
)

// FormFieldFile collects documents from the parent, for example the proof of
// a reduced income level (#4783). The answer is a list of FileAnswer items;
// the bytes live in enrollment.request_uploads, never in custom_data.
const FormFieldFile FormFieldType = "file"

// CodeFormFieldFileOptionsInvalid rejects a file field without a usable
// type list or size limit.
const CodeFormFieldFileOptionsInvalid = "enrollment.form_field_file_options_invalid"

// File types a school may allow on a file field. The wire names are the ones
// the editor shows; FileTypeContentTypes maps them to the content types the
// upload check detects from the bytes.
const (
	FileTypePDF = "pdf"
	FileTypeJPG = "jpg"
	FileTypePNG = "png"
)

// FileTypeContentTypes maps an allowed file type to its detected content type.
var FileTypeContentTypes = map[string]string{
	FileTypePDF: "application/pdf",
	FileTypeJPG: "image/jpeg",
	FileTypePNG: "image/png",
}

// File field limits. One file is at most MaxFileFieldSizeMB; a field that
// allows several files takes at most MaxFilesPerField.
const (
	MaxFileFieldSizeMB = 10
	MaxFilesPerField   = 5
)

// FormFileOptions configures a FormFieldFile field.
type FormFileOptions struct {
	AllowedTypes []string `json:"allowed_types"`
	MaxSizeMB    int      `json:"max_size_mb"`
	Multiple     bool     `json:"multiple,omitempty"`
}

// MaxFiles is the number of files the field accepts.
func (o *FormFileOptions) MaxFiles() int {
	if o != nil && o.Multiple {
		return MaxFilesPerField
	}
	return 1
}

// MaxBytes is the size limit of one file.
func (o *FormFileOptions) MaxBytes() int64 {
	if o == nil {
		return 0
	}
	return int64(o.MaxSizeMB) * 1024 * 1024
}

// AllowsContentType reports whether a detected content type is allowed.
func (o *FormFileOptions) AllowsContentType(contentType string) bool {
	if o == nil {
		return false
	}
	for _, t := range o.AllowedTypes {
		if FileTypeContentTypes[t] == contentType {
			return true
		}
	}
	return false
}

// validateType checks the field type and the file-specific options.
func (f *FormField) validateType() error {
	if !validFormFieldTypes[f.Type] {
		return fmt.Errorf("unknown form field type %q", f.Type)
	}
	return f.validateFileOptions()
}

// validateFileOptions checks the file-specific shape: options only on file
// fields, a parent-level field without target, a known non-empty type list
// and a size within the limit.
func (f *FormField) validateFileOptions() error {
	if f.Type != FormFieldFile {
		if f.FileOptions != nil {
			return invalidInputf(CodeFormFieldFileOptionsInvalid, "file_options", "only file fields may declare file_options (field %q)", f.Key)
		}
		return nil
	}
	// The uploads belong to the request, not to one child; a per-child file
	// field would need a child reference the upload table does not carry.
	if f.AppliesToCh {
		return invalidInputf(CodeFormFieldFileOptionsInvalid, "applies_to_child", "file field %q must be a parent-level field", f.Key)
	}
	if f.Target != "" || f.Validation != nil {
		return invalidInputf(CodeFormFieldFileOptionsInvalid, "file_options", "file field %q must not declare a target or validation", f.Key)
	}
	o := f.FileOptions
	if o == nil || len(o.AllowedTypes) == 0 {
		return invalidInputf(CodeFormFieldFileOptionsInvalid, "file_options.allowed_types", "file field %q must allow at least one file type", f.Key)
	}
	if o.MaxSizeMB < 1 || o.MaxSizeMB > MaxFileFieldSizeMB {
		return invalidInputf(CodeFormFieldFileOptionsInvalid, "file_options.max_size_mb", "file field %q: max_size_mb must be between 1 and %d", f.Key, MaxFileFieldSizeMB)
	}
	seen := make(map[string]bool, len(o.AllowedTypes))
	normalized := make([]string, 0, len(o.AllowedTypes))
	for _, t := range o.AllowedTypes {
		t = strings.ToLower(strings.TrimSpace(t))
		if _, ok := FileTypeContentTypes[t]; !ok {
			return invalidInputf(CodeFormFieldFileOptionsInvalid, "file_options.allowed_types", "file field %q: unknown file type %q", f.Key, t)
		}
		if !seen[t] {
			seen[t] = true
			normalized = append(normalized, t)
		}
	}
	o.AllowedTypes = normalized
	return nil
}

// FileAnswer is one item of a file field's answer. A parent sends Token for
// a fresh upload and UploadID for an upload the request already holds; the
// stored answer carries UploadID only, never a token.
type FileAnswer struct {
	UploadID    string `json:"upload_id,omitempty"`
	Token       string `json:"token,omitempty"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

// FileFieldKeys returns the keys of the schema's file fields.
func FileFieldKeys(schema *FormSchema) []string {
	if schema == nil {
		return nil
	}
	var keys []string
	for i := range schema.Fields {
		if schema.Fields[i].Type == FormFieldFile {
			keys = append(keys, schema.Fields[i].Key)
		}
	}
	return keys
}

// FileFieldByKey returns the schema's file field with key, or nil.
func FileFieldByKey(schema *FormSchema, key string) *FormField {
	if schema == nil {
		return nil
	}
	for i := range schema.Fields {
		if schema.Fields[i].Key == key && schema.Fields[i].Type == FormFieldFile {
			return &schema.Fields[i]
		}
	}
	return nil
}
