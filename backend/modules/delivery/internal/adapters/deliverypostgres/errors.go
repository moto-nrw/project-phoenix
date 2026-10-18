package delivery

// databaseError reports the native subscription store failure. Its message,
// unwrap chain and StoreFailure marker retain the established error contract.
type databaseError struct {
	Op  string
	Err error
}

func (e *databaseError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

func (e *databaseError) Unwrap() error    { return e.Err }
func (*databaseError) StoreFailure() bool { return true }
