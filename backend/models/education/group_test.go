package education

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroup_TableName(t *testing.T) {
	group := &Group{}
	assert.Equal(t, "education.groups", group.TableName())
}

func TestGroup_BeforeAppendModel(t *testing.T) {
	group := &Group{Name: "Test Group"}
	// This should not panic or return error
	err := group.BeforeAppendModel(nil)
	assert.NoError(t, err)
}
