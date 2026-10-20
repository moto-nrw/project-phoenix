package application

// The student half of the recording engine: every call is counted and its
// normalized arguments are kept so the module's validation can be asserted
// without a database.
type studentCall struct {
	ids     []int64
	classes []string
}
