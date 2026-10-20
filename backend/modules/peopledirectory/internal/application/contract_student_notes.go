package application

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func toDomainNoteFilter(filter peopledirectory.StudentNoteFilter) domain.StudentNoteFilter {
	return domain.StudentNoteFilter{
		StudentID:            filter.StudentID,
		NoteID:               filter.NoteID,
		Visibilities:         filter.Audience.Visibilities,
		LedActivityGroupIDs:  filter.Audience.LedActivityGroupIDs,
		LedEducationGroupIDs: filter.Audience.LedEducationGroupIDs,
		ReaderAccountID:      filter.Audience.ReaderAccountID,
		Kind:                 filter.Kind,
	}
}

func toDomainNoteSubject(subject peopledirectory.StudentNoteSubject) domain.StudentNoteSubject {
	return domain.StudentNoteSubject{
		Date:             subject.Date,
		ActivityGroupID:  subject.ActivityGroupID,
		EducationGroupID: subject.EducationGroupID,
	}
}

func toPublicNote(note domain.StudentNote) peopledirectory.StudentNote {
	return peopledirectory.StudentNote{
		ID: note.ID, StudentID: note.StudentID, AuthorAccountID: note.AuthorAccountID,
		Origin: note.Origin, Kind: note.Kind, Visibility: note.Visibility,
		Category: note.Category, Body: note.Body, AuthorName: note.AuthorName,
		Subject: peopledirectory.StudentNoteSubject{
			Date:             note.Subject.Date,
			ActivityGroupID:  note.Subject.ActivityGroupID,
			EducationGroupID: note.Subject.EducationGroupID,
		},
		CreatedAt: note.CreatedAt, UpdatedAt: note.UpdatedAt,
	}
}
