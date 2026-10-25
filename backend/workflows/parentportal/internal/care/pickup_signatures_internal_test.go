package care

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type pickupSignatureRuntimeStub struct{}

func (pickupSignatureRuntimeStub) WithinTenant(ctx context.Context, _ int64, fn func(context.Context) error) error {
	return fn(ctx)
}

func (pickupSignatureRuntimeStub) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (pickupSignatureRuntimeStub) RegisterAfterCommit(context.Context, func()) {}

func (pickupSignatureRuntimeStub) WithTenantID(ctx context.Context, _ int64) context.Context {
	return ctx
}

type pickupSignatureChildReadsStub struct {
	ChildReads
	child *ChildSummary
}

func (s pickupSignatureChildReadsStub) FindForAccount(context.Context, int64, int64) (*ChildSummary, error) {
	return s.child, nil
}

func TestPickupSignatureStatementWeeklyKeepsEveryWeekday(t *testing.T) {
	t.Parallel()

	service := &Service{Config: Config{
		Runtime: pickupSignatureRuntimeStub{},
		ChildRepo: pickupSignatureChildReadsStub{child: &ChildSummary{
			StudentID: 7,
			TenantID:  8,
			FirstName: "Mia",
			LastName:  "Muster",
			GuardianPermissions: map[string]bool{
				GuardianPermissionPickupManage: true,
			},
		}},
	}}

	statement, err := service.PickupSignatureStatement(context.Background(), 9, 7, PickupSignatureStatementInput{
		Kind:     PickupSignatureStatementWeekly,
		Weekdays: []int{1, 2, 3, 4, 5},
	})
	require.NoError(t, err)
	require.Equal(t, "Ich erlaube, dass das Kind Mia Muster an allen Tagen allein nach Hause geht.", statement)
}
