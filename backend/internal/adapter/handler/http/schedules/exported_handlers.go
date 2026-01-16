package schedules

import "net/http"

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// ListDateframesHandler returns the list dateframes handler
func (rs *Resource) ListDateframesHandler() http.HandlerFunc { return rs.listDateframes }

// GetDateframeHandler returns the get dateframe handler
func (rs *Resource) GetDateframeHandler() http.HandlerFunc { return rs.getDateframe }

// CreateDateframeHandler returns the create dateframe handler
func (rs *Resource) CreateDateframeHandler() http.HandlerFunc { return rs.createDateframe }

// UpdateDateframeHandler returns the update dateframe handler
func (rs *Resource) UpdateDateframeHandler() http.HandlerFunc { return rs.updateDateframe }

// DeleteDateframeHandler returns the delete dateframe handler
func (rs *Resource) DeleteDateframeHandler() http.HandlerFunc { return rs.deleteDateframe }

// GetDateframesByDateHandler returns the get dateframes by date handler
func (rs *Resource) GetDateframesByDateHandler() http.HandlerFunc { return rs.getDateframesByDate }

// GetOverlappingDateframesHandler returns the get overlapping dateframes handler
func (rs *Resource) GetOverlappingDateframesHandler() http.HandlerFunc {
	return rs.getOverlappingDateframes
}

// GetCurrentDateframeHandler returns the get current dateframe handler
func (rs *Resource) GetCurrentDateframeHandler() http.HandlerFunc { return rs.getCurrentDateframe }

// ListTimeframesHandler returns the list timeframes handler
func (rs *Resource) ListTimeframesHandler() http.HandlerFunc { return rs.listTimeframes }

// GetTimeframeHandler returns the get timeframe handler
func (rs *Resource) GetTimeframeHandler() http.HandlerFunc { return rs.getTimeframe }

// CreateTimeframeHandler returns the create timeframe handler
func (rs *Resource) CreateTimeframeHandler() http.HandlerFunc { return rs.createTimeframe }

// UpdateTimeframeHandler returns the update timeframe handler
func (rs *Resource) UpdateTimeframeHandler() http.HandlerFunc { return rs.updateTimeframe }

// DeleteTimeframeHandler returns the delete timeframe handler
func (rs *Resource) DeleteTimeframeHandler() http.HandlerFunc { return rs.deleteTimeframe }

// GetActiveTimeframesHandler returns the get active timeframes handler
func (rs *Resource) GetActiveTimeframesHandler() http.HandlerFunc { return rs.getActiveTimeframes }

// GetTimeframesByRangeHandler returns the get timeframes by range handler
func (rs *Resource) GetTimeframesByRangeHandler() http.HandlerFunc { return rs.getTimeframesByRange }

// ListRecurrenceRulesHandler returns the list recurrence rules handler
func (rs *Resource) ListRecurrenceRulesHandler() http.HandlerFunc { return rs.listRecurrenceRules }

// GetRecurrenceRuleHandler returns the get recurrence rule handler
func (rs *Resource) GetRecurrenceRuleHandler() http.HandlerFunc { return rs.getRecurrenceRule }

// CreateRecurrenceRuleHandler returns the create recurrence rule handler
func (rs *Resource) CreateRecurrenceRuleHandler() http.HandlerFunc { return rs.createRecurrenceRule }

// UpdateRecurrenceRuleHandler returns the update recurrence rule handler
func (rs *Resource) UpdateRecurrenceRuleHandler() http.HandlerFunc { return rs.updateRecurrenceRule }

// DeleteRecurrenceRuleHandler returns the delete recurrence rule handler
func (rs *Resource) DeleteRecurrenceRuleHandler() http.HandlerFunc { return rs.deleteRecurrenceRule }

// GetRecurrenceRulesByFrequencyHandler returns the get recurrence rules by frequency handler
func (rs *Resource) GetRecurrenceRulesByFrequencyHandler() http.HandlerFunc {
	return rs.getRecurrenceRulesByFrequency
}

// GetRecurrenceRulesByWeekdayHandler returns the get recurrence rules by weekday handler
func (rs *Resource) GetRecurrenceRulesByWeekdayHandler() http.HandlerFunc {
	return rs.getRecurrenceRulesByWeekday
}

// GenerateEventsHandler returns the generate events handler
func (rs *Resource) GenerateEventsHandler() http.HandlerFunc { return rs.generateEvents }

// CheckConflictHandler returns the check conflict handler
func (rs *Resource) CheckConflictHandler() http.HandlerFunc { return rs.checkConflict }

// FindAvailableSlotsHandler returns the find available slots handler
func (rs *Resource) FindAvailableSlotsHandler() http.HandlerFunc { return rs.findAvailableSlots }
