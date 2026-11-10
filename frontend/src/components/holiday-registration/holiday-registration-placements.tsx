"use client";

// Platz oder Warteliste je Kind und Woche nach dem Absenden (#4699). Die
// Plätze gehen in der Reihenfolge der Anmeldung; in einer vollen Woche steht
// das Kind auf der Warteliste und rückt automatisch nach. Vergibt das Büro
// die Plätze nach Kriterien (#4700), ist jeder Platz zuerst offen.

import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import type {
  HolidayReceiptPlacement,
  HolidayRegistrationWeek,
} from "~/lib/holiday-registration-api";
import type { ChildDraft } from "./holiday-registration-model";
import { useWeekLabels } from "./holiday-registration-sections";

export function HolidayRegistrationPlacements({
  childDrafts,
  weeks,
  placements,
}: Readonly<{
  childDrafts: readonly ChildDraft[];
  weeks: readonly HolidayRegistrationWeek[];
  placements: readonly HolidayReceiptPlacement[];
}>) {
  const t = useTranslations("holidayRegistration");
  const labels = useWeekLabels();
  if (placements.length === 0) return null;
  const waitlisted = placements.some(
    (placement) => placement.placement === "waitlisted",
  );
  const pending = placements.some(
    (placement) => placement.placement === "pending",
  );
  return (
    <div className="space-y-4">
      <SectionLabel as="h3">{t("placementHeading")}</SectionLabel>
      <ul className="space-y-4">
        {childDrafts.map((child, index) => {
          const own = placements.filter(
            (placement) => placement.childPosition === index + 1,
          );
          if (own.length === 0) return null;
          return (
            <li key={child.key} className="space-y-2">
              <p className="text-sm font-semibold text-gray-900">
                {`${child.firstName} ${child.lastName}`.trim()}
              </p>
              <ul className="space-y-1.5">
                {weeks
                  .filter((week) =>
                    own.some((placement) => placement.weekId === week.id),
                  )
                  .map((week) => {
                    const state =
                      own.find((placement) => placement.weekId === week.id)
                        ?.placement ?? "placed";
                    const badge = {
                      placed: { label: t("placementPlaced"), tone: "green" },
                      waitlisted: {
                        label: t("placementWaitlisted"),
                        tone: "warning",
                      },
                      pending: { label: t("placementPending"), tone: "purple" },
                    } as const;
                    return (
                      <li
                        key={week.id}
                        className="flex flex-wrap items-center justify-between gap-2 text-sm text-gray-700"
                      >
                        <span>{labels.range(week)}</span>
                        <StatusBadge
                          label={badge[state].label}
                          tone={badge[state].tone}
                          size="xs"
                        />
                      </li>
                    );
                  })}
              </ul>
            </li>
          );
        })}
      </ul>
      {pending ? <Alert type="info" message={t("pendingNote")} /> : null}
      {waitlisted ? <Alert type="info" message={t("waitlistNote")} /> : null}
    </div>
  );
}
