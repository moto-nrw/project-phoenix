package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// DatabaseFailures supplies driver-free facts for HTTP retry and refusal paths.
// The composition root binds Transaction Runtime; the HTTP adapter owns this port.
type DatabaseFailures struct {
	Transient  func(error) bool
	Constraint func(error) bool
	// SavepointRollback reports a savepoint whose rollback failed in the unit
	// of work (E19). Nil matches nothing.
	SavepointRollback func(error) bool
}

// savepointControlFailed reports a savepoint rollback the database failed
// while the request was still alive. Such a transaction was untrusted, so
// the answer is 500 ahead of any business, constraint or transient rule
// (E19, #6316). A cancelled or expired request keeps its existing answer:
// the savepoint failed because the request ended.
func (f DatabaseFailures) savepointControlFailed(r *http.Request, err error) bool {
	if f.SavepointRollback == nil || !f.SavepointRollback(err) || r.Context().Err() != nil {
		return false
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// requestEnded answers a database failure of a request whose own context
// ended: 499 when the client cancelled, 408 when its deadline expired. The
// request's state decides, not the error alone, because pgdriver reports an
// expired deadline as a network read timeout as often as
// context.DeadlineExceeded, and a failed savepoint control carries whichever
// it got (#6316). Business refusals keep their answer.
func (f DatabaseFailures) requestEnded(r *http.Request, err error) render.Renderer {
	ended := r.Context().Err()
	if ended == nil || !f.infrastructure(err) {
		return nil
	}
	if errors.Is(ended, context.Canceled) {
		return common.ErrorClientClosed(err)
	}
	return common.ErrorRequestTimeout(err)
}

func (f DatabaseFailures) infrastructure(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		(f.Transient != nil && f.Transient(err)) || (f.SavepointRollback != nil && f.SavepointRollback(err))
}
