package messaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Co-guardians appear in German dictionary order: an accent, an umlaut or a
// lowercase particle must not push a name behind Z (#4949).
func TestSortSharingRecipients_UsesGermanDictionaryOrder(t *testing.T) {
	t.Parallel()
	recipients := []RequestSharingRecipient{
		{LastName: "Zimmermann", FirstName: "Eva"},
		{LastName: "Özdemir", FirstName: "Can"},
		{LastName: "adler", FirstName: "Ida"},
		{LastName: "von Bergen", FirstName: "Lea"},
		{LastName: "Becker", FirstName: "Tom"},
		{LastName: "Ábel", FirstName: "Ana"},
	}
	sortSharingRecipients(recipients, nil)
	got := make([]string, len(recipients))
	for i, recipient := range recipients {
		got[i] = recipient.LastName
	}
	assert.Equal(t, []string{"Ábel", "adler", "Becker", "Özdemir", "von Bergen", "Zimmermann"}, got)
}
