package organizationtenancy_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type demoOrders struct {
	progress map[string]*organizationtenancy.DemoSchoolProgress
	retired  []string
	used     []string
	ordered  int
}

func (o *demoOrders) OrderDemoSchool(context.Context, string, string, string) (string, error) {
	o.ordered++
	return "neu-x1", nil
}

func (o *demoOrders) DemoSchoolProgress(_ context.Context, slug string) (*organizationtenancy.DemoSchoolProgress, error) {
	return o.progress[slug], nil
}

func (o *demoOrders) MarkDemoSchoolUsed(_ context.Context, slug string, _ time.Time) error {
	o.used = append(o.used, slug)
	return nil
}

func (o *demoOrders) RetireDemoSchool(_ context.Context, slug string) error {
	o.retired = append(o.retired, slug)
	return nil
}

type demoSchools map[string]organizationtenancy.School

func (s demoSchools) FindSchool(_ context.Context, id int64) (organizationtenancy.School, error) {
	for _, school := range s {
		if school.ID == id {
			return school, nil
		}
	}
	return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotFound
}

func (s demoSchools) FindSchoolBySlug(_ context.Context, slug string) (organizationtenancy.School, error) {
	if school, ok := s[slug]; ok {
		return school, nil
	}
	return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotFound
}

func TestDemoSchoolAccessOrdersAndEntersAVisitorsSchool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	orders := &demoOrders{progress: map[string]*organizationtenancy.DemoSchoolProgress{
		"bereit": {Status: organizationtenancy.DemoSchoolReady, SchoolID: 41, VisitorAccountID: 51, VisitorParentAccountID: 61},
		"wartet": {Status: organizationtenancy.DemoSchoolPreparing},
	}}
	access := organizationtenancy.NewDemoSchoolAccess(demoSchools{"bereit": {ID: 41, Active: true}}, organizationtenancy.NewDemoSchoolOrders(orders), "")

	slug, err := access.PrepareDemoSchool(ctx, "Schule", "Mia", "Meier")
	require.NoError(t, err)
	assert.Equal(t, "neu-x1", slug)
	slug, err = access.ReplaceDemoSchool(ctx, "bereit", "Schule", "Mia", "Meier")
	require.NoError(t, err)
	assert.Equal(t, "neu-x1", slug)
	assert.Equal(t, []string{"bereit"}, orders.retired)

	entry, err := access.DemoSchoolEntry(ctx, "bereit")
	require.NoError(t, err)
	assert.Equal(t, organizationtenancy.DemoSchoolEntry{Exists: true, Status: organizationtenancy.DemoSchoolReady, SchoolID: 41, AccountID: 51, ParentAccountID: 61}, entry)
	entry, _ = access.DemoSchoolEntry(ctx, "wartet")
	assert.Equal(t, organizationtenancy.DemoSchoolEntry{Exists: true, Status: organizationtenancy.DemoSchoolPreparing}, entry)
	entry, _ = access.DemoSchoolEntry(ctx, "unbekannt")
	assert.Equal(t, organizationtenancy.DemoSchoolEntry{Status: organizationtenancy.DemoSchoolPreparing}, entry)

	require.NoError(t, access.MarkDemoSchoolUsed(ctx, "bereit", time.Now()))
	assert.Equal(t, []string{"bereit"}, orders.used)
}

func TestDemoSchoolAccessSharesTheStandingSchool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	orders := &demoOrders{}
	standing := organizationtenancy.StandingDemoSchool(true)
	access := organizationtenancy.NewDemoSchoolAccess(demoSchools{standing: {ID: 42, Active: true}}, organizationtenancy.NewDemoSchoolOrders(orders), standing)

	slug, err := access.PrepareDemoSchool(ctx, "Schule", "Mia", "Meier")
	require.NoError(t, err)
	assert.Equal(t, "messe-demo", slug)
	assert.Zero(t, orders.ordered, "the standing school is never ordered")
	_, err = access.ReplaceDemoSchool(ctx, standing, "Schule", "Mia", "Meier")
	require.ErrorIs(t, err, organizationtenancy.ErrDemoRestartRefused)
	require.NoError(t, access.MarkDemoSchoolUsed(ctx, standing, time.Now()))
	assert.Empty(t, orders.used, "the standing school has no order to note")
	entry, err := access.DemoSchoolEntry(ctx, standing)
	require.NoError(t, err)
	assert.Equal(t, organizationtenancy.DemoSchoolEntry{Exists: true, Status: organizationtenancy.DemoSchoolReady, SchoolID: 42}, entry)
	assert.Empty(t, organizationtenancy.StandingDemoSchool(false))
}
