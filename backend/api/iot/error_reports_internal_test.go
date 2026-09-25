package iot

import (
	"testing"
	"time"
)

func TestErrorReportsLimiterBurstAndRefill(t *testing.T) {
	t.Parallel()

	reports := NewErrorReports(nil, ErrorReportsRuntime{}, nil)
	now := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	for i := range errorReportsPerMinute {
		if !reports.allowAt(1, now) {
			t.Fatalf("request %d of the initial burst was rejected", i+1)
		}
	}
	if reports.allowAt(1, now) {
		t.Fatal("request 61 at the same instant was allowed")
	}
	if !reports.allowAt(2, now) {
		t.Fatal("another device inherited the exhausted quota")
	}
	if !reports.allowAt(1, now.Add(time.Second)) {
		t.Fatal("one token was not refilled after one second")
	}
}
