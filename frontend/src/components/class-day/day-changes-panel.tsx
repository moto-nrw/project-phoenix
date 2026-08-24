"use client";

// "Anders als sonst": der Abweichungsblock der Klassenansicht (#2294).
//
// Er steht über den Klassenkarten und listet klassenübergreifend nur die
// Kinder, bei denen der Tag vom üblichen Plan abweicht: gemeldete
// Abwesenheiten, Abmeldungen und geänderte Abholzeiten. Damit muss eine
// Lehrkraft nicht mehr drei volle Klassenlisten gegen ihr Kopfwissen
// abgleichen, um zu sehen, wer heute früher gehen darf.
//
// Der Block ist reine Anzeige: keine Knopf-Optik, kein Zeiger-Cursor, keine
// Hover-Reaktion. Was hier steht, entscheidet das OGS-Team, nicht die
// Lehrkraft (siehe .claude/rules/verstaendlichkeit.md).

import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { formatChatClockTime, formatDate } from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { schoolClassLabel } from "~/lib/school-class-label";
import type { ClassDayReport } from "~/lib/class-day-api";
import {
  collectDayChanges,
  describeDayChange,
  isReportedToday,
  type DayChange,
} from "./day-changes";
import { statusColor, statusLabel } from "./status-labels";

/**
 * "Heute 11:24 gemeldet" trennt die kurzfristige Meldung von der seit zwei
 * Wochen geplanten. Ältere Meldungen nennen nur das Datum: die Uhrzeit von
 * vorletzter Woche beantwortet keine Frage.
 */
function reportedLabel(change: DayChange, now: Date): string | null {
  if (!change.reportedAt) return null;
  if (isReportedToday(change.reportedAt, now)) {
    return `Heute ${formatChatClockTime(change.reportedAt)} gemeldet`;
  }
  return `${formatDate(change.reportedAt)} gemeldet`;
}

function ChangeRow({
  change,
  showClass,
  now,
}: Readonly<{ change: DayChange; showClass: boolean; now: Date }>) {
  const reported = reportedLabel(change, now);
  const fresh = isReportedToday(change.reportedAt, now);
  return (
    <li className="flex items-start justify-between gap-3 rounded-xl border border-gray-100 bg-white px-3 py-2.5">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {showClass ? `${schoolClassLabel(change.schoolClass)} · ` : ""}
          {change.row.last_name}, {change.row.first_name}
        </p>
        <p className="truncate text-xs text-gray-600">
          {describeDayChange(change, statusLabel)}
        </p>
        {reported ? (
          <p
            className={`truncate text-xs ${fresh ? "font-medium text-gray-700" : "text-gray-400"}`}
          >
            {reported}
          </p>
        ) : null}
      </div>
      <div className="shrink-0">
        {change.kind === "status" ? (
          <StatusDotBadge
            label={statusLabel(change.row.status ?? "")}
            color={statusColor(change.row.status ?? "")}
          />
        ) : (
          <StatusDotBadge
            label="Andere Abholzeit"
            color={LOCATION_COLORS.WARNING}
          />
        )}
      </div>
    </li>
  );
}

export interface DayChangesPanelProps {
  readonly classes: readonly string[];
  readonly reports: Readonly<Record<string, ClassDayReport>>;
  readonly dateISO: string;
  /** Nur für Tests: fixiert den Vergleichstag für "Heute gemeldet". */
  readonly now?: Date;
}

export function DayChangesPanel({
  classes,
  reports,
  dateISO,
  now,
}: DayChangesPanelProps) {
  // Kein Default-Prop: eine `new Date()` in der Signatur bricht die
  // referenzielle Gleichheit bei jedem Render (oxlint react-Regel).
  const at = now ?? new Date();
  const changes = collectDayChanges(classes, reports);
  const multipleClasses = classes.length > 1;

  return (
    <section className="mt-4 rounded-2xl border border-gray-200 bg-gray-50 p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-sm font-semibold text-gray-900">
          Anders als sonst
          {changes.length > 0 ? ` (${changes.length})` : ""}
        </h3>
        <p className="text-xs text-gray-500">{formatDate(dateISO)}</p>
      </div>
      <p className="mt-1 text-xs leading-5 text-gray-600">
        {changes.length > 0
          ? "Nur diese Kinder weichen an diesem Tag vom üblichen Plan ab. Die vollständigen Klassenlisten stehen darunter. Änderungen macht das OGS-Team."
          : "An diesem Tag weicht in Ihren Klassen nichts vom üblichen Plan ab."}
      </p>
      {changes.length > 0 ? (
        <ul className="mt-3 grid gap-1.5 lg:grid-cols-2">
          {changes.map((change) => (
            <ChangeRow
              key={change.key}
              change={change}
              showClass={multipleClasses}
              now={at}
            />
          ))}
        </ul>
      ) : null}
    </section>
  );
}
