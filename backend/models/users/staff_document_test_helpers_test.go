package users

import "errors"

// IsValidStaffDocumentCategory reports whether v is a known category value.
func IsValidStaffDocumentCategory(v string) bool {
	for _, c := range StaffDocumentCategories {
		if c == v {
			return true
		}
	}
	return false
}

// Validate ensures the document row is storable.
func (d *StaffDocument) Validate() error {
	if d.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if !IsValidStaffDocumentCategory(d.Category) {
		return errors.New("unknown document category")
	}
	if d.FilenameDisplay == "" {
		return errors.New("filename_display is required")
	}
	if d.FilenameStored == "" {
		return errors.New("filename_stored is required")
	}
	if d.SizeBytes < 0 {
		return errors.New("size_bytes must not be negative")
	}
	if d.ContentType == "" {
		return errors.New("content_type is required")
	}
	if d.UploadedBy <= 0 {
		return errors.New("uploaded_by is required")
	}
	return nil
}
