package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type slotListStaffStub struct {
	found bool
	err   error
	calls int
}

func (s *slotListStaffStub) CurrentStaffIDOfPerson(context.Context) (int64, bool, error) {
	s.calls++
	return 0, s.found, s.err
}

// The root binds the slot lists' read access and renderer; these tests pin
// the two bindings the integration suite composes the projection with.

func TestClassDaySlotListAccess(t *testing.T) {
	t.Parallel()

	admin := testpkg.WithPermissionsContext(context.Background(), []string{"admin:*"})
	plain := context.Background()

	t.Run("administrator reads without a staff record", func(t *testing.T) {
		staff := &slotListStaffStub{}
		allowed, err := api.ClassDaySlotListAccess{Staff: staff}.CanReadStudents(admin)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Zero(t, staff.calls)
	})
	t.Run("staff member reads", func(t *testing.T) {
		allowed, err := api.ClassDaySlotListAccess{Staff: &slotListStaffStub{found: true}}.CanReadStudents(plain)
		require.NoError(t, err)
		assert.True(t, allowed)
	})
	t.Run("account without a staff record reads no children", func(t *testing.T) {
		allowed, err := api.ClassDaySlotListAccess{Staff: &slotListStaffStub{}}.CanReadStudents(plain)
		require.NoError(t, err)
		assert.False(t, allowed)
	})
	t.Run("without a staff reader only administrators read", func(t *testing.T) {
		allowed, err := api.ClassDaySlotListAccess{}.CanReadStudents(plain)
		require.NoError(t, err)
		assert.False(t, allowed)
	})
	t.Run("lookup failure propagates", func(t *testing.T) {
		failure := errors.New("staff lookup failed")
		allowed, err := api.ClassDaySlotListAccess{Staff: &slotListStaffStub{err: failure}}.CanReadStudents(plain)
		require.ErrorIs(t, err, failure)
		assert.False(t, allowed)
	})
}

type slotListRendererStub struct {
	doc      lists.Document
	format   lists.Format
	filename string
}

func (r *slotListRendererStub) Render(doc lists.Document, format lists.Format, filenameBase string) (lists.File, error) {
	r.doc, r.format, r.filename = doc, format, filenameBase
	return lists.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

func TestClassDaySlotListRendererTranslatesTheDocument(t *testing.T) {
	t.Parallel()

	generated := time.Date(2025, 9, 3, 12, 0, 0, 0, time.UTC)
	stub := &slotListRendererStub{}
	file, err := api.ClassDaySlotListRenderer{Renderer: stub}.Render(classday.ExportDocument{
		Title: "Tagesliste", Subtitle: "Mensa", GeneratedAt: generated, Filters: []string{"Datum: 03.09.2025"}, Footer: "Vertraulich",
		Columns: []classday.ExportColumn{
			{ID: classday.ExportColumnName, Label: "Name"}, {ID: classday.ExportColumnSchoolClass, Label: "Klasse"},
			{ID: classday.ExportColumnGroup, Label: "Gruppe"}, {ID: classday.ExportColumnSlot, Label: "Angebot / Zeitraum"},
			{ID: classday.ExportColumnPlannedPickup, Label: "Geplante Abholung"}, {ID: classday.ExportColumnPresenceStatus, Label: "Status"},
		},
		Rows: []classday.ExportRow{
			{GroupTitle: "Klasse 3a"},
			{Values: map[classday.ExportColumnID]string{classday.ExportColumnName: "Kind", classday.ExportColumnPresenceStatus: "anwesend"}},
		},
	}, classday.FormatXLSX, "Tagesliste Plan")
	require.NoError(t, err)

	assert.Equal(t, classday.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: "Tagesliste Plan.pdf"}, file)
	assert.Equal(t, lists.FormatXLSX, stub.format)
	assert.Equal(t, "Tagesliste Plan", stub.filename)
	assert.Equal(t, lists.Document{
		Title: "Tagesliste", Subtitle: "Mensa", GeneratedAt: generated, Filters: []string{"Datum: 03.09.2025"}, Footer: "Vertraulich",
		Columns: []lists.Column{
			{ID: lists.ColumnName, Label: "Name"}, {ID: lists.ColumnSchoolClass, Label: "Klasse"},
			{ID: lists.ColumnGroup, Label: "Gruppe"}, {ID: lists.ColumnSlot, Label: "Angebot / Zeitraum"},
			{ID: lists.ColumnPlannedPickup, Label: "Geplante Abholung"}, {ID: lists.ColumnPresenceStatus, Label: "Status"},
		},
		Rows: []lists.Row{
			{GroupTitle: "Klasse 3a"},
			{Values: map[lists.ColumnID]string{lists.ColumnName: "Kind", lists.ColumnPresenceStatus: "anwesend"}},
		},
	}, stub.doc)
}

func TestClassDaySlotListRendererRejectsUnknownVocabulary(t *testing.T) {
	t.Parallel()

	renderer := api.ClassDaySlotListRenderer{Renderer: &slotListRendererStub{}}
	_, err := renderer.Render(classday.ExportDocument{}, classday.Format("docx"), "Tagesliste")
	require.ErrorContains(t, err, "unsupported slot list format")

	_, err = renderer.Render(classday.ExportDocument{Columns: []classday.ExportColumn{{ID: "unknown"}}}, classday.FormatPDF, "Tagesliste")
	require.ErrorContains(t, err, "unknown slot list column")
}
