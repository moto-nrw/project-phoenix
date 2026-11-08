"use client";

// Die Mensa im Monatslauf (#4686): je freigegebenem Mensa-Monat eine Zeile
// auf der Rechnung des Kindes, die Monate ohne Freigabe der OGS-Leitung und
// die Kinder, deren Mensa dieser Lauf nicht abrechnet.

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

const UNBILLED_REASONS: Record<CareMealUnbilled["reason"], string> = {
  no_contract: "Kein Vertrag an der Schule in diesem Monat.",
  contract_billed:
    "Die Rechnung dieses Monats ist schon ausgestellt. Der Lauf des nächsten Monats rechnet die Mensa ab.",
};

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
  const bySchool = new Map<string, CareMealGap[]>();
  for (const gap of gaps) {
    bySchool.set(gap.schoolName, [
      ...(bySchool.get(gap.schoolName) ?? []),
      gap,
    ]);
  }
  return (
    <SectionCard
      title="Mensa"
      titleBadge={
        <span className="text-moto-orange-strong text-sm font-medium">
          {[
            gaps.length > 0
              ? `${gaps.length} ${gaps.length === 1 ? "Monat" : "Monate"} ohne Freigabe`
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
        {unbilled.map((entry) => (
          <li
            key={`unbilled-${entry.schoolId}-${entry.studentId}-${entry.month}`}
            className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0 sm:col-span-2"
          >
            <p className="text-sm font-semibold text-gray-900">
              {entry.childFirstName} {entry.childLastName}
            </p>
            <p className="text-sm text-gray-600">
              {entry.schoolName} · {formatMonth(entry.month)} · {entry.mealDays}{" "}
              Essen. {UNBILLED_REASONS[entry.reason]}
            </p>
          </li>
        ))}
      </ResponsiveActionList>
    </SectionCard>
  );
}
