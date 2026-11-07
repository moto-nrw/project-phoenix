package enrollment

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ContractApplication is an accepted application child of one school with
// the guardian who applied, as the Betreuungsvertrag of the Träger turns it
// into a contract (#4678, ADR 0153). It names no booking: Care Plan keeps
// those.
type ContractApplication struct {
	AcceptedAt        *time.Time
	Acceptances       []ContractAcceptance
	RequestChildID    int64
	StudentID         *int64
	ChildFirstName    string
	ChildLastName     string
	ChildBirthDate    *calendar.Date
	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	GuardianAccountID *int64
	PhaseID           int64
	PhaseName         string
	ServiceStartDate  *calendar.Date
	ActivateOn        *calendar.Date
}

// ContractPhase is an offering phase's name and inclusive service window.
type ContractPhase struct {
	Name         string
	ServiceStart *calendar.Date
	ServiceEnd   *calendar.Date
}

// ContractApplications reads the accepted application children of the
// school whose tenant transaction the caller holds. A non-empty
// requestChildIDs keeps only those. It reads nothing else.
type ContractApplications interface {
	AcceptedContractApplications(ctx context.Context, requestChildIDs []int64) ([]ContractApplication, error)
	// ContractPhases reads phase names and service bounds by ID.
	ContractPhases(ctx context.Context, phaseIDs []int64) (map[int64]ContractPhase, error)
}

// ContractAcceptance is the consent to an unchanged, checksummed terms block
// shown at a parent submission. Legacy snapshots without a checksum are not
// evidence for concluding a contract retrospectively.
type ContractAcceptance struct {
	Key         string
	Text        string
	SHA256      string
	SubmittedAt time.Time
	// Required tells that the form demanded the tick. The conditions of a
	// contract count only as a required block; an optional block the
	// parents ticked records a wish, such as the early start of care within
	// the withdrawal period (#4682).
	Required bool
}

// ContractAcceptances reads only the latest parent submission. A submission
// after the decision cannot establish what was accepted at that decision.
func ContractAcceptances(raw json.RawMessage, source string, acceptedAt *time.Time, fingerprint func([]byte) string) []ContractAcceptance {
	if fingerprint == nil || acceptedAt == nil || acceptedAt.IsZero() || (source != RequestSourcePublic && source != RequestSourceLateInvite) {
		return nil
	}
	var entries []LegalBlocksSnapshotEntry
	if json.Unmarshal(raw, &entries) != nil || len(entries) == 0 {
		return nil
	}
	entry := entries[len(entries)-1]
	// The parent's form must have shown the version whose texts the entry
	// holds; a form opened before the school published a new version proves
	// no agreement to the new wording (#4680).
	if entry.SnapshotAt.IsZero() || entry.SnapshotAt.After(*acceptedAt) ||
		entry.FormSchemaID == 0 || entry.DisplayedFormSchemaID != entry.FormSchemaID {
		return nil
	}
	var flags map[string]bool
	if json.Unmarshal(entry.ConsentFlags, &flags) != nil {
		return nil
	}
	var result []ContractAcceptance
	for _, block := range entry.Blocks {
		if proof, ok := contractAcceptance(block, flags, entry.SnapshotAt, fingerprint); ok {
			result = append(result, proof)
		}
	}
	return result
}

func contractAcceptance(block LegalBlockSnapshot, flags map[string]bool, at time.Time, fingerprint func([]byte) string) (ContractAcceptance, bool) {
	if block.Kind != LegalBlockKindTerms || block.Source != LegalBlockSourceCustom || !flags[block.Key] || block.Text == "" || len(block.Attachments) > 0 {
		return ContractAcceptance{}, false
	}
	checksum := fingerprint([]byte(block.Text))
	if block.TextSHA256 == "" || block.TextSHA256 != checksum {
		return ContractAcceptance{}, false
	}
	return ContractAcceptance{Key: block.Key, Text: block.Text, SHA256: checksum, SubmittedAt: at, Required: block.Required}, true
}
