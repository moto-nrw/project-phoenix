package guardians

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDeliveryProfileIDs(t *testing.T) {
	ids, err := parseDeliveryProfileIDs(" 7,11 ")
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 11}, ids)

	for _, raw := range []string{"0", "-1", "abc", "1,"} {
		_, err := parseDeliveryProfileIDs(raw)
		assert.Error(t, err, "input %q must be rejected", raw)
	}

	tooMany := make([]string, maxDeliveryProfileIDs+1)
	for i := range tooMany {
		tooMany[i] = strconv.Itoa(i + 1)
	}
	_, err = parseDeliveryProfileIDs(strings.Join(tooMany, ","))
	assert.Error(t, err)
}
