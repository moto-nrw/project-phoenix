package delivery

import "context"

// IntentTransactions binds provider work to the claimed intent's explicit scope.
type IntentTransactions struct {
	school func(context.Context, int64, func(context.Context) error) error
	system func(context.Context, func(context.Context) error) error
}

func NewIntentTransactions(school func(context.Context, int64, func(context.Context) error) error, system func(context.Context, func(context.Context) error) error) IntentTransactions {
	return IntentTransactions{school: school, system: system}
}
func (t IntentTransactions) WithinTenant(ctx context.Context, id int64, fn func(context.Context) error) error {
	return t.school(ctx, id, fn)
}
func (t IntentTransactions) WithinSystem(ctx context.Context, fn func(context.Context) error) error {
	return t.system(ctx, fn)
}
