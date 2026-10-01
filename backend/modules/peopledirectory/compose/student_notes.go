package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e engine) ListStudentNotes(ctx context.Context, filter peopledirectory.StudentNoteFilter) ([]peopledirectory.StudentNote, error) {
	notes, err := e.studentNotes.List(ctx, toDomainNoteFilter(filter))
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]peopledirectory.StudentNote, 0, len(notes))
	for _, note := range notes {
		result = append(result, toPublicNote(note))
	}
	return result, nil
}

func (e engine) CreateStudentNote(ctx context.Context, input peopledirectory.CreateStudentNote) (peopledirectory.StudentNote, error) {
	note, err := e.studentNotes.Create(ctx, domain.CreateStudentNote{
		StudentID: input.StudentID, AuthorAccountID: input.AuthorAccountID,
		Kind: input.Kind, Visibility: input.Visibility, Category: input.Category,
		Body: input.Body, Subject: toDomainNoteSubject(input.Subject), RevalidateSubject: input.RevalidateSubject,
	})
	if err != nil {
		return peopledirectory.StudentNote{}, mapError(err)
	}
	return toPublicNote(note), nil
}

func (e engine) UpdateStudentNote(ctx context.Context, input peopledirectory.UpdateStudentNote) (peopledirectory.StudentNote, error) {
	note, err := e.studentNotes.Update(ctx, domain.UpdateStudentNote{
		ID: input.ID, StudentID: input.StudentID, ActorAccountID: input.ActorAccountID,
		Kind: input.Kind, Visibility: input.Visibility, Category: input.Category, Body: input.Body,
		SubjectDate: input.SubjectDate,
	})
	if err != nil {
		return peopledirectory.StudentNote{}, mapError(err)
	}
	return toPublicNote(note), nil
}

func (e engine) DeleteStudentNote(ctx context.Context, input peopledirectory.DeleteStudentNote) error {
	var resolveAuthorization func(context.Context) (domain.StudentNoteDeleteAuthorization, error)
	if input.ResolveAuthorization != nil {
		resolveAuthorization = func(ctx context.Context) (domain.StudentNoteDeleteAuthorization, error) {
			resolved, err := input.ResolveAuthorization(ctx)
			return domain.StudentNoteDeleteAuthorization{
				Admin:                 resolved.Admin,
				LedActivityGroupIDs:   resolved.LedActivityGroupIDs,
				LedEducationGroupIDs:  resolved.LedEducationGroupIDs,
				ChildEducationGroupID: resolved.ChildEducationGroupID,
			}, err
		}
	}
	return mapError(e.studentNotes.Delete(ctx, domain.DeleteStudentNote{
		ID: input.ID, StudentID: input.StudentID, ActorAccountID: input.ActorAccountID,
		Authorization: domain.StudentNoteDeleteAuthorization{
			Admin:                 input.Authorization.Admin,
			LedActivityGroupIDs:   input.Authorization.LedActivityGroupIDs,
			LedEducationGroupIDs:  input.Authorization.LedEducationGroupIDs,
			ChildEducationGroupID: input.Authorization.ChildEducationGroupID,
		},
		ResolveAuthorization: resolveAuthorization,
	}))
}

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
