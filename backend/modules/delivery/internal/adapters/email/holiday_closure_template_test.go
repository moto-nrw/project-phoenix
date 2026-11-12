package email

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReopeningInstructionsRespectEachChildsPlace(t *testing.T) {
	t.Parallel()
	templates, err := parseTemplates(filepath.Join("..", "..", "..", "..", "..", "templates", "email"))
	require.NoError(t, err)
	children := []map[string]any{}
	for _, child := range []struct {
		name             string
		waiting, pending bool
	}{{"Lara", false, false}, {"Tim", true, false}, {"Mia", false, true}} {
		children = append(children, map[string]any{"Name": child.name, "Weeks": []map[string]any{{"Week": "Sommerferien", "ClosedDays": []string{"Montag"}, "Waitlisted": child.waiting, "Pending": child.pending}}})
	}
	var body bytes.Buffer
	require.NoError(t, templates.ExecuteTemplate(&body, "holiday-closure-notice.html", map[string]any{"ReopenedChildren": children}))
	assert.Contains(t, body.String(), "An diesen Tagen können Sie Lara wie gebucht bringen.")
	assert.Contains(t, body.String(), "Tim bleibt auf der Warteliste und hat noch keinen Platz.")
	assert.Contains(t, body.String(), "Das Büro entscheidet noch über den Platz für Mia.")
	assert.NotContains(t, body.String(), "An den geöffneten Tagen können Sie Ihr Kind wie gebucht bringen.")
}
