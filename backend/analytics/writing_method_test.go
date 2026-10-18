package analytics

import (
	"net/http"
	"testing"
)

func TestIsWritingMethod(t *testing.T) {
	t.Parallel()

	for method, want := range map[string]bool{
		http.MethodGet: false, http.MethodHead: false, http.MethodOptions: false,
		http.MethodPost: true, http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true,
	} {
		if got := IsWritingMethod(method); got != want {
			t.Errorf("IsWritingMethod(%q) = %v, want %v", method, got, want)
		}
	}
}
