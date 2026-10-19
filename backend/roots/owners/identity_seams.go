package owners

// retainedSentinel pairs a public outcome of Identity & Access with the
// sentinel a consumer that may not name the owner classifies on.
type retainedSentinel struct {
	public   error
	retained error
}

// retainedError keeps the public cause's text while exposing the retained
// sentinel to errors.Is.
type retainedError struct {
	text     string
	sentinel error
	cause    error
}

func (e *retainedError) Error() string { return e.text }

func (e *retainedError) Unwrap() []error { return []error{e.sentinel, e.cause} }
