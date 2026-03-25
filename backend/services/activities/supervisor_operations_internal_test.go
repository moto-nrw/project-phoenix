package activities

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSupervisorsForGroups_EmptyList(t *testing.T) {
	svc := &Service{}
	result, err := svc.GetSupervisorsForGroups(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, result)
}
