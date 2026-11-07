package owners

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
)

type letterStatusesDouble struct {
	delivery.Capability
	rows []deliveryContract.EmailDeliveryStatus
}

func (d letterStatusesDouble) EmailDeliveryStatuses(context.Context, int64, deliveryContract.RelatedEntity) ([]deliveryContract.EmailDeliveryStatus, error) {
	return d.rows, nil
}

// embeddedOutcomes names the embedded reads: its method of the same name would
// otherwise collide with the field.
type embeddedOutcomes = delivery.EmailOutcomes

type providerRecordDouble struct {
	embeddedOutcomes
	asked  *[]string
	blocks map[string]deliveryContract.EmailProviderBlock
}

func (d providerRecordDouble) ProviderEmailBlocks(_ context.Context, addresses []string) map[string]deliveryContract.EmailProviderBlock {
	*d.asked = append(*d.asked, addresses...)
	return d.blocks
}

// The provider is asked about the address another mail would go to, if a mail of the
// letter to exactly that address left a block. A new address has no such history.
func TestLetterRowsAskTheProviderAboutARefusedAddressOnly(t *testing.T) {
	t.Parallel()
	same, old, fresh, back := "gleich@example.test", "alt@example.test", "neu@example.test", "zurueck@example.test"
	asked := []string{}
	adapter := announcementDeliveryAdapter{
		module: letterStatusesDouble{rows: []deliveryContract.EmailDeliveryStatus{
			{DeliveryID: 11, RecipientEmail: &same, MailedTo: &same, Outcome: deliveryContract.EmailUnsubscribed},
			{DeliveryID: 12, RecipientEmail: &fresh, MailedTo: &old, Outcome: deliveryContract.EmailComplained},
			{DeliveryID: 13, RecipientEmail: &back, MailedTo: &fresh, Outcome: deliveryContract.EmailUndeliverable, CurrentAddressRefused: true},
		}},
		outcomes: providerRecordDouble{asked: &asked, blocks: map[string]deliveryContract.EmailProviderBlock{
			same: deliveryContract.EmailProviderBlockActive,
			back: deliveryContract.EmailProviderBlockActive,
		}},
	}

	rows, err := adapter.ListForEntity(context.Background(), 7, "parent_announcement", 42)

	require.NoError(t, err)
	assert.Equal(t, []string{same, back}, asked)
	require.Len(t, rows, 3)
	assert.Equal(t, "active", rows[0].ProviderBlock)
	assert.Empty(t, rows[1].ProviderBlock, "the corrected address is no question for the provider")
	assert.Equal(t, "active", rows[2].ProviderBlock, "the earlier refusal of this address counts")
}
