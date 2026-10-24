package studentpresencehttp

import (
	errors "errors"
	fmt "fmt"
	http "net/http"

	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// getStaffIDFromJWT extracts the staff ID from JWT claims by looking up the person and staff
func (rs *Resource) getStaffIDFromJWT(r *http.Request) (int64, error) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return 0, errors.New("no valid JWT claims found")
	}

	// Get person from account ID
	person, err := rs.PeopleDirectory.FindPersonByAccount(r.Context(), int64(claims.ID))
	if err != nil {
		if errors.Is(err, peopleModule.ErrPersonNotFound) {
			return 0, errPersonNotFoundForAccount
		}
		return 0, fmt.Errorf("find person by account: %w", err)
	}

	// Get staff from person ID
	staffID, found, err := rs.Persons.StaffIDForPerson(r.Context(), person.ID)
	if err != nil {
		return 0, fmt.Errorf("find staff by person: %w", err)
	}
	if !found {
		return 0, errUserNotStaff
	}

	return staffID, nil
}
