package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ---------------------------------------------------------------------------
// Staff work-schedule operations (issue #584: moved verbatim out of api/staff)
// ---------------------------------------------------------------------------

func (s *workSessionService) AssignScheduleTemplate(ctx context.Context, staff *StaffScheduleBinding, modelID int64) error {
	model, err := s.workModelRepo.FindByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	entries := modelEntriesToScheduleRows(model.Entries, model.RotationLength)
	anchor := model.RotationAnchorDate
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, entries, anchor); err != nil {
		return fmt.Errorf("write assigned schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	staffAnchor := anchor
	staff.RotationAnchorDate = &staffAnchor
	if err := s.staffRepo.BindSchedule(ctx, *staff); err != nil {
		return fmt.Errorf("bind template to staff: %w", err)
	}
	return nil
}

func (s *workSessionService) ApplyCustomScheduleRows(ctx context.Context, staff *StaffScheduleBinding, entries []*WorkScheduleRow, anchor, validFrom calendar.Date) error {
	// An omitted anchor keeps the staff-level one; the new version must be
	// stamped with that same effective anchor, or it would silently re-parity
	// once the staff anchor moves.
	effective := anchor
	if effective.IsZero() && staff.RotationAnchorDate != nil {
		effective = *staff.RotationAnchorDate
	}
	// First rotational schedule with no anchor uses its start day; a later
	// template assignment would otherwise re-parity historical A/B weeks and Saldo.
	if effective.IsZero() && isRotationalSchedule(entries) {
		effective = validFrom
		if effective.IsZero() {
			effective = calendar.DateFromTime(s.now())
		}
	}
	if err := s.scheduleRepo.ReplaceScheduleWithValidFrom(ctx, staff.ID, entries, effective, validFrom); err != nil {
		return fmt.Errorf("write custom schedule: %w", err)
	}

	staff.WorkTimeModelID = nil
	if !effective.IsZero() {
		staff.RotationAnchorDate = &effective
	}
	if err := s.staffRepo.BindSchedule(ctx, *staff); err != nil {
		return fmt.Errorf("unbind template: %w", err)
	}

	return nil
}

func (s *workSessionService) SaveCustomScheduleAsTemplate(ctx context.Context, staff *StaffScheduleBinding, name string, rotation int, anchor calendar.Date, entries []*WorkTimeTemplateEntry) error {
	if anchor.IsZero() {
		anchor = calendar.DateFromTime(s.now())
	}
	model := &WorkTimeTemplate{
		Name:               name,
		RotationLength:     rotation,
		RotationAnchorDate: anchor,
	}
	if err := s.workModelRepo.Create(ctx, model, entries); err != nil {
		return err
	}
	scheduleRows := modelEntriesToScheduleRows(entries, rotation)
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, scheduleRows, anchor); err != nil {
		return fmt.Errorf("write saved template schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	staff.RotationAnchorDate = &anchor
	if err := s.staffRepo.BindSchedule(ctx, *staff); err != nil {
		return fmt.Errorf("bind freshly created template: %w", err)
	}
	return nil
}

func (s *workSessionService) UpdateSchedule(ctx context.Context, staff *StaffScheduleBinding, in ScheduleUpdateInput) error {
	mode := in.Mode
	if mode == "" {
		// Backwards compatibility: missing mode + flat entries means the
		// caller still uses the single-week, no-rotation contract.
		mode = "custom"
		if in.RotationLength == 0 {
			in.RotationLength = 1
		}
	}

	var err error
	switch mode {
	case "template":
		if in.ModelID == nil || *in.ModelID == 0 {
			return scheduleValidationErrorf("model_id is required for mode=template")
		}
		err = s.AssignScheduleTemplate(ctx, staff, *in.ModelID)
	case "custom":
		err = s.applyCustomSchedule(ctx, staff, in)
	default:
		return scheduleValidationErrorf("invalid mode %q", mode)
	}
	if err != nil {
		return err
	}
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *workSessionService) applyCustomSchedule(ctx context.Context, staff *StaffScheduleBinding, in ScheduleUpdateInput) error {
	rotation := in.RotationLength
	if rotation == 0 {
		rotation = 1
	}
	if rotation < 1 || rotation > MaxScheduleRotation {
		return scheduleValidationErrorf("rotation_length must be between 1 and %d", MaxScheduleRotation)
	}

	anchor := calendar.Date("")
	if in.RotationAnchorDate != "" {
		parsed, err := calendar.ParseDate(in.RotationAnchorDate)
		if err != nil {
			return scheduleValidationErrorf("invalid rotation_anchor_date: %v", err)
		}
		anchor = parsed
	}

	entries, templateEntries, err := buildScheduleEntries(in.Entries, rotation)
	if err != nil {
		return err
	}

	if in.SaveAsTemplateName != "" {
		if err := s.SaveCustomScheduleAsTemplate(ctx, staff, in.SaveAsTemplateName, rotation, anchor, templateEntries); err != nil {
			return fmt.Errorf("save as template: %w", err)
		}
		return nil
	}

	return s.ApplyCustomScheduleRows(ctx, staff, entries, anchor, calendar.Date(in.ValidFrom))
}
