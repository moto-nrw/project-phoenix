package users

import "errors"

// Validate ensures the master data row is storable.
func (m *StaffMasterData) Validate() error {
	if m.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if m.Gender != nil {
		switch *m.Gender {
		case GenderFemale, GenderMale, GenderDiverse:
			// valid
		default:
			return errors.New("gender must be 'female', 'male', or 'diverse'")
		}
	}
	if m.WeeklyHours != nil && (*m.WeeklyHours < 0 || *m.WeeklyHours > 80) {
		return errors.New("weekly_hours must be between 0 and 80")
	}
	if m.EntryDate != nil && m.ContractEndDate != nil && m.ContractEndDate.Before(*m.EntryDate) {
		return errors.New("contract_end_date must not be before entry_date")
	}
	if m.EntryDate != nil && m.ProbationEndDate != nil && m.ProbationEndDate.Before(*m.EntryDate) {
		return errors.New("probation_end_date must not be before entry_date")
	}
	return nil
}

// Validate ensures the qualification row is storable.
func (q *StaffQualification) Validate() error {
	if q.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if q.Name == "" {
		return errors.New("name is required")
	}
	if q.AcquiredOn != nil && q.ExpiresOn != nil && q.ExpiresOn.Before(*q.AcquiredOn) {
		return errors.New("expires_on must not be before acquired_on")
	}
	return nil
}

// Validate ensures the financial row is storable. Field syntax is validated
// in the service layer (the rules carry business meaning, not storage rules).
func (f *StaffFinancialData) Validate() error {
	if f.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	return nil
}
