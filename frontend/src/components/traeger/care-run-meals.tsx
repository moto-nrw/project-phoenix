"use client";

// Die Mensa im Monatslauf (#4686): je freigegebenem Mensa-Monat eine Zeile
// auf der Rechnung des Kindes, die Monate ohne Freigabe der OGS-Leitung und
// die Kinder, deren Mensa dieser Lauf nicht abrechnet.

import { useId, useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import { Button } from "~/components/ui/button";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import type {
  CareMealGap,
  CareMealPosition,
  CareMealUnbilled,
} from "~/lib/traeger-care-meal-api";
import { formatMonth } from "./care-billing-format";
import { mealPositionLabel } from "./care-meal-format";
import { formatEuro } from "./contribution-format";

/** A Mensa month on an invoice: „2 Mittagessen im Januar 2026 à 4,80 €“. */
export function MealPositionLine({
  position,
}: Readonly<{ position: CareMealPosition }>) {
  return (
    <li className="flex items-baseline justify-between gap-4 py-1.5">
      <div className="min-w-0">
        <p className="text-sm text-gray-900">
          {mealPositionLabel(
            position.mealMonth,
            position.mode,
            position.quantity,
          )}
        </p>
        <p className="mt-0.5 text-xs text-gray-600">
          {position.mode === "per_meal"
            ? `Mensa · je ${formatEuro(position.unitCents)}`
            : "Mensa · fester Betrag je Monat"}
        </p>
      </div>
      <span className="shrink-0 text-sm text-gray-700 tabular-nums">
        {formatEuro(position.amountCents)}
      </span>
    </li>
  );
}

/** The gaps of each school, in the order the run lists them. */
function groupBySchool(
  gaps: readonly CareMealGap[],
): Map<string, CareMealGap[]> {
  const bySchool = new Map<string, CareMealGap[]>();
  for (const gap of gaps) {
    bySchool.set(gap.schoolName, [
      ...(bySchool.get(gap.schoolName) ?? []),
      gap,
    ]);
  }
  return bySchool;
}

const UNBILLED_REASONS: Record<CareMealUnbilled["reason"], string> = {
  no_contract: "Kein Vertrag an der Schule in diesem Monat.",
  contract_billed:
    "Die Rechnung dieses Monats ist schon ausgestellt. Der Lauf des nächsten Monats rechnet die Mensa ab.",
};

/** Children shown before „N weitere anzeigen“, as in the other hint lists. */
const INITIAL_CHILD_COUNT = 5;

/**
 * The children of one school and month that this run leaves out for the
 * same reason: one line with the count and the reason, the names below.
 */
function UnbilledGroup({
  entries,
}: Readonly<{ entries: readonly CareMealUnbilled[] }>) {
  const [expanded, setExpanded] = useState(false);
  const listId = useId();
  const first = entries[0];
  if (!first) return null;
  const meals = entries.reduce((sum, entry) => sum + entry.mealDays, 0);
  const sorted = [...entries].sort((a, b) =>
    `${a.childLastName} ${a.childFirstName}`.localeCompare(
      `${b.childLastName} ${b.childFirstName}`,
      "de",
    ),
  );
  const shown = expanded ? sorted : sorted.slice(0, INITIAL_CHILD_COUNT);
  const hiddenCount = sorted.length - shown.length;
  return (
    <li className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0 sm:col-span-2">
      <p className="text-sm font-semibold text-gray-900">
        {first.schoolName} · {formatMonth(first.month)}
      </p>
      <p className="text-sm text-gray-600">
        {entries.length} {entries.length === 1 ? "Kind" : "Kinder"} mit {meals}{" "}
        Essen nicht abgerechnet. {UNBILLED_REASONS[first.reason]}
      </p>
      <ul
        id={listId}
        className="mt-1 grid grid-cols-1 gap-x-8 gap-y-1 sm:grid-cols-2 lg:grid-cols-4"
      >
        {shown.map((entry) => (
          <li
            key={entry.studentId}
            className="flex min-w-0 items-baseline gap-2 text-sm"
          >
            <span className="min-w-0 truncate text-gray-900">
              {entry.childFirstName} {entry.childLastName}
            </span>
            <span className="shrink-0 text-gray-600 tabular-nums">
              {entry.mealDays} Essen
            </span>
          </li>
        ))}
      </ul>
      {sorted.length > INITIAL_CHILD_COUNT ? (
        <Button
          type="button"
          variant="ghost"
          size="md"
          className="mt-1 self-start"
          aria-expanded={expanded}
          aria-controls={listId}
          onClick={() => setExpanded((current) => !current)}
        >
          {expanded ? (
            <>
              Weniger anzeigen
              <ChevronUp className="size-4" aria-hidden="true" />
            </>
          ) : (
            <>
              {hiddenCount} weitere anzeigen
              <ChevronDown className="size-4" aria-hidden="true" />
            </>
          )}
        </Button>
      ) : null}
    </li>
  );
}

/**
 * The Mensa of a run in preview: the ended months a school has not released
 * and the children whose released months this run does not bill.
 */
export function CareRunMealsCard({
  gaps,
  unbilled,
}: Readonly<{
  gaps: readonly CareMealGap[];
  unbilled: readonly CareMealUnbilled[];
}>) {
  if (gaps.length === 0 && unbilled.length === 0) return null;
  const open = gaps.filter((gap) => !gap.expired);
  const expired = gaps.filter((gap) => gap.expired);
  const bySchool = groupBySchool(open);
  const expiredBySchool = groupBySchool(expired);
  const groups = new Map<string, CareMealUnbilled[]>();
  for (const entry of unbilled) {
    const key = `${entry.schoolId}-${entry.month}-${entry.reason}`;
    groups.set(key, [...(groups.get(key) ?? []), entry]);
  }
  return (
    <SectionCard
      title="Mensa"
      titleBadge={
        <span className="text-moto-orange-strong text-sm font-medium">
          {[
            open.length > 0
              ? `${open.length} ${open.length === 1 ? "Monat" : "Monate"} ohne Freigabe`
              : null,
            expired.length > 0
              ? `${expired.length} nicht mehr abrechenbar`
              : null,
            unbilled.length > 0 ? `${unbilled.length} nicht abgerechnet` : null,
          ]
            .filter(Boolean)
            .join(" · ")}
        </span>
      }
      description="Die Mensa eines Monats kommt erst auf die Rechnung, wenn die OGS-Leitung den Monat freigegeben hat. Ein späterer Lauf rechnet ihn dann nachträglich ab."
    >
      <ResponsiveActionList>
        {[...bySchool.entries()].map(([school, months]) => (
          <li
            key={`gap-${school}`}
            className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0 sm:col-span-2"
          >
            <p className="text-sm font-semibold text-gray-900">{school}</p>
            <p className="text-sm text-gray-600">
              Freigabe fehlt für{" "}
              {months.map((gap) => formatMonth(gap.month)).join(", ")}.
            </p>
          </li>
        ))}
        {[...expiredBySchool.entries()].map(([school, months]) => (
          <li
            key={`expired-${school}`}
            className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0 sm:col-span-2"
          >
            <p className="text-sm font-semibold text-gray-900">{school}</p>
            <p className="text-sm text-gray-600">
              Nicht mehr abrechenbar:{" "}
              {months.map((gap) => formatMonth(gap.month)).join(", ")}. Die
              Anwesenheiten dieser Monate sind nach der Aufbewahrungsfrist
              gelöscht.
            </p>
          </li>
        ))}
        {[...groups.entries()].map(([key, entries]) => (
          <UnbilledGroup key={key} entries={entries} />
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}
