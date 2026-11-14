package calendar

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// fileDateTime is a date with a time of day after it, as OGS Connect
// writes "31.12.2024 14:05" (#7009); the time is dropped.
var fileDateTime = regexp.MustCompile(`^(\S+)[ T]\d{1,2}:\d{2}(:\d{2})?$`)

// ParseFileDate reads a date as an import file writes it: German
// (31.12.2024, 1.2.2024, 31/12/2024) or ISO (2024-12-31), also with a time
// of day after it. A two-digit year is ambiguous and refused.
func ParseFileDate(value string) (Date, bool) {
	if match := fileDateTime.FindStringSubmatch(value); match != nil {
		value = match[1]
	}
	if date, err := ParseDate(value); err == nil {
		return date, true
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '/' })
	if len(parts) != 3 || len(parts[2]) != 4 {
		return "", false
	}
	day, errDay := strconv.Atoi(parts[0])
	month, errMonth := strconv.Atoi(parts[1])
	year, errYear := strconv.Atoi(parts[2])
	if errDay != nil || errMonth != nil || errYear != nil {
		return "", false
	}
	date, err := ParseDate(fmt.Sprintf("%04d-%02d-%02d", year, month, day))
	if err != nil {
		return "", false
	}
	return date, true
}
