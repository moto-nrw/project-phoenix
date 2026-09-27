package parent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The portal names who settled a child under "any" together with the date,
// so the other signer's submission time must reach the wire.
func TestDeclarationResponseCarriesOtherSignerSubmittedAt(t *testing.T) {
	agreed := usersModels.DeclarationActionAgreed
	at := time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC)
	out := toDeclarationResponse(&usersModels.AnnouncementFeedDeclaration{
		Children: []*usersModels.AnnouncementFeedDeclarationChild{{
			StudentID: 7,
			OtherSigners: []usersModels.DeclarationSignerState{
				{FirstName: "Sabine", LastName: "Muster", Action: &agreed, SubmittedAt: &at},
				{FirstName: "Tom", LastName: "Muster"},
			},
		}},
	})

	raw, err := json.Marshal(out.Children[0].OtherSigners)
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"first_name":"Sabine","last_name":"Muster","action":"agreed","submitted_at":"2026-09-20T08:30:00Z"},
		{"first_name":"Tom","last_name":"Muster","action":null,"submitted_at":null}
	]`, string(raw))
}
