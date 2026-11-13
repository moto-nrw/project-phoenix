"use client";

// Der gemeinsame Kopf eines Ferienzeitraums (#5146): „Planung“, „Formular“
// (#5198), „Anmeldungen“ und „Plätze“ (#4699) sind Reiter derselben Sache,
// jeder mit eigener Adresse. Vergibt der Zeitraum seine Plätze nach Kriterien
// (#4700), kommt der Reiter „Vergabe“ dazu.
// Der Status steht als Plakette in der Statuszeile, auf beiden Reitern gleich.

import type { ReactNode } from "react";
import { StatusBadge } from "~/components/ui/status-badge";
import type { TenantPageProps } from "~/components/ui/tenant-page";
import type { HolidayAllocationMode, HolidayPeriod } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";
import { formatDateRange, holidayStatusLabel } from "./holiday-period-format";

/** Was ein Klick auf die Kriterien der Platzvergabe tut (#6747). */
export const HOLIDAY_CRITERIA_HINT =
  "Öffnet die Kriterien für alle Ferien mit Vergabe nach Kriterien.";

export type HolidayPeriodTab =
  "planung" | "formular" | "anmeldungen" | "plaetze" | "vergabe";

/** Die Reiter eines Ferienzeitraums; jeder führt auf seine eigene Seite. */
export function holidayPeriodTabs(
  periodId: string,
  active: HolidayPeriodTab,
  registrationCount?: number,
  allocationMode?: HolidayAllocationMode,
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
        value: "formular",
        label: "Formular",
        href: traegerPath(`/traeger/ferien/${periodId}/formular`),
      },
      {
        value: "anmeldungen",
        label: "Anmeldungen",
        badge: registrationCount,
        href: traegerPath(`/traeger/ferien/${periodId}/anmeldungen`),
      },
      {
        value: "plaetze",
        label: "Plätze",
        href: traegerPath(`/traeger/ferien/${periodId}/plaetze`),
      },
      ...(allocationMode === "criteria" || active === "vergabe"
        ? [
            {
              value: "vergabe",
              label: "Vergabe",
              href: traegerPath(`/traeger/ferien/${periodId}/vergabe`),
            },
          ]
        : []),
    ],
  };
}

/**
 * Die Angaben einer Statuszeile im Ferienbereich: einzeln mit Abstand, ohne
 * Trennpunkte dazwischen (#5684).
 */
export function HolidayHeadFacts({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <span className="inline-flex flex-wrap items-center gap-x-3 gap-y-1">
      {children}
    </span>
  );
}

/** Statuszeile: Status des Zeitraums und seine Anmeldefrist. */
export function HolidayPeriodStatusLine({
  period,
  children,
}: Readonly<{ period: HolidayPeriod; children?: ReactNode }>) {
  return (
    <HolidayHeadFacts>
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
      {period.maxDaysPerChild == null ? null : (
        <span>höchstens {period.maxDaysPerChild} Tage je Kind</span>
      )}
      {children}
    </HolidayHeadFacts>
  );
}
