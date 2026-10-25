package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

// Erklärungen (#3430) as the guardian portal HTTP composition names them.
type (
	DeclarationInput  = messaging.DeclarationInput
	PasswordConfirmer = messaging.PasswordConfirmer
)

var (
	ErrDeclarationNotPermitted      = messaging.ErrDeclarationNotPermitted
	ErrDeclarationVersionChanged    = messaging.ErrDeclarationVersionChanged
	ErrDeclarationClosed            = messaging.ErrDeclarationClosed
	ErrDeclarationActionNotAllowed  = messaging.ErrDeclarationActionNotAllowed
	ErrDeclarationPasswordRequired  = messaging.ErrDeclarationPasswordRequired
	ErrDeclarationPasswordIncorrect = messaging.ErrDeclarationPasswordIncorrect
	ErrDeclarationSignatureRequired = messaging.ErrDeclarationSignatureRequired
	ErrDeclarationSignatureInvalid  = messaging.ErrDeclarationSignatureInvalid
)
