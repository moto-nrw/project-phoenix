package identityaccess

import "context"

// RFIDCards owns school-scoped card registration and lookup. Registration
// validates the identifier; lookup also accepts existing legacy identifiers.
type RFIDCards interface {
	LookupRFIDCard(context.Context, string) (id string, active, found bool, err error)
	RegisterRFIDCard(context.Context, string) error
	ValidateRFIDTag(string) error
}

func (m *Module) LookupRFIDCard(ctx context.Context, tag string) (string, bool, bool, error) {
	return m.engine.LookupRFIDCard(ctx, tag)
}

func (m *Module) RegisterRFIDCard(ctx context.Context, tag string) error {
	return m.engine.RegisterRFIDCard(ctx, tag)
}

func (m *Module) ValidateRFIDTag(tag string) error {
	return m.engine.ValidateRFIDTag(tag)
}
