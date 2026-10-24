package messaging

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChildEventPayloadPreservesLargeIntegerDetail(t *testing.T) {
	t.Parallel()
	document := json.RawMessage(`{"reference_id":9007199254740993}`)
	payload := eventcontract.NewChildEventPayload(document, nil)
	facts, err := childEventPayloadFacts(payload)
	require.NoError(t, err)
	persisted, err := json.Marshal(facts)
	require.NoError(t, err)
	assert.Equal(t, string(document), string(persisted))
}
