package owners

import (
	"context"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentHolidayWishes binds the parents portal's prefill to Enrollment's
// statement of the holiday care wish (#4703). It reads in the caller's
// tenant transaction of the child's school; the workflow checked the
// relationship right before.
func NewParentHolidayWishes(proofs enrollmentOwner.HolidayWishProofs) parentportal.HolidayWishStatements {
	return parentHolidayWishes{proofs: proofs}
}

type parentHolidayWishes struct {
	proofs enrollmentOwner.HolidayWishProofs
}

func (w parentHolidayWishes) HolidayWishStatement(ctx context.Context, claim parentportal.HolidayWishClaim) (string, string, error) {
	statement, err := w.proofs.HolidayWishProof(ctx, enrollmentOwner.HolidayWishClaim(claim))
	return statement.Wish, statement.Proof, err
}
