package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) RoomUtilization(ctx context.Context, windows []ports.StudentVisitWindow) ([]ports.RoomUtilization, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	if len(windows) == 0 {
		return []ports.RoomUtilization{}, ports.Stats{}, nil
	}
	input, err := json.Marshal(windows)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	const query = `WITH eligible_student AS (
 SELECT * FROM jsonb_to_recordset(?::jsonb) AS eligibility(student_id bigint, start_at timestamptz, end_at timestamptz)
),
retention AS (
	SELECT "privacy_consent".student_id,
	       MIN("privacy_consent".data_retention_days) AS retention_days
	FROM users.privacy_consents AS "privacy_consent"
	WHERE "privacy_consent".accepted = TRUE AND "privacy_consent".tenant_id = ?
	GROUP BY "privacy_consent".student_id
),
clamped AS (
	SELECT ag.room_id,
	       v.student_id,
	       GREATEST(v.entry_time, es.start_at) AS entry_at,
	       LEAST(COALESCE(v.exit_time, NOW()), es.end_at) AS exit_at
	FROM active.visits v
	JOIN active.groups ag ON ag.id = v.active_group_id
	JOIN eligible_student es ON es.student_id = v.student_id
	JOIN retention r ON r.student_id = v.student_id
	WHERE v.entry_time < es.end_at
	  AND (v.exit_time IS NULL OR v.exit_time > es.start_at)
	  AND v.created_at >= NOW() - make_interval(days => r.retention_days)
	  AND v.tenant_id = ? AND ag.tenant_id = ?
),
scoped AS (
	-- A clamp can leave nothing: care that ended before the window start, or
	-- an enrollment starting after its end, pushes exit_at to or before
	-- entry_at. Such a visit is no presence in the window, so it must not
	-- count a child, a day, or a peak — an exit sorted before its own entry
	-- would leave the sweep permanently one child too high.
	SELECT room_id, student_id, entry_at, exit_at
	FROM clamped
	WHERE exit_at > entry_at
),
events AS (
	SELECT room_id, entry_at AS at, 1 AS delta FROM scoped
	UNION ALL
	SELECT room_id, exit_at AS at, -1 AS delta FROM scoped
),
running AS (
	SELECT room_id, SUM(delta) OVER (PARTITION BY room_id ORDER BY at, delta) AS occupancy
	FROM events
),
peak AS (
	SELECT room_id, MAX(occupancy) AS peak_occupancy FROM running GROUP BY room_id
),
room_days AS (
	SELECT s.room_id,
	       COUNT(DISTINCT d.used_date) AS days_used
	FROM scoped s
	CROSS JOIN LATERAL generate_series(
		(s.entry_at AT TIME ZONE 'Europe/Berlin')::date,
		((s.exit_at - INTERVAL '1 microsecond') AT TIME ZONE 'Europe/Berlin')::date,
		INTERVAL '1 day'
	) AS d(used_date)
	GROUP BY s.room_id
)
SELECT s.room_id,
	   COALESCE(d.days_used, 0)::int AS days_used,
       COUNT(DISTINCT s.student_id) AS distinct_students,
       COALESCE(SUM(EXTRACT(EPOCH FROM (s.exit_at - s.entry_at))) / 60, 0)::int AS student_minutes,
       COALESCE(p.peak_occupancy, 0) AS peak_occupancy
FROM scoped s
LEFT JOIN peak p ON p.room_id = s.room_id
LEFT JOIN room_days d ON d.room_id = s.room_id
GROUP BY s.room_id, d.days_used, p.peak_occupancy
ORDER BY s.room_id`
	var rows []ports.RoomUtilization
	started := time.Now()
	err = db.NewRaw(query, string(input), tenantID, tenantID, tenantID).Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("statistics room utilization: %w", err)
	}
	return rows, stats, nil
}
