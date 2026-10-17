package staffmessaging

import "context"

// Runtime supplies the ambient school and commit boundary; the root binds it.
type Runtime interface {
	RegisterAfterCommit(context.Context, func())
}
