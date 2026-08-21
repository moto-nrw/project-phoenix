package activities

import "testing"

func TestGroupValidateOfferingSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   Group
		wantErr string
	}{
		{
			name: "angebot with source and filter is valid",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
				SourceGradeLevels:     []int{1, 2},
			},
		},
		{
			name: "angebot with source and empty filter is valid",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
			},
		},
		{
			name: "legacy angebot without source stays valid",
			group: Group{
				TargetGroupType: TargetGroupTypeAngebot,
			},
		},
		{
			name: "grade filter without source is rejected",
			group: Group{
				TargetGroupType:   TargetGroupTypeAngebot,
				SourceGradeLevels: []int{1},
			},
			wantErr: "source_grade_levels requires source_care_offering_ids",
		},
		{
			name: "source on non-angebot type is rejected",
			group: Group{
				TargetGroupType:       TargetGroupTypeJahrgang,
				TargetGradeLevel:      int16Ptr(2),
				SourceCareOfferingIDs: []int64{5},
			},
			wantErr: "source_care_offering_ids requires target group type 'angebot'",
		},
		{
			name: "non-positive source id is rejected",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{0},
			},
			wantErr: "source_care_offering_ids entries must be positive",
		},
		{
			name: "duplicate source ids are rejected",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5, 5},
			},
			wantErr: "source_care_offering_ids must not contain duplicates",
		},
		{
			name: "several distinct sources are valid",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5, 8, 9},
				SourceGradeLevels:     []int{1},
			},
		},
		{
			name: "out-of-range grade is rejected",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
				SourceGradeLevels:     []int{14},
			},
			wantErr: "source_grade_levels entries must be between 1 and 13",
		},
		{
			name: "duplicate grades are rejected",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
				SourceGradeLevels:     []int{2, 2},
			},
			wantErr: "source_grade_levels must not contain duplicates",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.group.ValidateTargetGroup()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("expected error %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestGroupValidateOfferingSourceNormalizesEmptyFilter(t *testing.T) {
	t.Parallel()

	group := Group{
		TargetGroupType:       TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{5},
		SourceGradeLevels:     []int{},
	}
	if err := group.ValidateTargetGroup(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.SourceGradeLevels != nil {
		t.Fatalf("empty filter must normalize to nil, got %v", group.SourceGradeLevels)
	}
}

func TestGroupMatchesSourceGradeFilter(t *testing.T) {
	t.Parallel()

	unfiltered := Group{TargetGroupType: TargetGroupTypeAngebot, SourceCareOfferingIDs: []int64{5}}
	if !unfiltered.MatchesSourceGradeFilter(nil) || !unfiltered.MatchesSourceGradeFilter(int16Ptr(3)) {
		t.Fatal("empty filter must admit every child")
	}

	filtered := Group{
		TargetGroupType:       TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{5},
		SourceGradeLevels:     []int{1, 2},
	}
	if !filtered.MatchesSourceGradeFilter(int16Ptr(2)) {
		t.Fatal("grade 2 must match filter [1 2]")
	}
	if filtered.MatchesSourceGradeFilter(int16Ptr(3)) {
		t.Fatal("grade 3 must not match filter [1 2]")
	}
	if filtered.MatchesSourceGradeFilter(nil) {
		t.Fatal("a child without derivable grade must not match a set filter")
	}
}

// The template list read model carries the jsonb grade filter as text
// (ad-hoc scan struct, no bun type mapping) — decoding it is the only place
// the filter turns back into the list the API serializes (#2137).
func TestTemplateListRowParseSourceGradeLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "NULL column yields no filter", raw: ""},
		{name: "empty array yields no filter", raw: "[]"},
		{name: "grade list is decoded", raw: "[1,2]", want: []int{1, 2}},
		{name: "malformed json is reported", raw: "{oops", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			levels, err := TemplateListRow{SourceGradeLevelsJSON: tc.raw}.ParseSourceGradeLevels()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected a parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(levels) != len(tc.want) {
				t.Fatalf("got %v, want %v", levels, tc.want)
			}
			for i, level := range levels {
				if level != tc.want[i] {
					t.Fatalf("got %v, want %v", levels, tc.want)
				}
			}
		})
	}
}

// The list read model also carries the source id array as jsonb text
// (multi-source follow-up to #2137).
func TestTemplateListRowParseSourceCareOfferingIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []int64
		wantErr bool
	}{
		{name: "NULL column yields no source", raw: ""},
		{name: "empty array yields no source", raw: "[]"},
		{name: "id list is decoded", raw: "[12,15]", want: []int64{12, 15}},
		{name: "malformed json is reported", raw: "{oops", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := TemplateListRow{SourceCareOfferingIDsJSON: tc.raw}.ParseSourceCareOfferingIDs()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected a parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ids) != len(tc.want) {
				t.Fatalf("got %v, want %v", ids, tc.want)
			}
			for i, id := range ids {
				if id != tc.want[i] {
					t.Fatalf("got %v, want %v", ids, tc.want)
				}
			}
		})
	}
}

// #2482: an offering-sourced Regeltermin may additionally be narrowed to
// concrete Schulklassen. Class and Jahrgang filter are mutually exclusive —
// a class already implies its grade, so the combination is either redundant
// or empty.
func TestGroupValidateSourceSchoolClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   Group
		wantErr string
	}{
		{
			name: "angebot with source and class filter is valid",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
				SourceSchoolClasses:   []string{"1a", "1b"},
			},
		},
		{
			name: "class filter without source is rejected",
			group: Group{
				TargetGroupType:     TargetGroupTypeAngebot,
				SourceSchoolClasses: []string{"1a"},
			},
			wantErr: "source_school_classes requires source_care_offering_ids",
		},
		{
			name: "class and grade filter together are rejected",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
				SourceGradeLevels:     []int{1},
				SourceSchoolClasses:   []string{"1a"},
			},
			wantErr: "source_school_classes and source_grade_levels cannot be combined",
		},
		{
			name: "blank class entry is rejected",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
				SourceSchoolClasses:   []string{"  "},
			},
			wantErr: "source_school_classes entries must not be empty",
		},
		{
			name: "classes differing only in case or padding are duplicates",
			group: Group{
				TargetGroupType:       TargetGroupTypeAngebot,
				SourceCareOfferingIDs: []int64{5},
				SourceSchoolClasses:   []string{"1a", " 1A "},
			},
			wantErr: "source_school_classes must not contain duplicates",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.group.ValidateTargetGroup()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("expected error %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestGroupValidateSourceSchoolClassesNormalizes(t *testing.T) {
	t.Parallel()

	group := Group{
		TargetGroupType:       TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{5},
		SourceSchoolClasses:   []string{" 1b ", "2A"},
	}
	if err := group.ValidateTargetGroup(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Stored trimmed but case-preserving: the school reads its own spelling
	// back in the editor; matching is case-insensitive at compare time.
	if len(group.SourceSchoolClasses) != 2 ||
		group.SourceSchoolClasses[0] != "1b" || group.SourceSchoolClasses[1] != "2A" {
		t.Fatalf("expected trimmed classes, got %#v", group.SourceSchoolClasses)
	}

	empty := Group{
		TargetGroupType:       TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{5},
		SourceSchoolClasses:   []string{},
	}
	if err := empty.ValidateTargetGroup(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if empty.SourceSchoolClasses != nil {
		t.Fatalf("empty filter must normalize to nil, got %v", empty.SourceSchoolClasses)
	}
}

func TestSourceClassFilterMatches(t *testing.T) {
	t.Parallel()

	if !SourceClassFilterMatches(nil, "") || !SourceClassFilterMatches(nil, "3a") {
		t.Fatal("empty filter must admit every child")
	}
	if !SourceClassFilterMatches([]string{"1b"}, " 1B ") {
		t.Fatal("class matching must ignore case and padding")
	}
	if SourceClassFilterMatches([]string{"1b"}, "1a") {
		t.Fatal("class 1a must not match filter [1b]")
	}
	if SourceClassFilterMatches([]string{"1b"}, "") {
		t.Fatal("a child without a school class must not match a set filter")
	}
}

func TestTemplateListRowParseSourceSchoolClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "NULL column yields no filter", raw: ""},
		{name: "empty array yields no filter", raw: "[]"},
		{name: "class list is decoded", raw: `["1a","1b"]`, want: []string{"1a", "1b"}},
		{name: "malformed json is reported", raw: "{oops", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			classes, err := TemplateListRow{SourceSchoolClassesJSON: tc.raw}.ParseSourceSchoolClasses()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected a parse error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(classes) != len(tc.want) {
				t.Fatalf("got %v, want %v", classes, tc.want)
			}
			for i, class := range classes {
				if class != tc.want[i] {
					t.Fatalf("got %v, want %v", classes, tc.want)
				}
			}
		})
	}
}
