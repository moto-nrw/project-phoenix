package activityrecords

type DatabaseError struct {
	Op  string
	Err error
}

func (e *DatabaseError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

func (e *DatabaseError) Unwrap() error { return e.Err }

type notFoundError struct{}

func (notFoundError) Error() string       { return "repository: not found" }
func (notFoundError) RepositoryNotFound() {}

var ErrNotFound error = notFoundError{}

func WrapDatabaseError(operation string, err error) error {
	return &DatabaseError{Op: operation, Err: err}
}

func WrapNotFoundDatabaseError(operation string) error {
	return WrapDatabaseError(operation, ErrNotFound)
}
