package owners

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unknownSettlementTransfer struct{}

func (unknownSettlementTransfer) Transfer(context.Context, exporttransfer.Request) (exporttransfer.Outcome, error) {
	return exporttransfer.Outcome{}, exporttransfer.ErrUnknownSettlement
}

func (unknownSettlementTransfer) Status(context.Context) (exporttransfer.Status, error) {
	return exporttransfer.Status{}, nil
}

func (unknownSettlementTransfer) RecordEmail(context.Context, exporttransfer.EmailRecord) error {
	return nil
}

func TestPayrollTransferMarksUnknownSettlementUncertain(t *testing.T) {
	t.Parallel()

	outcome, err := (payrollTransfer{module: unknownSettlementTransfer{}}).Transfer(context.Background(), workforce.PayrollTransferRequest{})
	require.NoError(t, err)
	assert.True(t, outcome.Uncertain)
	assert.False(t, outcome.Transferred)
}
