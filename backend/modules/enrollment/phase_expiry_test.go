package enrollment

import (
	"context"
	"testing"
)

func TestPhaseExpiryRejectsInvalidReportDatesBeforeTransaction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input PhaseExpiryInput
		want  string
	}{
		{"missing report date", PhaseExpiryInput{WarningThrough: "2026-09-05"}, "phase expiry report dates are required"},
		{"missing horizon", PhaseExpiryInput{AsOf: "2026-09-05"}, "phase expiry report dates are required"},
		{"reversed horizon", PhaseExpiryInput{AsOf: "2026-09-05", WarningThrough: "2026-09-04"}, "phase expiry warning horizon must not be before the report date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// No transaction runner: invalid input must fail before persistence.
			module := &Module{}
			rows, err := module.PhaseExpirySnapshots(context.Background(), tt.input)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("got error %v, want %q", err, tt.want)
			}
			if rows != nil {
				t.Fatalf("invalid input returned rows: %v", rows)
			}
		})
	}
}
