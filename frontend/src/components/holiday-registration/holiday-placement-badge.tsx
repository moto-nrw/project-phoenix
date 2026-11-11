"use client";

// Platz, Warteliste oder offener Platz einer Woche (#4699, #4700): dieselben
// Worte nach dem Absenden und in der Eltern-App (#6673).

import { useTranslations } from "next-intl";
import { StatusBadge } from "~/components/ui/status-badge";
import type { HolidayPlacement } from "~/lib/parent-holiday-registration-entries";

const PLACEMENT_BADGES = {
  placed: { key: "placementPlaced", tone: "green" },
  waitlisted: { key: "placementWaitlisted", tone: "warning" },
  pending: { key: "placementPending", tone: "purple" },
} as const;

export function HolidayPlacementBadge({
  placement,
}: Readonly<{ placement: HolidayPlacement }>) {
  const t = useTranslations("holidayRegistration");
  const badge = PLACEMENT_BADGES[placement];
  return <StatusBadge label={t(badge.key)} tone={badge.tone} size="xs" />;
}
