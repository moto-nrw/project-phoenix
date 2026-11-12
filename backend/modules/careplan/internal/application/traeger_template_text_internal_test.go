package application

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

func TestTraegerTemplateTextLimitsCountUnicodeCharacters(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"note", "title", "url", "partner"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			template := careplan.TraegerContractTemplate{TermKind: "school_year", NoticeTo: "month_end"}
			set := func(extra string) {
				switch field {
				case "note":
					template.SpecialTerminationNote = strings.Repeat("a", 900) + strings.Repeat("ü", 100) + extra
				case "title":
					template.StatuteTitle = strings.Repeat("a", 180) + strings.Repeat("ä", 20) + extra
				case "url":
					prefix := "https://example.test/"
					template.StatuteURL = prefix + strings.Repeat("a", 480-len(prefix)) + strings.Repeat("ö", 20) + extra
				case "partner":
					template.ContractPartner = strings.Repeat("a", 280) + strings.Repeat("ö", 20) + extra
				}
			}
			set("")
			require.NoError(t, validateTraegerContractTemplate(template), "Care Contract accepts these character counts")
			set("ü")
			require.ErrorIs(t, validateTraegerContractTemplate(template), careplan.ErrCareOfferingConfigInvalid)
		})
	}
}
