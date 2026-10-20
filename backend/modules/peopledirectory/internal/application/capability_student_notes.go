package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) ListStudentNotes(ctx context.Context, filter peopledirectory.StudentNoteFilter) ([]peopledirectory.StudentNote, error) {
	if filter.StudentID <= 0 {
		return nil, &peopledirectory.InvalidStudentError{Reason: "student notes require a positive student ID"}
	}
	if filter.Kind != "" && !peopledirectory.ValidStudentNoteKind(filter.Kind) {
		return nil, peopledirectory.ErrStudentNoteInvalid
	}

	notes, err := s.studentNotes.List(ctx, toDomainNoteFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.StudentNote, 0, len(notes))
	for _, note := range notes {
		result = append(result, toPublicNote(note))
	}
	return result, nil
}

func (s *Service) CreateStudentNote(ctx context.Context, input peopledirectory.CreateStudentNote) (peopledirectory.StudentNote, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.StudentID <= 0 || input.AuthorAccountID <= 0 {
		return peopledirectory.StudentNote{}, peopledirectory.ErrStudentNoteInvalid
	}

	note, err := s.studentNotes.Create(ctx, domain.CreateStudentNote{
		StudentID: input.StudentID, AuthorAccountID: input.AuthorAccountID,
		Kind: input.Kind, Visibility: input.Visibility, Category: input.Category,
		Body: input.Body, Subject: toDomainNoteSubject(input.Subject), RevalidateSubject: input.RevalidateSubject,
	})
	if err != nil {
		return peopledirectory.StudentNote{}, MapError(err)
	}
	return toPublicNote(note), nil
}

func (s *Service) UpdateStudentNote(ctx context.Context, input peopledirectory.UpdateStudentNote) (peopledirectory.StudentNote, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.ID <= 0 || input.StudentID <= 0 || input.ActorAccountID <= 0 {
		return peopledirectory.StudentNote{}, peopledirectory.ErrStudentNoteInvalid
	}

	note, err := s.studentNotes.Update(ctx, domain.UpdateStudentNote{
		ID: input.ID, StudentID: input.StudentID, ActorAccountID: input.ActorAccountID,
		Kind: input.Kind, Visibility: input.Visibility, Category: input.Category, Body: input.Body,
		SubjectDate: input.SubjectDate,
	})
	if err != nil {
		return peopledirectory.StudentNote{}, MapError(err)
	}
	return toPublicNote(note), nil
}

func (s *Service) DeleteStudentNote(ctx context.Context, input peopledirectory.DeleteStudentNote) error {
	if input.ID <= 0 || input.StudentID <= 0 || input.ActorAccountID <= 0 {
		return peopledirectory.ErrStudentNoteInvalid
	}

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
	return MapError(s.studentNotes.Delete(ctx, domain.DeleteStudentNote{
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
