package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicMealPlanContractPersistsNormalizedDishes(t *testing.T) {
	t.Parallel()
	enabled := func(context.Context) (bool, error) { return true, nil }
	settings, err := compose.NewBoundSettings(enabled, enabled, func(context.Context) (string, int, error) { return "09:00", 0, nil })
	require.NoError(t, err)
	module, err := compose.New(compose.Dependencies{DB: testpkg.SetupTestDB(t), Settings: settings, Observe: func(compose.Observation) {}, Now: time.Now, Participants: func(context.Context, string) ([]mealplan.ParticipantCandidate, error) { return nil, nil }})
	require.NoError(t, err)
	require.NoError(t, module.ReplaceDay(testpkg.Ctx(t), mealplan.ReplaceDay{Date: "2026-09-07", Dishes: []mealplan.Dish{{Dish: "  Suppe  "}, {Dish: "  "}}}))
	entries, err := module.Week(testpkg.Ctx(t), "2026-09-07")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "Suppe", entries[0].Dish)
	require.Equal(t, mealplan.Date("2026-09-07"), entries[0].Date)
}
