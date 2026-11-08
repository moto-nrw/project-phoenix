package owners

import (
	"context"
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// parentFamilyChildren binds the children of the family in the parents
// portal (#5918) to the Elternabrechnung, which stores the number and
// records every read and statement in the Träger transaction the flow
// opens, and translates its refusals into the flow's words.
type parentFamilyChildren struct {
	statements parentbilling.ParentFamilyChildrenStatement
}

// NewParentFamilyChildren binds the parents' statement to the
// Elternabrechnung.
func NewParentFamilyChildren() parentportal.FamilyChildrenStatementsPort {
	return parentFamilyChildren{statements: parentBillingCompose.NewParentFamilyChildren()}
}

func (p parentFamilyChildren) FamilyChildren(ctx context.Context, organizationID int64, reader parentportal.FamilyChildrenReader) (parentportal.FamilyChildren, error) {
	children, err := p.statements.ParentFamilyChildren(ctx, organizationID, parentbilling.ParentFamilyReader(reader))
	return parentFamilyChildrenView(children), parentFamilyChildrenError(err)
}

func (p parentFamilyChildren) StateFamilyChildren(ctx context.Context, organizationID int64, reader parentportal.FamilyChildrenReader, children int) (parentportal.FamilyChildren, error) {
	stated, err := p.statements.StateFamilyChildren(ctx, organizationID, parentbilling.ParentFamilyReader(reader), children)
	return parentFamilyChildrenView(stated), parentFamilyChildrenError(err)
}

func parentFamilyChildrenView(children parentbilling.ParentFamilyChildren) parentportal.FamilyChildren {
	return parentportal.FamilyChildren{StatedChildren: children.StatedChildren, StatedAt: children.StatedAt,
		ConfirmedChildren: children.ConfirmedChildren, ValidFrom: children.ValidFrom, ValidUntil: children.ValidUntil}
}

func parentFamilyChildrenError(err error) error {
	var fields validation.Errors
	switch {
	case err == nil:
		return nil
	case errors.As(err, &fields):
		return fmt.Errorf("%w: %w", parentportal.ErrFamilyChildrenInvalid, err)
	case errors.Is(err, parentbilling.ErrFamilyChildrenNotFound):
		return fmt.Errorf("%w: %w", parentportal.ErrBillingDocumentNotFound, err)
	}
	return err
}
