package activities

import "testing"

func TestGroupValidateOfferingSource(t *testing.T) {
	tests := []struct {
		name    string
		group   Group
		wantErr string
	}{
		{
			name: "angebot with source and filter is valid",
			group: Group{
				TargetGroupType:      TargetGroupTypeAngebot,
				SourceCareOfferingID: int64Ptr(5),
				SourceGradeLevels:    []int{1, 2},
			},
		},
		{
			name: "angebot with source and empty filter is valid",
			group: Group{
				TargetGroupType:      TargetGroupTypeAngebot,
				SourceCareOfferingID: int64Ptr(5),
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
			wantErr: "source_grade_levels requires source_care_offering_id",
		},
		{
			name: "source on non-angebot type is rejected",
			group: Group{
				TargetGroupType:      TargetGroupTypeJahrgang,
				TargetGradeLevel:     int16Ptr(2),
				SourceCareOfferingID: int64Ptr(5),
			},
			wantErr: "source_care_offering_id requires target group type 'angebot'",
		},
		{
			name: "non-positive source id is rejected",
			group: Group{
				TargetGroupType:      TargetGroupTypeAngebot,
				SourceCareOfferingID: int64Ptr(0),
			},
			wantErr: "source_care_offering_id must be positive when set",
		},
		{
			name: "out-of-range grade is rejected",
			group: Group{
				TargetGroupType:      TargetGroupTypeAngebot,
				SourceCareOfferingID: int64Ptr(5),
				SourceGradeLevels:    []int{14},
			},
			wantErr: "source_grade_levels entries must be between 1 and 13",
		},
		{
			name: "duplicate grades are rejected",
			group: Group{
				TargetGroupType:      TargetGroupTypeAngebot,
				SourceCareOfferingID: int64Ptr(5),
				SourceGradeLevels:    []int{2, 2},
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
	group := Group{
		TargetGroupType:      TargetGroupTypeAngebot,
		SourceCareOfferingID: int64Ptr(5),
		SourceGradeLevels:    []int{},
	}
	if err := group.ValidateTargetGroup(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.SourceGradeLevels != nil {
		t.Fatalf("empty filter must normalize to nil, got %v", group.SourceGradeLevels)
	}
}

func TestGroupMatchesSourceGradeFilter(t *testing.T) {
	unfiltered := Group{TargetGroupType: TargetGroupTypeAngebot, SourceCareOfferingID: int64Ptr(5)}
	if !unfiltered.MatchesSourceGradeFilter(nil) || !unfiltered.MatchesSourceGradeFilter(int16Ptr(3)) {
		t.Fatal("empty filter must admit every child")
	}

	filtered := Group{
		TargetGroupType:      TargetGroupTypeAngebot,
		SourceCareOfferingID: int64Ptr(5),
		SourceGradeLevels:    []int{1, 2},
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
