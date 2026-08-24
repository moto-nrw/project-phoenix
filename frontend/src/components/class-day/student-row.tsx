"use client";

// Die Kindzeile der Klassenansicht und ihr Abschnitt.
//
// Aus class-day-view.tsx herausgelöst (#2294), als die Ansicht in Übersicht
// und Klassenseite zerfiel. Beide Abschnitte einer Klassenseite rendern
// dieselbe Zeile; die Zeile ist der einzige Ort, an dem eine Abweichung vom
// üblichen Plan sichtbar wird.

import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type { ClassDayRow } from "~/lib/class-day-api";
import { reportedTodayLabel } from "./day-changes";
import { statusColor, statusLabel } from "./status-labels";

function rowDetailLine(row: ClassDayRow, enrollmentKnown: boolean): string {
  // Klassenlisteneintrag (#2382): "Keine Betreuung" ist die ganze Aussage —
  // das Badge rechts trägt sie, die Detailzeile bleibt leer (keine erfundene
  // Gehregel, keine Abholzeit).
  if (row.list_entry) return "";
  const parts: string[] = [];
  if (row.stays_today && row.offerings.length > 0) {
    parts.push(row.offerings.join(", "));
  }
  if (row.departure) parts.push(row.departure);
  // Ohne abdeckende Anmeldephase ist "keine OGS-Anmeldung" keine Aussage,
  // sondern genau das, was enrollment_known als unbekannt deklariert.
  if (enrollmentKnown && !row.registered) parts.push("keine OGS-Anmeldung");
  return parts.join(" · ");
}

export function StudentRow({
  row,
  enrollmentKnown,
  now,
}: {
  readonly row: ClassDayRow;
  readonly enrollmentKnown: boolean;
  readonly now: Date;
}) {
  const detail = rowDetailLine(row, enrollmentKnown);
  // Nur bei Meldungen von heute: dann ist der Zeitpunkt eine Neuigkeit.
  const reported = reportedTodayLabel(row.reported_at, now);
  // Eine abweichende Abholzeit ohne gemeldeten Status: der Status trägt
  // sonst das Kennzeichen und ist die stärkere Aussage.
  const showPickupBadge = Boolean(
    row.pickup_changed && !row.status && !row.list_entry,
  );
  return (
    // min-w-0: die Zeile ist ein Grid-Item, und Grid-Items haben
    // min-width:auto — ohne das schiebt ein langer Name die ganze Liste
    // auf schmalen Bildschirmen über den Rand.
    <li className="flex min-w-0 items-start justify-between gap-3 rounded-xl border border-gray-100 bg-white px-3 py-2.5">
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-gray-900">
          {row.last_name}, {row.first_name}
        </p>
        {detail ? (
          <p className="truncate text-xs text-gray-500">{detail}</p>
        ) : null}
        {reported ? (
          <p className="truncate text-xs font-medium text-gray-700">
            {reported}
          </p>
        ) : null}
      </div>
      {/* Auf schmalen Bildschirmen stapeln sich Zeit und Kennzeichen, statt
          die Zeile über den Rand zu schieben; ab sm stehen sie nebeneinander. */}
      <div className="flex shrink-0 flex-col items-end gap-1 sm:flex-row sm:items-start sm:gap-2">
        {/* "bis HH:MM" für Kinder, die bleiben — bei Heimgehern und
            Abgemeldeten wäre die Abholzeit irreführend. Ausnahme: eine vom
            üblichen Plan abweichende Abholzeit wird immer genannt, sonst
            trüge die Zeile das Kennzeichen "Andere Abholzeit" ohne die Zeit,
            um die es geht. Die Regelzeit steht darunter: ohne sie liest sich
            "bis 12:15" wie der Normalfall. */}
        {!row.status &&
        (row.stays_today || row.pickup_changed) &&
        row.pickup ? (
          <span className="text-right">
            <span className="block text-xs font-medium text-gray-600 tabular-nums">
              bis {row.pickup}
            </span>
            {row.pickup_changed && row.pickup_regular ? (
              <span className="block text-xs text-gray-400 tabular-nums">
                sonst {row.pickup_regular}
              </span>
            ) : null}
          </span>
        ) : null}
        {showPickupBadge ? (
          <StatusDotBadge
            label="Andere Abholzeit"
            color={LOCATION_COLORS.WARNING}
          />
        ) : null}
        {/* Klassenlisteneintrag (#2382): Kind ohne OGS-Datensatz — eindeutig
            als "Keine Betreuung" gekennzeichnet, unabhängig von Anmeldephasen. */}
        {row.list_entry ? (
          <StatusDotBadge
            label="Keine Betreuung"
            color={LOCATION_COLORS.HOME}
          />
        ) : null}
        {row.status ? (
          <StatusDotBadge
            label={statusLabel(row.status)}
            color={statusColor(row.status)}
          />
        ) : null}
      </div>
    </li>
  );
}

export function Section({
  title,
  count,
  accent = "text-gray-500",
  rows,
  enrollmentKnown = true,
  now,
}: Readonly<{
  title: string;
  count: number;
  accent?: string;
  rows: ClassDayRow[];
  enrollmentKnown?: boolean;
  now: Date;
}>) {
  if (rows.length === 0) return null;
  return (
    <div>
      <h3
        className={`mb-2 text-xs font-semibold tracking-wide uppercase ${accent}`}
      >
        {title} ({count})
      </h3>
      {/* Zweispaltig ab lg, damit eine volle Klasse nicht zu einer langen
          schmalen Liste wird. */}
      <ul className="grid gap-1.5 lg:grid-cols-2">
        {rows.map((row) => (
          <StudentRow
            // Klassenlisteneinträge haben student_id 0 — eigener Key-Raum.
            key={row.list_entry ? `entry-${row.list_entry_id}` : row.student_id}
            row={row}
            enrollmentKnown={enrollmentKnown}
            now={now}
          />
        ))}
      </ul>
    </div>
  );
}
