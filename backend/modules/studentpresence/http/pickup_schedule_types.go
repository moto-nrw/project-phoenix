package studentpresencehttp

import (
	errors "errors"
)

var (
	errPersonNotFoundForAccount = errors.New("person not found for account")
	errUserNotStaff             = errors.New("user is not a staff member")
)
