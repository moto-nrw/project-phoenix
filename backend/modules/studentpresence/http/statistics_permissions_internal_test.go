package studentpresencehttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatisticsReportPermissionsNeedSettingsAndChildrenRead(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"config:read", "users:read"}, StatisticsReportPermissions())
}
