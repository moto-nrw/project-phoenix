package postgres

import (
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// TranslateNotFound joins the persistence-neutral not-found sentinel to a
// driver no-rows error, as the generic repositories do.
func TranslateNotFound(err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return errors.Join(domain.ErrRecordNotFound, err)
}
