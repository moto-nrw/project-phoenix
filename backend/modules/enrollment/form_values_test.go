package enrollment_test

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormValuesEncodeEmptyAsObject(t *testing.T) {
	t.Parallel()

	var zero enrollment.FormValues
	encoded, err := json.Marshal(zero)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(encoded))

	emptied := enrollment.CollectFormValues(maps.All(map[string]any{"a": true}))
	emptied.Unset("a")
	encoded, err = json.Marshal(struct {
		Values enrollment.FormValues `json:"values"`
	}{emptied})
	require.NoError(t, err)
	assert.JSONEq(t, `{"values":{}}`, string(encoded))
}

func TestFormValuesDecodeNullAndAbsentAsEmpty(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`null`, `{}`, ``} {
		values := enrollment.CollectFormValues(maps.All(map[string]any{"stale": 1.0}))
		require.NoError(t, values.UnmarshalJSON([]byte(input)), input)
		assert.True(t, values.IsZero(), input)
		assert.Equal(t, 0, values.Len(), input)
	}

	var wrapped struct {
		Values enrollment.FormValues `json:"values"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"values":null}`), &wrapped))
	assert.True(t, wrapped.Values.IsZero())
}

func TestFormValuesRoundTrip(t *testing.T) {
	t.Parallel()

	input := `{"allergies":"Nüsse","count":2,"days":["mo","di"],"nested":{"ok":true},"none":null}`
	var values enrollment.FormValues
	require.NoError(t, json.Unmarshal([]byte(input), &values))
	assert.Equal(t, 5, values.Len())
	allergies, ok := values.Lookup("allergies")
	assert.True(t, ok)
	assert.Equal(t, "Nüsse", allergies)
	count, _ := values.Lookup("count")
	assert.Equal(t, 2.0, count)
	none, ok := values.Lookup("none")
	assert.True(t, ok, "a null answer is a present key")
	assert.Nil(t, none)
	_, ok = values.Lookup("missing")
	assert.False(t, ok)

	encoded, err := json.Marshal(values)
	require.NoError(t, err)
	assert.JSONEq(t, input, string(encoded))
	assert.Equal(t, maps.Collect(values.All()), maps.Collect(values.Clone().All()))
}

func TestFormValuesRejectNonObject(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`[]`, `"text"`, `1`, `true`, `{`} {
		var values enrollment.FormValues
		assert.Error(t, json.Unmarshal([]byte(`{"v":`+input+`}`), &struct {
			V *enrollment.FormValues `json:"v"`
		}{&values}), input)
	}
}

func TestFormValuesCloneIsIndependent(t *testing.T) {
	t.Parallel()

	original := enrollment.CollectFormValues(maps.All(map[string]any{"a": 1.0}))
	clone := original.Clone()
	clone.Set("b", 2.0)
	clone.Unset("a")
	assert.Equal(t, map[string]any{"a": 1.0}, maps.Collect(original.All()))
	assert.Equal(t, map[string]any{"b": 2.0}, maps.Collect(clone.All()))
}
