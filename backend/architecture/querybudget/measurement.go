package querybudget

// Measurement keeps a SQL bucket and the final chi route of its request together.
// Empty Route denotes a measurement without an HTTP request.
type Measurement struct {
	Statements []string
	Route      Routes
}
