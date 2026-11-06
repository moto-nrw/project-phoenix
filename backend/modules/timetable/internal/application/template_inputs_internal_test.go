package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type templateInputMockTx struct{}

func (templateInputMockTx) RunWrite(ctx context.Context, _ bool, fn func(context.Context) error) error {
	return fn(ctx)
}

// Return detached rules, as the production SQL store does.
type templateInputMockStore struct {
	internalports.Store
	class   string
	targets bool
}

func (s *templateInputMockStore) ListGroupTargets(context.Context, []int64) (map[int64][]timetable.GroupTarget, domain.OperationStats, error) {
	out := map[int64][]timetable.GroupTarget{}
	if s.targets {
		value := s.class
		out[7] = []timetable.GroupTarget{{TargetGroupType: "klasse", TargetSchoolClass: &value}}
	}
	return out, domain.OperationStats{}, nil
}
func (s *templateInputMockStore) ListTemplateRows(context.Context, *int64, string) ([]timetable.TemplateListRow, domain.OperationStats, error) {
	return []timetable.TemplateListRow{{TemplateID: 7}}, domain.OperationStats{}, nil
}
func (s *templateInputMockStore) ListTemplateCapacityOccurrences(_ context.Context, _ *int64, _ []int64, _ []timetable.TemplateCapacityPeriod, members map[int64][]int64) ([]timetable.TemplateCapacityOccurrence, domain.OperationStats, error) {
	return []timetable.TemplateCapacityOccurrence{{TemplateID: 7, OccurrenceDate: "2026-11-09", EnrollmentCount: len(members[7])}}, domain.OperationStats{}, nil
}

type templateInputMockDirectory struct {
	internalports.StudentDirectory
	class string
}

func (s *templateInputMockDirectory) ListEnrolledStudents(context.Context) ([]domain.TargetStudent, domain.OperationStats, error) {
	return []domain.TargetStudent{{ID: 42, SchoolClass: s.class}}, domain.OperationStats{}, nil
}
func templateInputMockService(store *templateInputMockStore, directory *templateInputMockDirectory) *Service {
	return &Service{store: store, students: directory, tx: templateInputMockTx{}, today: func() string { return "2026-11-05" }, observe: func(internalports.Observation) {}}
}

func TestTemplateInputsKeepCapacityFresh(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"normal", "zero-target", "dynamic-membership"} {
		t.Run(mode, func(t *testing.T) {
			store := &templateInputMockStore{class: "1a", targets: mode != "zero-target"}
			directory := &templateInputMockDirectory{class: "1a"}
			svc := templateInputMockService(store, directory)
			data := &timetableData{deps: ports.TimetableDataDependencies{Templates: svc, Directories: ports.RepositoryDirectories{
				RoomNames:  func(context.Context, []int64) (map[int64]string, error) { return map[int64]string{}, nil },
				ShiftTypes: func(context.Context) ([]ports.ShiftType, error) { return nil, nil },
				GroupNames: func(context.Context, []int64) (map[int64]string, error) { return map[int64]string{}, nil },
				ActivePeriods: func(context.Context) ([]timetable.TemplateCapacityPeriod, error) {
					if mode == "dynamic-membership" {
						directory.class = "2a"
					}
					return nil, nil
				},
			}}}
			entries, err := data.ListTemplateEntries(context.Background(), nil, 10)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if mode != "normal" {
				want = 0
			}
			t.Logf("mode=%s display=%d capacity=%d", mode, entries[0].EnrollmentCount, entries[0].CapacityEnrollmentCount)
			if entries[0].CapacityEnrollmentCount != want {
				t.Errorf("parent capacity=%d; got %d", want, entries[0].CapacityEnrollmentCount)
			}
		})
	}
}
