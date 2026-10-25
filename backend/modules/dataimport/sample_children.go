package dataimport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Sample children of a test Träger (#4658, spec #4611 user story 125): the
// moto team fills the test schools of a test Träger with invented children,
// so the new Träger can try everything without real personal data. The rows
// are built in, not uploaded, and go through the student import into each
// test school's own tenant transaction. Running it again creates nothing
// twice: an existing child is skipped like a duplicate row of a file.

// ErrNotTestTraeger reports a Träger without the test mark. Sample children
// never enter the schools of a real Träger.
var ErrNotTestTraeger = errors.New("data import: the Träger is not a test Träger")

// ErrTestTraegerInactive reports a deactivated or deleted test Träger.
var ErrTestTraegerInactive = errors.New("data import: the test Träger is deactivated or deleted")

// ErrTestTraegerNotFound reports an unknown Träger.
var ErrTestTraegerNotFound = errors.New("data import: the Träger does not exist")

// ErrSampleAuditUnavailable reports that the operator audit could not record
// the request; no child is written then.
var ErrSampleAuditUnavailable = errors.New("data import: the operator audit is unavailable")

// TestTraegerSchools records the operator's request in the operator audit
// and names the test schools of the test Träger. The root binds it to
// Organisation & Tenancy and translates its refusals into ErrNotTestTraeger,
// ErrTestTraegerInactive, ErrTestTraegerNotFound and
// ErrSampleAuditUnavailable.
type TestTraegerSchools interface {
	RecordSampleChildrenRequest(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) ([]int64, error)
}

// SampleSchoolResult is the outcome in one test school: children created and
// rows skipped because the child already exists.
type SampleSchoolResult struct {
	SchoolID     int64 `json:"school_id"`
	CreatedCount int   `json:"created_count"`
	SkippedCount int   `json:"skipped_count"`
}

// SampleChildrenResult is the outcome over every test school of the Träger.
type SampleChildrenResult struct {
	Schools      []SampleSchoolResult `json:"schools"`
	CreatedCount int                  `json:"created_count"`
}

// SampleChildren fills the test schools of a test Träger with invented
// children.
type SampleChildren interface {
	FillTestTraeger(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) (SampleChildrenResult, error)
}

// The sample children of a test school: invented combinations of common
// first and last names with invented birthdays. They carry no guardian, no
// address, no health note and no contact data.

var sampleFirstNames = []string{
	"Anna", "Ben", "Clara", "David", "Ella", "Finn", "Greta", "Henri",
	"Ida", "Jonas", "Klara", "Leon", "Mia", "Noah", "Oskar", "Paula",
	"Quentin", "Romy", "Samuel", "Tilda", "Uwe", "Vera", "Willi", "Zoe",
}

var sampleLastNames = []string{
	"Ahorn", "Birkenfeld", "Dachs", "Eichler", "Fuchsberg", "Gänseblum",
	"Haselnuss", "Igelhof", "Kastanie", "Lerchenfeld", "Mohnblum", "Nussbaum",
}

// sampleClasses are the school classes of every test school; each gets
// sampleChildrenPerClass children.
var sampleClasses = []struct {
	name  string
	grade int
}{{"1a", 1}, {"2a", 2}, {"3a", 3}, {"4a", 4}}

const (
	sampleChildrenPerClass = 6
	// sampleRetentionDays is the default retention of a child's visit data
	// (Student Presence's DefaultPrivacyConsentRetentionDays).
	sampleRetentionDays = 30
)

// sampleChildren fills the test schools of a test Träger through the student
// import, one tenant transaction per school.
type sampleChildren struct {
	schools      TestTraegerSchools
	students     RowImporter[StudentImportRow]
	withinSchool func(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	today        func() calendar.Date
}

// NewSampleChildren builds the sample children workflow. withinSchool opens
// the tenant transaction of one school.
func NewSampleChildren(
	schools TestTraegerSchools, students RowImporter[StudentImportRow],
	withinSchool func(context.Context, int64, func(context.Context) error) error,
	today func() calendar.Date,
) SampleChildren {
	if schools == nil || students == nil || withinSchool == nil || today == nil {
		panic("data import: sample children need the test schools, the student import, the school transaction and the clock")
	}
	return &sampleChildren{schools: schools, students: students, withinSchool: withinSchool, today: today}
}

// FillTestTraeger has the request recorded in the operator audit and then
// imports the sample children into every test school. A child that already
// exists is skipped, so a second run after a failure only adds what is
// missing.
func (s *sampleChildren) FillTestTraeger(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) (SampleChildrenResult, error) {
	result := SampleChildrenResult{Schools: []SampleSchoolResult{}}
	schoolIDs, err := s.schools.RecordSampleChildrenRequest(ctx, organizationID, operatorID, clientIP)
	if err != nil {
		return result, err
	}
	today := s.today()
	for index, schoolID := range schoolIDs {
		rows := SampleChildRows(index, today)
		var imported *ImportResult[StudentImportRow]
		err := s.withinSchool(ctx, schoolID, func(txCtx context.Context) error {
			var err error
			imported, err = s.students.Import(txCtx, ImportRequest[StudentImportRow]{
				Rows: rows, Mode: ImportModeCreate, SkipInvalidRows: true,
			})
			return err
		})
		if err != nil {
			return result, fmt.Errorf("import sample children into school %d: %w", schoolID, err)
		}
		result.Schools = append(result.Schools, SampleSchoolResult{
			SchoolID: schoolID, CreatedCount: imported.CreatedCount, SkippedCount: imported.ErrorCount,
		})
		result.CreatedCount += imported.CreatedCount
	}
	return result, nil
}

// SampleChildRows are the sample children of the test school at index: every
// class with its children, named so that no name repeats within a school and
// the schools differ. A child is as old as its grade suggests on today.
func SampleChildRows(index int, today calendar.Date) []StudentImportRow {
	rows := make([]StudentImportRow, 0, len(sampleClasses)*sampleChildrenPerClass)
	for classIndex, class := range sampleClasses {
		for seat := range sampleChildrenPerClass {
			number := classIndex*sampleChildrenPerClass + seat
			birthYear := today.Year() - 5 - class.grade
			rows = append(rows, StudentImportRow{
				FirstName:         sampleFirstNames[(number+index*7)%len(sampleFirstNames)],
				LastName:          sampleLastNames[(number+index*5)%len(sampleLastNames)],
				Birthday:          calendar.NewDate(birthYear, time.Month(number%12+1), number*5%28+1).String(),
				SchoolClass:       class.name,
				DataRetentionDays: sampleRetentionDays,
			})
		}
	}
	return rows
}
