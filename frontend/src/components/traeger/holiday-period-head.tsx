"use client";

// Der gemeinsame Kopf eines Ferienzeitraums (#5146): „Planung“ und
// „Anmeldungen“ sind zwei Reiter derselben Sache, jeder mit eigener Adresse.
// Der Status steht als Plakette in der Statuszeile, auf beiden Reitern gleich.

import type { ReactNode } from "react";
import { StatusBadge } from "~/components/ui/status-badge";
import type { TenantPageProps } from "~/components/ui/tenant-page";
import type { HolidayPeriod } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";
import { formatDateRange, holidayStatusLabel } from "./holiday-period-format";

export type HolidayPeriodTab = "planung" | "anmeldungen";

/** Die Reiter eines Ferienzeitraums; jeder führt auf seine eigene Seite. */
export function holidayPeriodTabs(
  periodId: string,
  active: HolidayPeriodTab,
  registrationCount?: number,
): NonNullable<TenantPageProps["tabs"]> {
  return {
    value: active,
    // Die Reiter sind Links; die Navigation übernimmt der Link selbst.
    onChange: () => undefined,
    label: "Bereiche des Ferienzeitraums",
    items: [
      {
        value: "planung",
        label: "Planung",
        href: traegerPath(`/traeger/ferien/${periodId}`),
      },
      {
        value: "anmeldungen",
        label: "Anmeldungen",
        badge: registrationCount,
        href: traegerPath(`/traeger/ferien/${periodId}/anmeldungen`),
      },
    ],
  };
}

/** Statuszeile: Status des Zeitraums und seine Anmeldefrist. */
export function HolidayPeriodStatusLine({
  period,
  children,
}: Readonly<{ period: HolidayPeriod; children?: ReactNode }>) {
  return (
    <span className="inline-flex flex-wrap items-center gap-x-2 gap-y-1">
      <StatusBadge
        label={holidayStatusLabel(period)}
        tone={period.status === "published" ? "green" : "gray"}
      />
      <span>
        Anmeldung{" "}
        {formatDateRange(
          period.registrationOpensOn,
          period.registrationClosesOn,
        )}
      </span>
      {children}
    </span>
  );
}
