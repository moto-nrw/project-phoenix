package querybudget

// Measurement keeps a SQL bucket and the final chi route of its request together.
// HTTPRequest also records unmatched requests, which cannot bind a budget.
type Measurement struct {
	Statements  []string
	Route       Routes
	HTTPRequest bool
}
