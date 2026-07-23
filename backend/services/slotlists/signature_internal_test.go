package slotlists

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestListSignature guards the #1565 review-pass-2 atomic export drift guard:
// the export endpoint rebuilds the list in its own request after the client
// verified a preview, so RenderList compares the fresh build's signature against
// the client's ExpectedSignature. The signature must therefore be stable for
// identical content and change for any row/counter/label difference the export
// would render.
func TestListSignature(t *testing.T) {
	gid := int64(7)
	base := func() *Result {
		return &Result{
			ListLabel:  "Ganztag",
			Provenance: "Plan",
			Counters:   Counters{Planned: 2, Present: 1},
			Rows: []Row{
				{StudentID: 1, Name: "Anna", SchoolClass: "1a", GroupName: "Rot", GroupID: &gid, InstanceID: 5, Slot: "AG", StatusLabel: "Anwesend", Planned: true, Present: true},
				{StudentID: 2, Name: "Ben", SchoolClass: "1b", GroupName: "Blau", Slot: "AG", StatusLabel: "Fehlt", Planned: true},
			},
		}
	}

	t.Run("identical content yields identical signature", func(t *testing.T) {
		assert.Equal(t, listSignature(base()), listSignature(base()))
	})

	t.Run("a changed status flips the signature", func(t *testing.T) {
		changed := base()
		changed.Rows[1].StatusLabel = "Abgemeldet"
		changed.Rows[1].Excused = true
		assert.NotEqual(t, listSignature(base()), listSignature(changed))
	})

	t.Run("a changed counter flips the signature", func(t *testing.T) {
		changed := base()
		changed.Counters.Present = 2
		assert.NotEqual(t, listSignature(base()), listSignature(changed))
	})

	t.Run("a changed label flips the signature", func(t *testing.T) {
		changed := base()
		changed.ListLabel = "Ganztag lang"
		assert.NotEqual(t, listSignature(base()), listSignature(changed))
	})

	t.Run("an added row flips the signature", func(t *testing.T) {
		changed := base()
		changed.Rows = append(changed.Rows, Row{StudentID: 3, Name: "Cara"})
		assert.NotEqual(t, listSignature(base()), listSignature(changed))
	})

	t.Run("field boundaries are unambiguous", func(t *testing.T) {
		// Moving a character across the Name/SchoolClass boundary must not
		// produce a colliding signature — the unit/record separators guard this.
		a := base()
		a.Rows[0].Name = "Ann"
		a.Rows[0].SchoolClass = "a1a"
		b := base()
		b.Rows[0].Name = "Anna"
		b.Rows[0].SchoolClass = "1a"
		assert.NotEqual(t, listSignature(a), listSignature(b))
	})
}
