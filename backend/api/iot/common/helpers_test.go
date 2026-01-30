package common_test

import (
	"testing"

	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeTagID_AlreadyNormalized(t *testing.T) {
	assert.Equal(t, "ABCD1234", iotCommon.NormalizeTagID("ABCD1234"))
}

func TestNormalizeTagID_WithColons(t *testing.T) {
	assert.Equal(t, "ABCD1234", iotCommon.NormalizeTagID("AB:CD:12:34"))
}

func TestNormalizeTagID_WithDashes(t *testing.T) {
	assert.Equal(t, "ABCD1234", iotCommon.NormalizeTagID("ab-cd-12-34"))
}

func TestNormalizeTagID_WithSpaces(t *testing.T) {
	assert.Equal(t, "ABCD1234", iotCommon.NormalizeTagID("ab cd 12 34"))
}

func TestNormalizeTagID_LeadingTrailingSpaces(t *testing.T) {
	assert.Equal(t, "ABCD1234", iotCommon.NormalizeTagID("  ABCD1234  "))
}

func TestNormalizeTagID_EmptyString(t *testing.T) {
	assert.Equal(t, "", iotCommon.NormalizeTagID(""))
}

func TestNormalizeTagID_MixedSeparators(t *testing.T) {
	assert.Equal(t, "ABCD1234", iotCommon.NormalizeTagID("aB:cD-12 34"))
}

func TestNormalizeTagID_Lowercase(t *testing.T) {
	assert.Equal(t, "ABCDEF", iotCommon.NormalizeTagID("abcdef"))
}

func TestNormalizeTagID_OnlySpaces(t *testing.T) {
	assert.Equal(t, "", iotCommon.NormalizeTagID("   "))
}
