package settings

// IntRange creates a validation schema for integer range
func IntRange(min, max int64) *ValidationSchema {
	return &ValidationSchema{
		Min: &min,
		Max: &max,
	}
}

// IntMin creates a validation schema for minimum integer value
func IntMin(min int64) *ValidationSchema {
	return &ValidationSchema{
		Min: &min,
	}
}

// IntMax creates a validation schema for maximum integer value
func IntMax(max int64) *ValidationSchema {
	return &ValidationSchema{
		Max: &max,
	}
}

// StringLength creates a validation schema for string length
func StringLength(minLen, maxLen int) *ValidationSchema {
	return &ValidationSchema{
		MinLength: &minLen,
		MaxLength: &maxLen,
	}
}

// StringMinLength creates a validation schema for minimum string length
func StringMinLength(minLen int) *ValidationSchema {
	return &ValidationSchema{
		MinLength: &minLen,
	}
}

// StringMaxLength creates a validation schema for maximum string length
func StringMaxLength(maxLen int) *ValidationSchema {
	return &ValidationSchema{
		MaxLength: &maxLen,
	}
}

// Pattern creates a validation schema with a regex pattern
func Pattern(pattern string) *ValidationSchema {
	return &ValidationSchema{
		Pattern: &pattern,
	}
}

// Required creates a validation schema marking the field as required
func Required() *ValidationSchema {
	return &ValidationSchema{
		Required: true,
	}
}

// Combine merges multiple validation schemas into one
func Combine(schemas ...*ValidationSchema) *ValidationSchema {
	result := &ValidationSchema{}
	for _, s := range schemas {
		if s == nil {
			continue
		}
		if s.Min != nil {
			result.Min = s.Min
		}
		if s.Max != nil {
			result.Max = s.Max
		}
		if s.MinLength != nil {
			result.MinLength = s.MinLength
		}
		if s.MaxLength != nil {
			result.MaxLength = s.MaxLength
		}
		if s.Pattern != nil {
			result.Pattern = s.Pattern
		}
		if s.Required {
			result.Required = true
		}
	}
	return result
}
