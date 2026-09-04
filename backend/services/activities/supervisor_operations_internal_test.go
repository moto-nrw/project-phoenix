package activities

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSupervisorsForGroups_EmptyList(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	result, err := svc.GetSupervisorsForGroups(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, result)
}
