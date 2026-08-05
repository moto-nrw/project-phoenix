"use client";

import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { BirthdayCelebration } from "~/lib/birthdays-api";

/**
 * The dashboard birthday display (#1542).
 *
 * Deliberately a list of names rather than a counter: "2 Geburtstage" tells
 * nobody who to congratulate, which is the entire purpose of the card.
 *
 * Two rules the first draft got wrong and this one enforces:
 *
 *  1. Every row states WHEN the birthday was. A Monday card carries Saturday
 *     and Sunday along, and "Samstag" alone still leaves the reader counting
 *     backwards, so the badge names the day AND the date.
 *  2. The badge means one thing only. It answers "wann", never "wer" —
 *     children and staff are separated by a section heading instead, so a
 *     green pill cannot mean "today" in one row and "colleague" in the next.
 *
 * What may appear here is decided in the backend (school settings + personal
 * opt-out); this component renders whatever it is handed.
 */
export function BirthdayList({
  celebrations,
  isLoading,
}: {
  readonly celebrations: readonly BirthdayCelebration[];
  readonly isLoading: boolean;
}) {
  if (isLoading) {
    return (
      <div className="space-y-2">
        {[1, 2].map((i) => (
          <div key={i} className="h-12 animate-pulse rounded-xl bg-gray-100" />
        ))}
      </div>
    );
  }

  if (celebrations.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-gray-500">
        Heute keine Geburtstage
      </p>
    );
  }

  const children = celebrations.filter((entry) => entry.kind === "student");
  const staff = celebrations.filter((entry) => entry.kind === "staff");
  // Headings only earn their space once both populations are present. With
  // children alone (the common case) a lone "KINDER" label is pure noise.
  const showHeadings = children.length > 0 && staff.length > 0;

  return (
    <div className="space-y-4">
      {children.length > 0 && (
        <BirthdaySection
          label="Kinder"
          showLabel={showHeadings}
          celebrations={children}
        />
      )}
      {staff.length > 0 && (
        <BirthdaySection
          label="Team"
          showLabel={showHeadings}
          celebrations={staff}
        />
      )}
    </div>
  );
}

function BirthdaySection({
  label,
  showLabel,
  celebrations,
}: {
  readonly label: string;
  readonly showLabel: boolean;
  readonly celebrations: readonly BirthdayCelebration[];
}) {
  return (
    <div>
      {showLabel && (
        <h4 className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {label}
        </h4>
      )}
      <ul className="grid gap-2 sm:grid-cols-2">
        {celebrations.map((celebration) => (
          <li
            key={`${celebration.kind}-${celebration.id}`}
            className="flex items-center justify-between gap-3 rounded-xl border border-gray-200/50 bg-gray-50/50 p-3"
          >
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-gray-900">
                {celebration.name}
              </p>
              {describeCelebration(celebration) && (
                <p className="truncate text-xs text-gray-500">
                  {describeCelebration(celebration)}
                </p>
              )}
            </div>
            <StatusBadge
              label={whenLabel(celebration)}
              tone={celebration.isToday ? "green" : "gray"}
              title={formatDate(celebration.date)}
            />
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * The secondary line: where the child belongs and which birthday it is. Staff
 * entries carry neither — publishing a colleague's age is exactly what the
 * personal opt-out exists to prevent, and their section heading already says
 * who they are — so their line is omitted rather than filled with a label.
 */
function describeCelebration(celebration: BirthdayCelebration): string {
  if (celebration.kind === "staff") return "";

  const parts: string[] = [];
  if (celebration.groupName) parts.push(celebration.groupName);
  // The stored class already reads like a label ("Klasse 1a" as well as "1a"
  // depending on the school), so it is printed verbatim — prefixing "Klasse"
  // produced "Klasse Klasse 1a" on real data.
  if (celebration.schoolClass) parts.push(celebration.schoolClass);
  if (celebration.age && celebration.age > 0) {
    parts.push(`wird ${celebration.age}`);
  }
  return parts.join(" · ");
}

const SHORT_WEEKDAYS = ["So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"] as const;

/**
 * When the birthday was: "Heute", otherwise the weekday plus its date
 * ("Sa, 01.08."). The date is what makes a Monday card readable — the weekday
 * alone still leaves the reader working out which Saturday is meant.
 */
function whenLabel(celebration: BirthdayCelebration): string {
  if (celebration.isToday) return "Heute";

  const parsed = parseISOParts(celebration.date);
  if (!parsed) return formatDate(celebration.date);

  const { year, month, day } = parsed;
  // Local midnight, never `new Date("YYYY-MM-DD")` — that is UTC midnight and
  // shifts the weekday for Berlin.
  const weekday = SHORT_WEEKDAYS[new Date(year, month - 1, day).getDay()];
  const dayMonth = `${String(day).padStart(2, "0")}.${String(month).padStart(2, "0")}.`;
  return weekday ? `${weekday}, ${dayMonth}` : dayMonth;
}

function parseISOParts(
  isoDate: string,
): { year: number; month: number; day: number } | null {
  const [year, month, day] = isoDate.split("-").map(Number);
  if (!year || !month || !day) return null;
  return { year, month, day };
}
