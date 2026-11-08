package enrollment

import "context"

// RequestFormLabels name a request's answers with the questions the parent
// answered (#6142). Fields come from the form version the request is pinned
// to, empty when it has none. LegalBlocks are the consents the request is
// validated against, followed by the pinned version's switched-off blocks,
// so a consent stored before the school switched it off keeps its title.
type RequestFormLabels struct {
	Fields      []FormField
	LegalBlocks []LegalBlock
}

// IntakeFormLabels reads the labels of a request. A change request compares
// answers of the same request, and the change form renders the same pinned
// version, so the labels stay those of the snapshot even after the school
// edits its form.
type IntakeFormLabels interface {
	// FormLabels loads the labels of a request of the school in context.
	// The caller runs it in its tenant transaction; a request of another
	// school answers ErrRequestNotFound.
	FormLabels(ctx context.Context, requestID int64) (*RequestFormLabels, error)
	// FormLabelsByStatusToken loads the labels of the request behind a valid
	// status token. The caller runs the lookup in an administrative
	// transaction; every later read is scoped to the request's school.
	FormLabelsByStatusToken(ctx context.Context, token string) (*RequestFormLabels, error)
}
