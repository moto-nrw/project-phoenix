package enrollment

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Pure-helper tests for the request service. No DB; these run on
// every CI invocation regardless of test-DB wiring.

// ---- newStatusToken -----------------------------------------------------

func TestNewStatusToken_DecodesToThirtyTwoBytes(t *testing.T) {
	t.Parallel()

	token, err := newStatusToken()
	require.NoError(t, err)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err, "token must be RawURLEncoding base64")
	assert.Len(t, raw, 32, "32 random bytes per CSPRNG guarantee")
}

func TestNewStatusToken_UnpaddedRawURLEncoding(t *testing.T) {
	t.Parallel()

	token, err := newStatusToken()
	require.NoError(t, err)
	// RawURLEncoding strips the trailing '=' padding. 32 bytes →
	// 43 base64 chars, no padding.
	assert.Len(t, token, 43)
	for _, r := range token {
		// URL-safe alphabet only: a-z A-Z 0-9 - _.
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_'
		assert.True(t, ok, "char %q must be in the RawURL alphabet", r)
	}
}

func TestNewStatusToken_TokensAreDistinct(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 32)
	for i := 0; i < 32; i++ {
		token, err := newStatusToken()
		require.NoError(t, err)
		_, dup := seen[token]
		require.False(t, dup, "CSPRNG must produce distinct tokens")
		seen[token] = struct{}{}
	}
}

// ---- fnvHash64 ----------------------------------------------------------

func TestFnvHash64_Stable(t *testing.T) {
	t.Parallel()

	// Stability matters: the value is used as an advisory-lock key on
	// the lowercased guardian email so concurrent submissions for the
	// same parent serialize. Changing the algorithm would silently
	// allow duplicates through during a rolling deploy.
	a := fnvHash64("parent@example.com")
	b := fnvHash64("parent@example.com")
	assert.Equal(t, a, b)
}

func TestFnvHash64_DifferentInputsCollideRarely(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t,
		fnvHash64("parent@example.com"),
		fnvHash64("other@example.com"),
	)
}

func TestFnvHash64_EmptyStringIsFnvOffsetBasis(t *testing.T) {
	t.Parallel()

	// FNV-1a offset basis for 64-bit; sanity check that the hash isn't
	// silently zero-initialised.
	assert.Equal(t, uint64(0xcbf29ce484222325), fnvHash64(""))
}

// ---- validateOfferingSelections -----------------------------------------

func TestValidateOfferingSelections_AcceptsKnownOfferings(t *testing.T) {
	t.Parallel()

	open := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "A"},
		2: {Name: "B"},
	}
	children := []SubmitChild{
		{OfferingIDs: []int64{1}},
		{OfferingIDs: []int64{2, 1}},
	}
	assert.NoError(t, validateOfferingSelections(children, open))
}

func TestValidateOfferingSelections_RejectsUnknownOffering(t *testing.T) {
	t.Parallel()

	open := map[int64]*enrollmentModels.CareOffering{1: {Name: "A"}}
	children := []SubmitChild{
		{OfferingIDs: []int64{1, 99}}, // 99 not in catalog
	}
	err := validateOfferingSelections(children, open)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCareOfferingClosed),
		"stale-client picks must surface ErrCareOfferingClosed")
}

func TestValidateOfferingSelections_EmptyChildrenIsOK(t *testing.T) {
	t.Parallel()

	assert.NoError(t, validateOfferingSelections(nil, nil))
}

func TestValidateOfferingSelections_ChildWithNoPicksIsOK(t *testing.T) {
	t.Parallel()

	// Phase-level care offering selection is enforced separately. This
	// helper only checks that everything the parent DID pick is in the
	// open catalog, a child with no picks is silently fine here.
	assert.NoError(t, validateOfferingSelections([]SubmitChild{{}}, map[int64]*enrollmentModels.CareOffering{}))
}

// ---- validateRequiredOfferings ------------------------------------------

func TestValidateRequiredOfferings_NoRequiredIsOK(t *testing.T) {
	t.Parallel()

	open := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "A", IsRequired: false},
		2: {Name: "B", IsRequired: false},
	}
	children := []SubmitChild{{OfferingIDs: nil}}
	assert.NoError(t, validateRequiredOfferings(children, open))
}

func TestValidateRequiredOfferings_AcceptsWhenRequiredSelected(t *testing.T) {
	t.Parallel()

	open := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "Mittagessen", IsRequired: true},
		2: {Name: "AG", IsRequired: false},
	}
	children := []SubmitChild{
		{OfferingIDs: []int64{1}},
		{OfferingIDs: []int64{2, 1}},
	}
	assert.NoError(t, validateRequiredOfferings(children, open))
}

func TestValidateRequiredOfferings_RejectsWhenRequiredMissing(t *testing.T) {
	t.Parallel()

	open := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "Mittagessen", IsRequired: true},
	}
	children := []SubmitChild{
		{OfferingIDs: []int64{}}, // required offering 1 not selected
	}
	err := validateRequiredOfferings(children, open)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRequiredCareOfferingMissing),
		"a missing required offering must surface ErrRequiredCareOfferingMissing")
}

func TestValidateRequiredOfferings_RejectsWhenOnlySomeChildrenComply(t *testing.T) {
	t.Parallel()

	open := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "Mittagessen", IsRequired: true},
	}
	children := []SubmitChild{
		{OfferingIDs: []int64{1}}, // ok
		{OfferingIDs: []int64{}},  // missing
	}
	err := validateRequiredOfferings(children, open)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRequiredCareOfferingMissing))
}

func TestValidateRequiredOfferings_EmptyCatalogIsOK(t *testing.T) {
	t.Parallel()

	assert.NoError(t, validateRequiredOfferings([]SubmitChild{{}}, map[int64]*enrollmentModels.CareOffering{}))
}
