// Package application runs one school search (ADR 0231): it reads the query
// once, asks the reader only for the kinds the Scope opens and joins the
// registration answers Enrollment matched to the child rows.
package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
)

// Reader is the projection's SQL side. Every read runs in the caller's tenant
// transaction and returns at most limit rows.
type Reader interface {
	// Children finds children by name, class or group; with notes also by
	// their parent note (Elternnotizen).
	Children(ctx context.Context, text string, notes bool, limit int) ([]schoolsearchview.Child, error)
	// ChildrenByID reads the first limit of the given children that are
	// still at the school, in the order of ids.
	ChildrenByID(ctx context.Context, ids []int64, limit int) ([]schoolsearchview.Child, error)
	// Contacts finds contact persons of a child at the school by name or
	// e-mail address, or by the phone key when phoneKey is not empty.
	Contacts(ctx context.Context, text, phoneKey string, limit int) ([]schoolsearchview.Contact, error)
	Staff(ctx context.Context, text string, limit int) ([]schoolsearchview.Staff, error)
	Rooms(ctx context.Context, text string, limit int) ([]schoolsearchview.Room, error)
	Groups(ctx context.Context, text string, limit int) ([]schoolsearchview.Group, error)
}

// Search is the projection's search capability.
type Search struct {
	reader  Reader
	answers schoolsearchview.StudentAnswerSearch
}

// New builds the search; both ports are required.
func New(reader Reader, answers schoolsearchview.StudentAnswerSearch) (*Search, error) {
	if reader == nil || answers == nil {
		return nil, errors.New("school search: reader and answer search are required")
	}
	return &Search{reader: reader, answers: answers}, nil
}

// SearchSchool answers one query within its Scope.
func (s *Search) SearchSchool(ctx context.Context, query schoolsearchview.Query) (schoolsearchview.Result, error) {
	result := schoolsearchview.Result{
		Children: []schoolsearchview.Child{}, Contacts: []schoolsearchview.Contact{},
		Staff: []schoolsearchview.Staff{}, Rooms: []schoolsearchview.Room{}, Groups: []schoolsearchview.Group{},
		Scope: query.Scope,
	}
	text, err := schoolsearchview.ValidateQueryText(query.Text)
	if err != nil {
		return result, err
	}
	scope := query.Scope
	phoneKey := schoolsearchview.PhoneSearchKey(text)
	if err := s.searchPeople(ctx, text, phoneKey, scope, &result); err != nil {
		return result, err
	}
	return result, s.searchDirectory(ctx, text, scope, &result)
}

// searchPeople fills children and contacts. Contacts match a phone query by
// its key; every other field compares the text as typed, so digits still find
// a room "1012" or a parent note that names a number.
func (s *Search) searchPeople(ctx context.Context, text, phoneKey string, scope schoolsearchview.Scope, result *schoolsearchview.Result) error {
	if scope.Children {
		children, err := s.reader.Children(ctx, text, scope.ChildDetails, schoolsearchview.ChildLimit)
		if err != nil {
			return fmt.Errorf("school search: children: %w", err)
		}
		withNoteSnippets(children, text)
		if scope.ChildDetails {
			children, err = s.withAnswerHits(ctx, text, children)
			if err != nil {
				return err
			}
		}
		result.Children = children
	}
	if scope.Contacts {
		contacts, err := s.reader.Contacts(ctx, text, phoneKey, schoolsearchview.ContactLimit)
		if err != nil {
			return fmt.Errorf("school search: contacts: %w", err)
		}
		result.Contacts = contacts
	}
	return nil
}

// withAnswerHits appends the children found through a registration answer
// after the direct hits, up to the child limit. A child already found
// directly keeps its direct reason. Enrollment's hits may name children who
// have left the school, so the search asks for every hit and keeps the first
// ones still at the school, newest answers first: former pupils never leave a
// free place empty.
func (s *Search) withAnswerHits(ctx context.Context, text string, children []schoolsearchview.Child) ([]schoolsearchview.Child, error) {
	free := schoolsearchview.ChildLimit - len(children)
	if free <= 0 {
		return children, nil
	}
	hits, err := s.answers.SearchStudentAnswers(ctx, text, 0)
	if err != nil {
		return nil, fmt.Errorf("school search: registration answers: %w", err)
	}
	found := make(map[int64]bool, len(children))
	for _, child := range children {
		found[child.ID] = true
	}
	byStudent := make(map[int64]enrollment.StudentAnswerMatch, len(hits))
	ids := make([]int64, 0, len(hits))
	for _, hit := range hits {
		if found[hit.StudentID] {
			continue
		}
		if _, seen := byStudent[hit.StudentID]; seen {
			continue
		}
		byStudent[hit.StudentID] = hit
		ids = append(ids, hit.StudentID)
	}
	if len(ids) == 0 {
		return children, nil
	}
	rows, err := s.reader.ChildrenByID(ctx, ids, free)
	if err != nil {
		return nil, fmt.Errorf("school search: answer children: %w", err)
	}
	for _, row := range rows {
		hit := byStudent[row.ID]
		row.Match = schoolsearchview.Match{Field: schoolsearchview.MatchEnrollmentAnswer, Label: hit.Label, Text: hit.Answer}
		children = append(children, row)
	}
	return children, nil
}

// searchDirectory fills staff, rooms and groups.
func (s *Search) searchDirectory(ctx context.Context, text string, scope schoolsearchview.Scope, result *schoolsearchview.Result) error {
	if scope.Staff {
		staff, err := s.reader.Staff(ctx, text, schoolsearchview.OtherLimit)
		if err != nil {
			return fmt.Errorf("school search: staff: %w", err)
		}
		result.Staff = staff
	}
	if scope.Rooms {
		rooms, err := s.reader.Rooms(ctx, text, schoolsearchview.OtherLimit)
		if err != nil {
			return fmt.Errorf("school search: rooms: %w", err)
		}
		result.Rooms = rooms
	}
	if scope.Groups {
		groups, err := s.reader.Groups(ctx, text, schoolsearchview.OtherLimit)
		if err != nil {
			return fmt.Errorf("school search: groups: %w", err)
		}
		result.Groups = groups
	}
	return nil
}

// withNoteSnippets cuts the parent note of a note hit to the part around the
// match.
func withNoteSnippets(children []schoolsearchview.Child, text string) {
	for i := range children {
		if children[i].Match.Field == schoolsearchview.MatchParentNote {
			children[i].Match.Text = matchSnippet(children[i].Match.Text, text)
		}
	}
}
