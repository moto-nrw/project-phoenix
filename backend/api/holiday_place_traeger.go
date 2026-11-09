package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// A cancellation, rebooking or new capacity can move a child up the waiting
// list of a holiday week (#4699); the mail to its parents names the Träger
// and the host school. The Träger directory reads in an administrative
// transaction, which cannot open inside the Träger transaction of the
// change. So a change of the Elternabrechnung's holiday routes reads the
// session's Träger first, before its Träger transaction, like the export
// reads the school names; the placement takes it from the request.

type holidayPlaceTraegerKey struct{}

type prefetchedHolidayTraeger struct {
	organizationID int64
	traeger        enrollmentOwner.HolidayTraeger
}

// errHolidayTraegerNotRead reports a move up the waiting list in a request
// that did not read its Träger first.
var errHolidayTraegerNotRead = errors.New("api: the Träger of the holiday change was not read before its transaction")

// holidayPlaceTraeger serves the Träger the request read before its Träger
// transaction.
type holidayPlaceTraeger struct{}

func (holidayPlaceTraeger) HolidayTraeger(ctx context.Context, organizationID int64) (enrollmentOwner.HolidayTraeger, error) {
	read, ok := ctx.Value(holidayPlaceTraegerKey{}).(prefetchedHolidayTraeger)
	if !ok || read.organizationID != organizationID {
		return enrollmentOwner.HolidayTraeger{}, errHolidayTraegerNotRead
	}
	return read.traeger, nil
}

// holidayPlaceRoutes are ProtectedTraegerRoutes whose changing requests read
// the session's Träger in an administrative transaction before their Träger
// transaction. A Träger that cannot be read is left out; a move up then
// fails and rolls the change back, so no child moves up unannounced.
func holidayPlaceRoutes(directory enrollmentOwner.HolidayTraegerDirectory) func(chi.Router, func(chi.Router, common.Middleware)) {
	return func(r chi.Router, fn func(chi.Router, common.Middleware)) {
		ProtectedTraegerRoutes(r, func(gr chi.Router, withTx common.Middleware) {
			fn(gr, func(next http.Handler) http.Handler {
				inTx := withTx(next)
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method == http.MethodGet {
						inTx.ServeHTTP(w, req)
						return
					}
					inTx.ServeHTTP(w, req.WithContext(readHolidayPlaceTraeger(req.Context(), directory)))
				})
			})
		})
	}
}

func readHolidayPlaceTraeger(ctx context.Context, directory enrollmentOwner.HolidayTraegerDirectory) context.Context {
	principal, err := common.CurrentPrincipal(ctx)
	if err != nil || principal.OrganizationID() <= 0 {
		return ctx
	}
	var traeger enrollmentOwner.HolidayTraeger
	if err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		read, err := directory.HolidayTraeger(adminCtx, principal.OrganizationID())
		traeger = read
		return err
	}); err != nil {
		return ctx
	}
	return context.WithValue(ctx, holidayPlaceTraegerKey{}, prefetchedHolidayTraeger{organizationID: principal.OrganizationID(), traeger: traeger})
}
