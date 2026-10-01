package domain

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The note card's rules are the table's CHECK constraints spelled once in Go,
// so an invalid write fails with a domain error instead of surfacing a
// constraint name to a user. These tests pin the pairs that couple two fields —
// the ones a later edit is most likely to break apart.

func validNote() CreateStudentNote {
	date := calendar.NewDate(2026, time.September, 9)
	return CreateStudentNote{
		StudentID: 7, AuthorAccountID: 3,
		Kind: StudentNoteKindJournal, Visibility: StudentNoteVisibilityAllStaff,
		Body:    "Hat heute in der Hausaufgabenzeit gut mitgearbeitet.",
		Subject: StudentNoteSubject{Date: &date},
	}
}

func TestCreateStudentNoteValidate(t *testing.T) {
	t.Parallel()

	activityID, educationID := int64(11), int64(12)
	date := calendar.NewDate(2026, time.September, 9)

	tests := []struct {
		name    string
		mutate  func(*CreateStudentNote)
		wantErr bool
	}{
		{name: "plain journal entry", mutate: func(*CreateStudentNote) {}},
		{
			name: "durable hint carries no day",
			mutate: func(c *CreateStudentNote) {
				c.Kind = StudentNoteKindPermanent
				c.Subject.Date = nil
			},
		},
		{
			name: "one occurrence is the activity plus the day",
			mutate: func(c *CreateStudentNote) {
				c.Subject.ActivityGroupID = &activityID
				c.Subject.Date = &date
			},
		},
		{
			name: "group leads needs something to lead",
			mutate: func(c *CreateStudentNote) {
				c.Visibility = StudentNoteVisibilityGroupLeads
			},
			wantErr: true,
		},
		{
			name: "group leads with a group reference is fine",
			mutate: func(c *CreateStudentNote) {
				c.Visibility = StudentNoteVisibilityGroupLeads
				c.Subject.EducationGroupID = &educationID
			},
		},
		{
			name: "two group references leave two leaderships to ask",
			mutate: func(c *CreateStudentNote) {
				c.Subject.ActivityGroupID = &activityID
				c.Subject.EducationGroupID = &educationID
			},
			wantErr: true,
		},
		{
			name: "a durable hint has no day",
			mutate: func(c *CreateStudentNote) {
				c.Kind = StudentNoteKindPermanent
				c.Subject.Date = &date
			},
			wantErr: true,
		},
		{
			name: "a journal entry has a day",
			mutate: func(c *CreateStudentNote) {
				c.Subject.Date = nil
			},
			wantErr: true,
		},
		{
			name:    "empty body",
			mutate:  func(c *CreateStudentNote) { c.Body = "" },
			wantErr: true,
		},
		{
			name: "body over the bound",
			mutate: func(c *CreateStudentNote) {
				runes := make([]rune, MaxStudentNoteRunes+1)
				for index := range runes {
					runes[index] = 'a'
				}
				c.Body = string(runes)
			},
			wantErr: true,
		},
		{
			name:    "unknown visibility",
			mutate:  func(c *CreateStudentNote) { c.Visibility = "everyone" },
			wantErr: true,
		},
		{
			name:    "unknown category",
			mutate:  func(c *CreateStudentNote) { c.Category = "gossip" },
			wantErr: true,
		},
		{
			name:    "no author",
			mutate:  func(c *CreateStudentNote) { c.AuthorAccountID = 0 },
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			note := validNote()
			test.mutate(&note)
			err := note.Validate()
			if test.wantErr {
				require.ErrorIs(t, err, ErrStudentNoteInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStudentNoteAllowsUpdate(t *testing.T) {
	t.Parallel()

	var author, colleague, groupID int64 = 3, 4, 12
	date := calendar.NewDate(2026, time.September, 9)

	base := StudentNote{
		ID: 1, StudentID: 7, AuthorAccountID: &author,
		Origin: StudentNoteOriginStaff, Kind: StudentNoteKindJournal,
		Visibility: StudentNoteVisibilityAllStaff, Subject: StudentNoteSubject{Date: &date},
	}
	update := UpdateStudentNote{
		ID: 1, StudentID: 7, ActorAccountID: author,
		Kind: StudentNoteKindJournal, Visibility: StudentNoteVisibilityAllStaff,
		Body: "Korrigiert.", SubjectDate: &date,
	}

	t.Run("the author corrects", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, base.AllowsUpdate(update))
	})

	t.Run("a colleague does not", func(t *testing.T) {
		t.Parallel()
		attempt := update
		attempt.ActorAccountID = colleague
		require.ErrorIs(t, base.AllowsUpdate(attempt), ErrStudentNoteNotAuthor)
	})

	t.Run("a carried-over hint has nobody to stand behind it", func(t *testing.T) {
		t.Parallel()
		carried := base
		carried.AuthorAccountID = nil
		carried.Origin = StudentNoteOriginMasterData
		require.ErrorIs(t, carried.AllowsUpdate(update), ErrStudentNoteImmutable)
	})

	t.Run("a dated entry can become a durable hint", func(t *testing.T) {
		t.Parallel()
		promote := update
		promote.Kind = StudentNoteKindPermanent
		promote.SubjectDate = nil
		require.NoError(t, base.AllowsUpdate(promote))
	})

	t.Run("a durable hint can become a dated entry", func(t *testing.T) {
		t.Parallel()
		undatedHint := base
		undatedHint.Kind = StudentNoteKindPermanent
		undatedHint.Subject.Date = nil
		require.NoError(t, undatedHint.AllowsUpdate(update))
	})

	t.Run("narrowing to the leadership needs a group", func(t *testing.T) {
		t.Parallel()
		narrow := update
		narrow.Visibility = StudentNoteVisibilityGroupLeads
		require.ErrorIs(t, base.AllowsUpdate(narrow), ErrStudentNoteInvalid)

		withGroup := base
		withGroup.Subject.EducationGroupID = &groupID
		require.NoError(t, withGroup.AllowsUpdate(narrow))
	})
}

func TestStudentNoteAllowsDelete(t *testing.T) {
	t.Parallel()

	groupID, activityID := int64(12), int64(13)
	note := StudentNote{Subject: StudentNoteSubject{EducationGroupID: &groupID}}

	assert.ErrorIs(t, note.AllowsDelete(StudentNoteDeleteAuthorization{}), ErrStudentNoteDeleteForbidden)
	assert.NoError(t, note.AllowsDelete(StudentNoteDeleteAuthorization{
		LedEducationGroupIDs: []int64{groupID},
	}))
	assert.NoError(t, note.AllowsDelete(StudentNoteDeleteAuthorization{Admin: true}))

	note.Subject = StudentNoteSubject{ActivityGroupID: &activityID}
	assert.ErrorIs(t, note.AllowsDelete(StudentNoteDeleteAuthorization{
		LedEducationGroupIDs: []int64{groupID},
	}), ErrStudentNoteDeleteForbidden)
	assert.NoError(t, note.AllowsDelete(StudentNoteDeleteAuthorization{
		LedActivityGroupIDs: []int64{activityID},
	}))

	note.Subject = StudentNoteSubject{}
	assert.NoError(t, note.AllowsDelete(StudentNoteDeleteAuthorization{
		LedEducationGroupIDs: []int64{groupID}, ChildEducationGroupID: &groupID,
	}))
}

func TestStudentNoteEdited(t *testing.T) {
	t.Parallel()

	note := StudentNote{}
	assert.False(t, note.Edited(), "a note nobody touched is not edited")

	note.CreatedAt = note.CreatedAt.AddDate(0, 0, -1)
	assert.True(t, note.Edited(), "a later update marks the entry as reworded")
}
