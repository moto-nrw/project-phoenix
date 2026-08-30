package architecture

import (
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceLocationRejectsUnavailableAndExternalPositions(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	tests := []struct {
		name     string
		position token.Position
		want     string
	}{
		{name: "unavailable", position: token.Position{}, want: "source position is unavailable"},
		{name: "outside project", position: token.Position{Filename: filepath.Join(filepath.Dir(project), "outside.go"), Line: 1}, want: "source path is outside project"},
	}
	for _, test := range tests {
		_, err := sourceLocation(project, test.position, "declaration")
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s error = %v, want %q", test.name, err, test.want)
		}
	}
}
