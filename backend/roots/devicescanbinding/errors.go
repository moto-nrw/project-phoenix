package devicescanbinding

// mappedError keeps the retained service's message on the wire while
// answering errors.Is for the workflow's port sentinel.
type mappedError struct {
	err      error
	sentinel error
}

func (e mappedError) Error() string { return e.err.Error() }

func (e mappedError) Unwrap() error { return e.err }

func (e mappedError) Is(target error) bool { return target == e.sentinel }

func mapError(err, sentinel error) error { return mappedError{err: err, sentinel: sentinel} }
