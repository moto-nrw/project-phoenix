package owners

import (
	"context"
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The children of the family in the parents portal (#5918): the inbox port
// of the Elternabrechnung reads and stores the number in the Träger
// transaction the flow opens and records every read and statement; these
// methods translate its values and refusals into the flow's words.

func (d parentBillingDocuments) FamilyChildren(ctx context.Context, organizationID int64, reader parentportal.FamilyChildrenReader) (parentportal.FamilyChildren, error) {
	children, err := d.documents.ParentFamilyChildren(ctx, organizationID, parentbilling.ParentFamilyReader(reader))
	return parentFamilyChildrenView(children), parentFamilyChildrenError(err)
}

func (d parentBillingDocuments) StateFamilyChildren(ctx context.Context, organizationID int64, reader parentportal.FamilyChildrenReader, children int) (parentportal.FamilyChildren, error) {
	stated, err := d.documents.StateFamilyChildren(ctx, organizationID, parentbilling.ParentFamilyReader(reader), children)
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
