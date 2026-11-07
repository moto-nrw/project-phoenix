package enrollment

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractAcceptancesRequireUnchangedParentEvidenceBeforeApproval(t *testing.T) {
	t.Parallel()
	submitted := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	accepted := submitted.Add(time.Hour)
	text := "Der Vertrag beginnt mit der Annahme."
	fingerprint := func(content []byte) string {
		if string(content) == text {
			return "known-text-fingerprint"
		}
		return "different-text-fingerprint"
	}
	entry := LegalBlocksSnapshotEntry{SnapshotAt: submitted, ConsentFlags: json.RawMessage(`{"contract":true}`),
		FormSchemaID: 81, DisplayedFormSchemaID: 81,
		Blocks: []LegalBlockSnapshot{{Key: "contract", Kind: LegalBlockKindTerms, Required: true, Source: LegalBlockSourceCustom,
			Text: text, TextSHA256: "known-text-fingerprint"}}}
	tests := map[string]func(*LegalBlocksSnapshotEntry){
		"valid":                             func(*LegalBlocksSnapshotEntry) {},
		"optional block the parents ticked": func(e *LegalBlocksSnapshotEntry) { e.Blocks[0].Required = false },
		"optional consent block ticked": func(e *LegalBlocksSnapshotEntry) {
			e.Blocks[0].Required, e.Blocks[0].Kind = false, LegalBlockKindConsent
		},
		"old snapshot without checksum":      func(e *LegalBlocksSnapshotEntry) { e.Blocks[0].TextSHA256 = "" },
		"form showed an older version":       func(e *LegalBlocksSnapshotEntry) { e.DisplayedFormSchemaID = 80 },
		"form reported no version":           func(e *LegalBlocksSnapshotEntry) { e.DisplayedFormSchemaID = 0 },
		"standard text is no own consent":    func(e *LegalBlocksSnapshotEntry) { e.Blocks[0].Source = LegalBlockSourceStandard },
		"altered wording":                    func(e *LegalBlocksSnapshotEntry) { e.Blocks[0].Text += " Neue Bedingungen." },
		"no consent":                         func(e *LegalBlocksSnapshotEntry) { e.ConsentFlags = json.RawMessage(`{"contract":false}`) },
		"privacy is not agreement":           func(e *LegalBlocksSnapshotEntry) { e.Blocks[0].Kind = LegalBlockKindPrivacyNotice },
		"submission after approval":          func(e *LegalBlocksSnapshotEntry) { e.SnapshotAt = accepted.Add(time.Hour) },
		"linked document is not frozen text": func(e *LegalBlocksSnapshotEntry) { e.Blocks[0].Attachments = []FormAttachment{{}} },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := entry
			candidate.Blocks = append([]LegalBlockSnapshot(nil), entry.Blocks...)
			change(&candidate)
			raw, err := json.Marshal([]LegalBlocksSnapshotEntry{candidate})
			require.NoError(t, err)
			found := ContractAcceptances(raw, RequestSourcePublic, &accepted, fingerprint)
			switch name {
			case "valid":
				require.Len(t, found, 1)
				assert.Equal(t, text, found[0].Text)
				assert.Equal(t, submitted, found[0].SubmittedAt)
				assert.True(t, found[0].Required)
			case "optional block the parents ticked", "optional consent block ticked":
				require.Len(t, found, 1, "a ticked wish is evidence too (#4682)")
				assert.False(t, found[0].Required, "but never the required conditions")
			default:
				assert.Empty(t, found)
			}
			assert.Empty(t, ContractAcceptances(raw, RequestSourceAdminManual, &accepted, fingerprint))
			assert.Empty(t, ContractAcceptances(raw, RequestSourcePublic, nil, fingerprint))
		})
	}
}
