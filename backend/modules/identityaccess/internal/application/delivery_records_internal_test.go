package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type attachedKey struct{}

type deliveryRecordStore struct {
	err      error
	attached []bool
	errors   []int
}

func (s *deliveryRecordStore) record(ctx context.Context, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	s.attached = append(s.attached, ctx.Value(attachedKey{}) == true)
	s.errors = append(s.errors, len([]rune(*delivery.Error)))
	return domain.OperationStats{}, s.err
}

type resetRecordStore struct {
	ports.PasswordResetStore
	*deliveryRecordStore
}

func (s resetRecordStore) RecordPasswordResetDelivery(ctx context.Context, _ int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	return s.record(ctx, delivery)
}

type invitationRecordStore struct {
	ports.SchoolInvitationStore
	*deliveryRecordStore
}

func (s invitationRecordStore) RecordSchoolInvitationDelivery(ctx context.Context, _ int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	return s.record(ctx, delivery)
}

// The delivery records write under the module's unit of work, bound the
// stored error like the flows do, and answer in the module's vocabulary.
func TestDeliveryRecords_RecordLikeTheModule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	attach := func(ctx context.Context) context.Context { return context.WithValue(ctx, attachedKey{}, true) }
	store := &deliveryRecordStore{err: errors.New("record failed")}
	long := strings.Repeat("x", 5000)
	delivery := identityaccess.TokenDelivery{Error: &long}
	id := int64(len(t.Name()))

	resetErr := NewPasswordResetDeliveryRecords(resetRecordStore{deliveryRecordStore: store}, attach).RecordPasswordResetDelivery(ctx, id, delivery)
	require.EqualError(t, resetErr, passwordResetError(store.err).Error())
	invitationErr := NewSchoolInvitationDeliveryRecords(invitationRecordStore{deliveryRecordStore: store}, attach).RecordSchoolInvitationDelivery(ctx, id, delivery)
	require.EqualError(t, invitationErr, invitationError(store.err).Error())

	require.Equal(t, []bool{true, true}, store.attached)
	bounded := len([]rune(*domain.TokenDelivery(delivery).Bounded().Error))
	require.Equal(t, []int{bounded, bounded}, store.errors)
	require.Less(t, bounded, len(long))
}
