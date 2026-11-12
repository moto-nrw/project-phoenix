package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// TestTraegerNameFallsBackToThePartnerWithoutALetterhead: a contract is sent
// with a partner but no letterhead; its mails must still name the Träger.
func TestTraegerNameFallsBackToThePartnerWithoutALetterhead(t *testing.T) {
	t.Parallel()
	terms := carecontract.TemplateInput{ContractPartner: "Ganztag im Quartier e.V."}
	assert.Equal(t, "Ganztag im Quartier e.V.", traegerName("  \n", terms))
	assert.Equal(t, "Träger aus dem Briefkopf", traegerName("Träger aus dem Briefkopf\nAm Markt 4", terms))
}
