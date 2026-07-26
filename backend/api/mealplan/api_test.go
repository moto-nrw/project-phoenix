package mealplan

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/render"
)

// TestSetDayRequest_Bind pins down the clear-vs-malformed contract at the HTTP
// binding boundary: an absent or null "dishes" field is a malformed request
// (400), while an explicit empty array is a valid clear-the-day operation.
// Without the pointer + Bind guard a "{}" body would decode to a nil slice and
// silently wipe the day's stored dishes.
func TestSetDayRequest_Bind(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantErr    bool
		wantDishes int // only checked when wantErr is false
	}{
		{name: "empty object rejected", body: `{}`, wantErr: true},
		{name: "null dishes rejected", body: `{"dishes":null}`, wantErr: true},
		{name: "empty array clears day", body: `{"dishes":[]}`, wantErr: false, wantDishes: 0},
		{name: "one dish accepted", body: `{"dishes":[{"dish":"Nudeln"}]}`, wantErr: false, wantDishes: 1},
		{
			name:       "dish with note accepted",
			body:       `{"dishes":[{"dish":"Menü 1","note":"vegetarisch"}]}`,
			wantErr:    false,
			wantDishes: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/2026-07-01", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")

			req := &SetDayRequest{}
			err := render.Bind(r, req)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected Bind to reject %q, got nil error", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("Bind rejected valid body %q: %v", tc.body, err)
			}
			if req.Dishes == nil {
				t.Fatalf("Dishes should be non-nil for %q", tc.body)
			}
			if len(*req.Dishes) != tc.wantDishes {
				t.Errorf("dishes count: got %d, want %d", len(*req.Dishes), tc.wantDishes)
			}
		})
	}
}
