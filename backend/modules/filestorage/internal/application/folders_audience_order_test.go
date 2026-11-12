package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/filestorage"
	"github.com/moto-ogs/project-phoenix/modules/filestorage/internal/ports"
)

type audienceIdentity struct {
	ports.Identity
	roles []filestorage.AudienceRole
	ids   []int64
}

func (a audienceIdentity) ListShareableRoles(context.Context) ([]filestorage.AudienceRole, error) {
	return a.roles, nil
}

func (a audienceIdentity) ListActiveSchoolAccountIDs(context.Context) ([]int64, error) {
	return a.ids, nil
}

type audiencePeople map[int64]ports.PersonName

func (p audiencePeople) PersonNames(context.Context, []int64) (map[int64]ports.PersonName, error) {
	return p, nil
}

// The share picker lists roles and people in German dictionary order: an
// accent or a lowercase initial no longer lands behind Z (#4949).
func TestAudience_UsesGermanDictionaryOrder(t *testing.T) {
	t.Parallel()
	service := &Service{deps: Dependencies{
		Identity: audienceIdentity{
			roles: []filestorage.AudienceRole{{ID: 1, Name: "Zivildienst"}, {ID: 2, Name: "Érzieher"}, {ID: 3, Name: "betreuung"}},
			ids:   []int64{11, 12, 13, 14},
		},
		People: audiencePeople{
			11: {FirstName: "Eva", LastName: "Zimmermann"},
			12: {FirstName: "Can", LastName: "Özdemir"},
			13: {FirstName: "Ana", LastName: "Ábel"},
			14: {FirstName: "Ida", LastName: "adler"},
		},
	}}

	roles, err := service.audienceRoles(context.Background())
	require.NoError(t, err)
	roleNames := make([]string, len(roles))
	for i, role := range roles {
		roleNames[i] = role.Name
	}
	assert.Equal(t, []string{"betreuung", "Érzieher", "Zivildienst"}, roleNames)

	accounts, err := service.audienceAccounts(context.Background())
	require.NoError(t, err)
	ids := make([]int64, len(accounts))
	for i, account := range accounts {
		ids[i] = account.AccountID
	}
	assert.Equal(t, []int64{13, 14, 12, 11}, ids)
}
