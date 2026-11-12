//go:build testhooks

package api

import (
	"context"
	"sync"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-ogs/project-phoenix/modules/delivery/runtime"
)

// testProviderBlocks stands in for the mail provider's record: tests set what it
// holds per address, everything else runs for real.
var testProviderBlocks sync.Map

// testProviderAsked records the addresses the provider was asked about.
var testProviderAsked sync.Map

// realEmailOutcomes names the embedded reads: its method of the same name would
// otherwise collide with the field.
type realEmailOutcomes = deliveryRuntime.EmailOutcomes

type providerBlockOutcomes struct {
	realEmailOutcomes
}

// ProviderEmailBlocks answers from the test's record, within the budget Delivery
// applies. A request that shares one keeps sharing it here.
func (providerBlockOutcomes) ProviderEmailBlocks(ctx context.Context, addresses []string) map[string]deliveryContract.EmailProviderBlock {
	blocks := make(map[string]deliveryContract.EmailProviderBlock, len(addresses))
	budget := deliveryContract.ProviderBlockBudgetOf(ctx)
	for _, address := range addresses {
		key := deliveryContract.NormalizedEmailAddress(address)
		if budget.Attempt(ctx, func(context.Context) error { return nil }) != nil {
			blocks[key] = deliveryContract.EmailProviderBlockUnreadable
			continue
		}
		testProviderAsked.Store(key, true)
		blocks[key] = deliveryContract.EmailProviderBlockNone
		if block, ok := testProviderBlocks.Load(key); ok {
			blocks[key] = block.(deliveryContract.EmailProviderBlock)
		}
	}
	return blocks
}
