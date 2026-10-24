//go:build testhooks

package presenceprojectionhttp

import "net/http"

// StatusDayReadHandlerForTests lets mixed read/write suites mount the actual projection.
func (rs *Resource) StatusDayReadHandlerForTests() http.HandlerFunc { return rs.getStudentStatusDays }
