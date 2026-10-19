package owners

// StringValue reads a nullable projection column; NULL reads as "".
func StringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
