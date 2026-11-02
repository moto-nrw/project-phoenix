package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentNoteDependencies are the People Directory note seams of moto
// schule. The Lehrkraft's day hint (#4795): without DayHints the day-hint
// capabilities answer classday.ErrDayHintsNotConfigured and the day report
// carries no hints; Today defaults to the Berlin calendar day. The notes the
// OGS released for the school (#4797): People Directory's SharedNotes and
// School Structure's ClassTeachers; without both the shared-notes capability
// answers classday.ErrSharedNotesNotConfigured and no class offers the view.
type StudentNoteDependencies struct {
	DayHints      DayHints
	Today         func() calendar.Date
	SharedNotes   SharedNoteReader
	ClassTeachers ClassTeacherAssignments
}

// dayHints is the Lehrkraft's day hint for the OGS (#4795): a short message
// for today, for an assigned class or for one of its children. The hints are
// People Directory's dated notes with origin school; this seam adds what the
// school portal decides: the school's setting, today only, and that the class
// hint reaches exactly the children of the class that day. The class
// assignment itself is checked by the HTTP adapter through ResolveClass, as
// for the arrival exceptions.
type dayHints struct {
	store     DayHints
	scope     DayHintWriteScope
	rosters   DayRosters
	today     func() calendar.Date
	announcer ArrivalScheduleAnnouncer
}

// writeOpen applies operations.school_portal_write_scope: without the ticked
// day-hint right every write is refused with classday.ErrDayHintsClosed.
func (h dayHints) writeOpen(ctx context.Context) error {
	open, err := h.scopeOpen(ctx)
	if err != nil {
		return err
	}
	if !open {
		return classday.ErrDayHintsClosed
	}
	return nil
}

func (h dayHints) scopeOpen(ctx context.Context) (bool, error) {
	if h.store == nil {
		return false, classday.ErrDayHintsNotConfigured
	}
	if h.scope == nil {
		return false, errors.New("class day hints: settings not configured")
	}
	open, err := h.scope.SchoolMayWriteDayHints(ctx)
	if err != nil {
		return false, fmt.Errorf("class day hints: resolve school portal write scope: %w", err)
	}
	return open, nil
}

// SetDayHint writes the caller's hint for the class (StudentID 0) or for one
// child of the class. A class hint goes to every child of the class with an
// OGS record that day; a class-list-only entry has no note card to carry it.
func (h dayHints) SetDayHint(ctx context.Context, in classday.DayHintWrite) error {
	if err := h.writeOpen(ctx); err != nil {
		return err
	}
	// The length limit belongs to People Directory, which refuses a longer
	// hint; mapDayHintError turns that into classday.ErrDayHintInvalid.
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return classday.ErrDayHintInvalid
	}
	day, err := h.todayOnly(in.Date)
	if err != nil {
		return err
	}
	studentIDs, err := h.classStudentIDs(ctx, in.SchoolClass, day)
	if err != nil {
		return err
	}
	write := peopledirectory.SetSchoolDayHint{
		AuthorAccountID: in.AuthorAccountID, Date: day, Body: body,
	}
	if in.StudentID == 0 {
		if len(studentIDs) == 0 {
			return classday.ErrDayHintClassEmpty
		}
		write.SchoolClass = strings.TrimSpace(in.SchoolClass)
		write.StudentIDs = studentIDs
	} else {
		if !slices.Contains(studentIDs, in.StudentID) {
			return classday.ErrDayHintStudentNotInClass
		}
		write.StudentIDs = []int64{in.StudentID}
	}
	return h.announced(ctx, h.store.SetSchoolDayHint(ctx, write))
}

// ClearDayHint removes one of the caller's hints of today. A child hint is
// only reachable for a child of the class, so a teacher cannot reach past her
// class by naming another child.
func (h dayHints) ClearDayHint(ctx context.Context, target classday.DayHintTarget) error {
	if err := h.writeOpen(ctx); err != nil {
		return err
	}
	day, err := h.todayOnly(target.Date)
	if err != nil {
		return err
	}
	removal := peopledirectory.ClearSchoolDayHint{AuthorAccountID: target.AuthorAccountID, Date: day}
	if target.StudentID == 0 {
		removal.SchoolClass = strings.TrimSpace(target.SchoolClass)
	} else {
		studentIDs, err := h.classStudentIDs(ctx, target.SchoolClass, day)
		if err != nil {
			return err
		}
		if !slices.Contains(studentIDs, target.StudentID) {
			return classday.ErrDayHintStudentNotInClass
		}
		removal.StudentID = target.StudentID
	}
	return h.announced(ctx, h.store.ClearSchoolDayHint(ctx, removal))
}

// announced maps the owner's error and, after a successful write, tells the
// open OGS views to refetch. The hint rides on the arrival-schedule event
// like the class trip: it is tenant-wide, carries no child, and already
// revalidates Aufsicht, Gruppe and Kindersuche once the unit commits.
func (h dayHints) announced(ctx context.Context, err error) error {
	if err != nil {
		return mapDayHintError(err)
	}
	if h.announcer != nil {
		h.announcer.AnnounceArrivalScheduleChange(ctx)
	}
	return nil
}

// todayOnly parses the date and refuses every day but today.
func (h dayHints) todayOnly(date classday.Date) (calendar.Date, error) {
	day, err := parseDate(date)
	if err != nil {
		return "", err
	}
	if day != h.currentDay() {
		return "", classday.ErrDayHintNotToday
	}
	return day, nil
}

func (h dayHints) currentDay() calendar.Date {
	if h.today != nil {
		return h.today()
	}
	return calendar.TodayDate()
}

// classStudentIDs returns the children of the class with an OGS record on the
// day, from the same roster the day view shows.
func (h dayHints) classStudentIDs(ctx context.Context, schoolClass string, day calendar.Date) ([]int64, error) {
	if h.rosters == nil {
		return nil, classday.ErrDayHintsNotConfigured
	}
	roster, err := h.rosters.ClassRosterDay(ctx, schoolClass, day)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(roster.Rows))
	for _, row := range roster.Rows {
		if !row.ListEntry && row.StudentID > 0 {
			ids = append(ids, row.StudentID)
		}
	}
	return ids, nil
}

// attachDayHints puts the day's hints on the report: the hints for one child
// on its row, the hints for the whole class once on the report. A class hint
// is one row per child; the report shows it once per author.
func (h dayHints) attachDayHints(ctx context.Context, report *classday.DayReport, day calendar.Date, actorAccountID int64) error {
	if h.store == nil || report == nil {
		return nil
	}
	// A hint is written for today only, and only when the school opened it.
	// The HTTP adapter adds the caller's permission to this verdict.
	if day == h.currentDay() {
		open, err := h.scopeOpen(ctx)
		if err != nil {
			return err
		}
		report.CanWriteDayHints = open
	}
	rowIndex, studentIDs := reportStudentRows(report)
	if len(studentIDs) == 0 {
		return nil
	}
	hints, err := h.store.ListSchoolDayHints(ctx, day, studentIDs)
	if err != nil {
		return fmt.Errorf("class day report: day hints: %w", err)
	}
	placeDayHints(report, rowIndex, hints, actorAccountID)
	return nil
}

// reportStudentRows indexes the rows of children with an OGS record; a
// class-list-only entry has no note card and therefore no hint.
func reportStudentRows(report *classday.DayReport) (map[int64]int, []int64) {
	rowIndex := make(map[int64]int, len(report.Rows))
	studentIDs := make([]int64, 0, len(report.Rows))
	for index, row := range report.Rows {
		if row.ListEntry || row.StudentID <= 0 {
			continue
		}
		rowIndex[row.StudentID] = index
		studentIDs = append(studentIDs, row.StudentID)
	}
	return rowIndex, studentIDs
}

// placeDayHints puts a hint for one child on its row and a hint for the
// class once on the report.
func placeDayHints(report *classday.DayReport, rowIndex map[int64]int, hints []peopledirectory.SchoolDayHint, actorAccountID int64) {
	classKey := schoolstructure.NormalizeClass(report.SchoolClass)
	seenClassHints := map[int64]bool{}
	for _, hint := range hints {
		view := toDayHint(hint, actorAccountID)
		if hint.SchoolClass == "" {
			if index, ok := rowIndex[hint.StudentID]; ok {
				report.Rows[index].DayHints = append(report.Rows[index].DayHints, view)
			}
			continue
		}
		key := authorKey(hint.AuthorAccountID, hint.ID)
		// A class hint written for another class reaches a child that has
		// changed class since; the report of this class does not repeat it.
		if schoolstructure.NormalizeClass(hint.SchoolClass) != classKey || seenClassHints[key] {
			continue
		}
		seenClassHints[key] = true
		report.ClassDayHints = append(report.ClassDayHints, view)
	}
}

// authorKey groups the rows of one class hint by their author. A hint whose
// author account is gone has no group and stands for itself.
func authorKey(authorAccountID *int64, id int64) int64 {
	if authorAccountID != nil {
		return *authorAccountID
	}
	return -id
}

func toDayHint(hint peopledirectory.SchoolDayHint, actorAccountID int64) classday.DayHint {
	return classday.DayHint{
		Body:       hint.Body,
		AuthorName: hint.AuthorName,
		Own:        hint.AuthorAccountID != nil && *hint.AuthorAccountID == actorAccountID,
		UpdatedAt:  hint.UpdatedAt,
	}
}

// mapDayHintError translates People Directory's refusals into the
// projection's sentinels.
func mapDayHintError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, peopledirectory.ErrStudentNoteInvalid):
		return classday.ErrDayHintInvalid
	case errors.Is(err, peopledirectory.ErrStudentNoteNotFound):
		return classday.ErrDayHintNotFound
	case errors.Is(err, peopledirectory.ErrStudentNotFound):
		return classday.ErrDayHintStudentNotInClass
	default:
		return err
	}
}
