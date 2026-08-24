package students

import (
	"reflect"
	"testing"
)

func TestMatchesGradeLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		schoolClass string
		gradeLevels []int
		want        bool
	}{
		{"filter off (no grade levels) matches everything", "3a", nil, true},
		{"exact grade match", "3a", []int{3}, true},
		{"different grade does not match", "4a", []int{3}, false},
		{"leading-digit boundary: 13a must not match grade 1", "13a", []int{1}, false},
		{"double-digit grade matches", "13a", []int{13}, true},
		{"display label with grade matches", "Klasse 3a", []int{3}, true},
		{"class with no numeric prefix never matches a set grade", "Bienen", []int{3}, false},
		// #2218: two grades supervised together must both land in one list.
		{"first of several grades matches", "3a", []int{3, 4}, true},
		{"second of several grades matches", "4b", []int{3, 4}, true},
		{"grade outside the selection does not match", "2c", []int{3, 4}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesGradeLevel(tt.schoolClass, tt.gradeLevels); got != tt.want {
				t.Errorf("matchesGradeLevel(%q, %v) = %v, want %v", tt.schoolClass, tt.gradeLevels, got, tt.want)
			}
		})
	}
}

// Multi-value filters travel comma-separated, but a repeated parameter must
// mean the same thing so a hand-built URL is not silently truncated to its
// first value (#2218).
func TestParseMultiValueParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []string
		want []string
	}{
		{"absent parameter", nil, []string{}},
		{"single value", []string{"3a"}, []string{"3a"}},
		{"comma separated", []string{"3a,4b"}, []string{"3a", "4b"}},
		{"whitespace trimmed", []string{" 3a , 4b "}, []string{"3a", "4b"}},
		{"repeated parameter", []string{"3a", "4b"}, []string{"3a", "4b"}},
		{"blanks dropped", []string{"3a,,4b,"}, []string{"3a", "4b"}},
		{"duplicates collapsed", []string{"3a,3a"}, []string{"3a"}},
		{"empty value", []string{""}, []string{}},
		// school_class is free text, so a class may carry the separator itself.
		// The frontend escapes it, and only then is one class distinguishable
		// from two (#2218 review).
		{"escaped comma stays one value", []string{`A\,B`}, []string{"A,B"}},
		{"escaped and separating comma", []string{`A\,B,3a`}, []string{"A,B", "3a"}},
		{"escaped backslash", []string{`A\\B`}, []string{`A\B`}},
		{"backslash before separator", []string{`A\\,3a`}, []string{`A\`, "3a"}},
		{"trailing lone backslash dropped", []string{`3a\`}, []string{"3a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseMultiValueParam(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseMultiValueParam(%v) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

// An unusable id has always meant "no restriction" rather than a 400, and the
// multi-value form must not turn one bad entry into an empty list.
func TestParseGroupIDList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []string
		want []int64
	}{
		{"absent parameter", nil, []int64{}},
		{"single id", []string{"7"}, []int64{7}},
		{"comma separated ids", []string{"7,9"}, []int64{7, 9}},
		{"non-numeric entry skipped", []string{"7,abc,9"}, []int64{7, 9}},
		{"zero and negative skipped", []string{"0,-3,9"}, []int64{9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseGroupIDList(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseGroupIDList(%v) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseGradeLevelList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []string
		want []int
	}{
		{"absent parameter", nil, []int{}},
		{"single level", []string{"3"}, []int{3}},
		{"comma separated levels", []string{"3,4"}, []int{3, 4}},
		{"non-numeric entry skipped", []string{"3,x,4"}, []int{3, 4}},
		{"zero and negative skipped", []string{"0,-1,4"}, []int{4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseGradeLevelList(tt.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseGradeLevelList(%v) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
