package records

// Renderer is the consumer-owned rendering dependency over native list values.
type Renderer interface {
	Render(Document, Format, string) (File, error)
}
