package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClassLetterMailIntroNamesTheSender pins the mail of #4800: parents read
// the Lehrkraft, her class and the school before the usual line, which then
// starts a new sentence; a letter of the OGS keeps its intro.
func TestClassLetterMailIntroNamesTheSender(t *testing.T) {
	t.Parallel()
	name, class := "Lena Lehrerin", "3b"
	letter := &StaffParentAnnouncement{SenderName: &name, SenderSchoolClass: &class}

	assert.Equal(t,
		"diese Nachricht kommt von Lena Lehrerin, Klasse 3b, OGS Am Berg. Die folgende Mitteilung ist ein Elternbrief. Den vollständigen Text finden Sie unten.",
		classLetterMailIntro(letter, "OGS Am Berg", letterIntro))
	assert.Equal(t,
		"diese Nachricht kommt von Lena Lehrerin, Klasse 3b. Den vollständigen Inhalt finden Sie im Eltern-Portal.",
		classLetterMailIntro(letter, "", ""))
	assert.Equal(t, letterIntro, classLetterMailIntro(&StaffParentAnnouncement{}, "OGS Am Berg", letterIntro))
}
