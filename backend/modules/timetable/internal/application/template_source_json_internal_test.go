package application

import "testing"

func TestDecodeTemplateSourceGradeLevels(t *testing.T) {
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
			levels, err := decodeTemplateSource[int](tc.raw)
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

func TestDecodeTemplateSourceCareOfferingIDs(t *testing.T) {
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
			ids, err := decodeTemplateSource[int64](tc.raw)
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

func TestDecodeTemplateSourceSchoolClasses(t *testing.T) {
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
			classes, err := decodeTemplateSource[string](tc.raw)
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
