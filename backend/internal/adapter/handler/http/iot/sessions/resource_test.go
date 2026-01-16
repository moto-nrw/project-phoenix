package sessions

import "net/http"

// StartSessionHandler returns the startActivitySession handler.
func (rs *Resource) StartSessionHandler() http.HandlerFunc { return rs.startActivitySession }

// EndSessionHandler returns the endActivitySession handler.
func (rs *Resource) EndSessionHandler() http.HandlerFunc { return rs.endActivitySession }

// GetCurrentSessionHandler returns the getCurrentSession handler.
func (rs *Resource) GetCurrentSessionHandler() http.HandlerFunc { return rs.getCurrentSession }

// CheckConflictHandler returns the checkSessionConflict handler.
func (rs *Resource) CheckConflictHandler() http.HandlerFunc { return rs.checkSessionConflict }

// UpdateSupervisorsHandler returns the updateSessionSupervisors handler.
func (rs *Resource) UpdateSupervisorsHandler() http.HandlerFunc { return rs.updateSessionSupervisors }

// ProcessTimeoutHandler returns the processSessionTimeout handler.
func (rs *Resource) ProcessTimeoutHandler() http.HandlerFunc { return rs.processSessionTimeout }

// GetTimeoutConfigHandler returns the getSessionTimeoutConfig handler.
func (rs *Resource) GetTimeoutConfigHandler() http.HandlerFunc { return rs.getSessionTimeoutConfig }

// UpdateActivityHandler returns the updateSessionActivity handler.
func (rs *Resource) UpdateActivityHandler() http.HandlerFunc { return rs.updateSessionActivity }

// ValidateTimeoutHandler returns the validateSessionTimeout handler.
func (rs *Resource) ValidateTimeoutHandler() http.HandlerFunc { return rs.validateSessionTimeout }

// GetTimeoutInfoHandler returns the getSessionTimeoutInfo handler.
func (rs *Resource) GetTimeoutInfoHandler() http.HandlerFunc { return rs.getSessionTimeoutInfo }
