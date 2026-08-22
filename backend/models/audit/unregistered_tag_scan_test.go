package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnregisteredTagScanMetadataMethods(t *testing.T) {
	t.Parallel()

	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()
	scan := &UnregisteredTagScan{
		TagUID: "ABC123",
	}
	scan.ID = 42
	scan.CreatedAt = createdAt
	scan.UpdatedAt = updatedAt

	require.Equal(t, int64(42), scan.GetID())
	require.Equal(t, createdAt, scan.GetCreatedAt())
	require.Equal(t, updatedAt, scan.GetUpdatedAt())
}
