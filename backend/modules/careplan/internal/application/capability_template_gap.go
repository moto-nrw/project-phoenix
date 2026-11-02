package application

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// maxTemplateGapAckClassLength bounds one class name; school classes are
// short labels such as "3a".
const maxTemplateGapAckClassLength = 50

// SetCareOfferingTemplateGapAck stores the school's decision on an offering
// without a matching Regeltermin (#5140) for one planning period. The
// classes are trimmed, deduplicated and sorted; "" stays for children
// without a class. An empty list withdraws the decision for that period.
func (s *Service) SetCareOfferingTemplateGapAck(ctx context.Context, offeringID, calendarPeriodID int64, schoolClasses []string) error {
	if offeringID <= 0 || calendarPeriodID <= 0 {
		return invalid(careplan.ErrInvalidCareOffering, "care offering and planning period IDs are required")
	}
	if len(schoolClasses) > careplan.MaxTemplateGapAckClasses {
		return invalid(careplan.ErrInvalidCareOffering, "too many school classes")
	}
	classes := make([]string, 0, len(schoolClasses))
	for _, class := range schoolClasses {
		class = strings.TrimSpace(class)
		if utf8.RuneCountInString(class) > maxTemplateGapAckClassLength {
			return invalid(careplan.ErrInvalidCareOffering, "school class is too long")
		}
		classes = append(classes, class)
	}
	slices.Sort(classes)
	classes = slices.Compact(classes)
	return s.run(ctx, "set_care_offering_template_gap_ack", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SetCareOfferingTemplateGapAck(ctx, offeringID, calendarPeriodID, classes)
		stats.Add(queryStats)
		return err
	})
}
