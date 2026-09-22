// Package timerecords holds the retained Workforce time-tracking rows and
// repository contracts (work sessions, breaks, staff absences, balance
// adjustments, vacation quotas and openings) that the retained time-tracking
// services in modules/workforce/legacy still speak (#3422). The compatibility
// adapters in modules/workforce/legacy serve them on top of the Workforce
// capability; persistence is native in modules/workforce/internal/adapters/postgres.
// Status, type and source values are the public workforce constants. Every
// entry goes with the last retained service that uses it.
package timerecords
