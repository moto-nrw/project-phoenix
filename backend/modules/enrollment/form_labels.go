package enrollment

import "context"

// RequestFormLabels name a request's answers with the questions and consents
// the parent saw (#6142). Fields and legal blocks come from the latest
// submission snapshot when available; legacy requests fall back to their
// pinned form version and current legal texts.
type RequestFormLabels struct {
	Fields      []FormField
	LegalBlocks []LegalBlock
}

// IntakeFormLabels reads the labels of a request. A change request compares
// answers of the same request, so historical labels must remain stable after
// the school edits its form or legal texts.
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
