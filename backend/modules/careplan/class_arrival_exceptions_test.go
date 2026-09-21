package careplan

import "testing"

func TestClassArrivalExceptionLabelPreservesClassAndReason(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		class  string
		reason string
		want   string
	}{
		{" 3a ", "", "Klasse 3a: andere Ankunftszeit"},
		{" Klasse 3a ", "", "Klasse 3a: andere Ankunftszeit"},
		{" kLaSsE 3a ", " Unterricht fällt aus ", "Klasse 3a: Unterricht fällt aus"},
		{"3a", "  ", "Klasse 3a: andere Ankunftszeit"},
	} {
		t.Run(tc.class+tc.reason, func(t *testing.T) {
			row := ClassArrivalException{SchoolClass: tc.class, Reason: &tc.reason}
			if got := row.Label(); got != tc.want {
				t.Errorf("Label() = %q, want %q", got, tc.want)
			}
		})
	}
	row := ClassArrivalException{SchoolClass: "3a"}
	if got := row.Label(); got != "Klasse 3a: andere Ankunftszeit" {
		t.Errorf("nil reason: Label() = %q", got)
	}
}
