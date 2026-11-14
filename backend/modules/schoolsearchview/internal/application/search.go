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
// transaction and returns bounded lists.
type Reader interface {
	// Search reads every kind the scope opens in one statement: children by
	// name, class or group and with ChildDetails also by their parent note
	// (Elternnotizen); contact persons of a child at the school by name or
	// e-mail address, or by the phone key and its alternative reading when
	// phoneKey is not empty; staff, rooms and groups by name.
	Search(ctx context.Context, text, phoneKey, alternativeKey string, scope schoolsearchview.Scope) (schoolsearchview.Result, error)
	// ChildrenByID reads the first limit of the given children that are
	// still at the school, in the order of ids.
	ChildrenByID(ctx context.Context, ids []int64, limit int) ([]schoolsearchview.Child, error)
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

// SearchSchool answers one query within its Scope. Contact persons match a
// phone query by its key; every other field compares the text as typed, so
// digits still find a room "1012" or a parent note that names a number.
func (s *Search) SearchSchool(ctx context.Context, query schoolsearchview.Query) (schoolsearchview.Result, error) {
	empty := schoolsearchview.Result{
		Children: []schoolsearchview.Child{}, Contacts: []schoolsearchview.Contact{},
		Staff: []schoolsearchview.Staff{}, Rooms: []schoolsearchview.Room{}, Groups: []schoolsearchview.Group{},
		Scope: query.Scope,
	}
	text, err := schoolsearchview.ValidateQueryText(query.Text)
	if err != nil {
		return empty, err
	}
	scope := query.Scope
	result, err := s.reader.Search(ctx, text, schoolsearchview.PhoneSearchKey(text), schoolsearchview.PhoneSearchAlternativeKey(text), scope)
	if err != nil {
		return empty, fmt.Errorf("school search: %w", err)
	}
	result.Scope = scope
	withNoteSnippets(result.Children, text)
	if scope.Children && scope.ChildDetails {
		result.Children, err = s.withAnswerHits(ctx, text, result.Children)
		if err != nil {
			return empty, err
		}
	}
	return result, nil
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
		row.Match = schoolsearchview.Match{Field: schoolsearchview.MatchEnrollmentAnswer, Label: hit.Label, Text: matchSnippet(hit.Answer, text)}
		children = append(children, row)
	}
	return children, nil
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
