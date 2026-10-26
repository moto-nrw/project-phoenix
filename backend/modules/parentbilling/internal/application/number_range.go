package application

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The pattern of a number range (spec #4611 user story 75, ADR 0119): text
// with the running number {N}…{NNNNNNNN}, padded with zeros to the count of
// N, and optionally the year of the issue date as {JJJJ} or {JJ}. A pattern
// with a year counts from 1 every year; one without counts on.

var (
	numberToken     = regexp.MustCompile(`\{N{1,8}\}`)
	yearToken       = regexp.MustCompile(`\{JJJJ\}|\{JJ\}`)
	patternLiterals = regexp.MustCompile(`^[A-Za-z0-9./_-]*$`)
)

// normalizeNumberPattern checks a pattern: one running number, at most one
// year, otherwise letters, digits and . / _ -.
func normalizeNumberPattern(pattern string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	errs := validation.Errors{}
	rest := yearToken.ReplaceAllString(numberToken.ReplaceAllString(pattern, ""), "")
	switch {
	case pattern == "":
		errs["pattern"] = errors.New("is required")
	case len([]rune(pattern)) > parentbilling.MaxNumberPatternLength:
		errs["pattern"] = errors.New("is too long")
	case len(numberToken.FindAllString(pattern, -1)) != 1:
		errs["pattern"] = errors.New("needs exactly one running number {N} to {NNNNNNNN}")
	case len(yearToken.FindAllString(pattern, -1)) > 1:
		errs["pattern"] = errors.New("may name the year once")
	case !patternLiterals.MatchString(rest):
		errs["pattern"] = errors.New("may only hold letters, digits, . / _ - and the placeholders")
	}
	if len(errs) > 0 {
		return pattern, errs
	}
	return pattern, nil
}

// numberYear is the counter of a pattern on the issue year: the year when
// the pattern names it, else 0 for one counter across the years.
func numberYear(pattern string, issueYear int) int {
	if yearToken.MatchString(pattern) {
		return issueYear
	}
	return 0
}

// formatInvoiceNumber writes the number sequence of year into pattern.
func formatInvoiceNumber(pattern string, year int, sequence int64) string {
	number := numberToken.ReplaceAllStringFunc(pattern, func(token string) string {
		return fmt.Sprintf("%0*d", len(token)-2, sequence)
	})
	return yearToken.ReplaceAllStringFunc(number, func(token string) string {
		if token == "{JJ}" {
			return fmt.Sprintf("%02d", year%100)
		}
		return fmt.Sprintf("%04d", year)
	})
}
