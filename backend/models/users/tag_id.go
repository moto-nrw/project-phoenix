package users

import "github.com/moto-nrw/project-phoenix/models/auth"

// NormalizeTagID normalizes an RFID tag ID the way identity-access does
// (#2662): trimmed, separators removed, upper case. Person.TagID and the
// kiosk handlers compare tags in this form.
func NormalizeTagID(tagID string) string {
	return auth.NormalizeTagID(tagID)
}

// ValidateTagID reports why a tag cannot be an RFID card identifier, with
// the card owner's rule (length and hexadecimal form after normalization).
func ValidateTagID(tagID string) error {
	candidate := &auth.RFIDCard{}
	candidate.ID = tagID
	return candidate.Validate()
}
