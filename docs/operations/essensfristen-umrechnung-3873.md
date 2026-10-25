# Essensfristen: Werktage werden Kalendertage (#3873)

Migration 1.15.529 rechnet den Vorlauf der Essensfristen von #4866 um.
Development zählt `operations.meal_registration_cutoff_lead_days` und
`operations.meal_cancellation_cutoff_lead_days` in Werktagen: Wochenende,
gesetzliche Feiertage und Schließtage zählen nicht mit. Die Bestellregeln
zählen Kalendertage. Die Migration speichert deshalb
`N + 2 * ceil(N / 5)` Kalendertage (1 → 3, 2 → 4, 5 → 7, 6 → 10, 10 → 14,
0 bleibt 0): so viele Kalendertage überspannen N Werktage höchstens über
Wochenenden. Ohne Feiertage oder Schließtage im Vorlauf endet keine Frist
später als vorher, manche früher. Down stellt N exakt wieder her.

Liegt ein Feiertag oder Schließtag im Vorlauf, endete die alte Frist noch
einen Werktag früher, die neue endet dann später. Die folgende Abfrage listet
die Schulen und Essenstage, für die das im nächsten Jahr gilt. Sie läuft
lesend auf dem Stand von development, **vor** der Migration. Gesetzliche
Feiertage berechnet moto im Code; die Abfrage enthält die Feiertage in
Nordrhein-Westfalen bis Ende 2027. Für Schulen anderer Bundesländer
(`operations.federal_state`) die Liste `holidays` ergänzen.

```sql
WITH holidays (region, day) AS (
	VALUES ('DE-NW', DATE '2026-11-01'), ('DE-NW', '2026-12-25'), ('DE-NW', '2026-12-26'),
		('DE-NW', '2027-01-01'), ('DE-NW', '2027-03-26'), ('DE-NW', '2027-03-29'),
		('DE-NW', '2027-05-01'), ('DE-NW', '2027-05-06'), ('DE-NW', '2027-05-17'),
		('DE-NW', '2027-05-27'), ('DE-NW', '2027-10-03'), ('DE-NW', '2027-11-01'),
		('DE-NW', '2027-12-25'), ('DE-NW', '2027-12-26')
),
leads AS (
	SELECT lead.tenant_id, lead.setting_key, (lead.value #>> '{}')::int AS working_days
	FROM config.setting_values AS lead
	WHERE jsonb_typeof(lead.value) = 'number' AND (lead.value #>> '{}')::int > 0
		AND (
			(lead.setting_key = 'operations.meal_registration_cutoff_lead_days'
				AND NOT EXISTS (SELECT 1 FROM config.setting_values AS weekday
					WHERE weekday.tenant_id = lead.tenant_id
						AND weekday.setting_key = 'operations.meal_registration_cutoff_weekday'
						AND weekday.value #>> '{}' IN ('1', '2', '3', '4', '5')))
			OR (lead.setting_key = 'operations.meal_cancellation_cutoff_lead_days'
				AND EXISTS (SELECT 1 FROM config.setting_values AS separate
					WHERE separate.tenant_id = lead.tenant_id
						AND separate.setting_key = 'operations.meal_cancellation_cutoff_separate'
						AND separate.value = 'true'::jsonb))
		)
),
schools AS (
	SELECT leads.*, (leads.working_days + 2 * ceil(leads.working_days / 5.0))::int AS calendar_days,
		COALESCE((SELECT state.value #>> '{}' FROM config.setting_values AS state
			WHERE state.tenant_id = leads.tenant_id AND state.setting_key = 'operations.federal_state'), 'DE-NW') AS region
	FROM leads
),
closed (tenant_id, day) AS (
	SELECT schools.tenant_id, holidays.day FROM schools JOIN holidays ON holidays.region = schools.region
	UNION
	SELECT closing.tenant_id, generate_series(closing.start_date, closing.end_date, INTERVAL '1 day')::date
	FROM schedule.closing_days AS closing
),
meal_days AS (
	SELECT schools.*, candidate::date AS meal_day
	FROM schools, generate_series(current_date, current_date + 365, INTERVAL '1 day') AS candidate
	WHERE extract(isodow FROM candidate) <= 5
		AND NOT EXISTS (SELECT 1 FROM closed WHERE closed.tenant_id = schools.tenant_id AND closed.day = candidate::date)
)
SELECT school.id AS tenant_id, school.name, meal_days.setting_key, meal_days.working_days,
	meal_days.calendar_days, meal_days.meal_day, counted.working_days_in_span
FROM meal_days
JOIN platform.schools AS school ON school.id = meal_days.tenant_id
CROSS JOIN LATERAL (
	SELECT count(*) AS working_days_in_span
	FROM generate_series(meal_days.meal_day - meal_days.calendar_days, meal_days.meal_day - 1, INTERVAL '1 day') AS span
	WHERE extract(isodow FROM span) <= 5
		AND NOT EXISTS (SELECT 1 FROM closed WHERE closed.tenant_id = meal_days.tenant_id AND closed.day = span::date)
) AS counted
WHERE counted.working_days_in_span < meal_days.working_days
ORDER BY school.name, meal_days.setting_key, meal_days.meal_day;
```

Eine Zeile heißt: an diesem Essenstag endet die Frist nach der Migration
später als vorher, weil im Vorlauf weniger als N Werktage liegen. Die Schule
kann den Vorlauf nach der Migration in den Fristen des Mittagessens erhöhen.
