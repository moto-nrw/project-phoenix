"use client";

// Sektion 2 des /staff-Schul-Dashboards (#1417 Tranche 2a): die aggregierte
// Sicht auf die ganze Schule. Läuft mit users:read und enthält bewusst KEINE
// Zahlen zu einzelnen Personen — die stehen in der Zeitkonten-Tabelle, die
// time_tracking:manage verlangt.
//
// Bewusst kein Chart: das Issue nennt "Mini-Chart + Donut", aber die DSGVO-
// Ausschlüsse (kein Ranking, kein Vergleich zwischen Mitarbeitenden) lassen nur
// Aggregate übrig, und bei Schulgrößen um 20 Personen ist ein Donut über drei
// kleine ganze Zahlen schlechter lesbar als die Zahlen selbst. Die eine
// Visualisierung, die etwas hinzufügt, ist der Fortschrittsbalken in der
// KpiCard (Ist gegen Soll, Eingestempelte gegen Erwartete) — dasselbe Muster
// wie auf der Detailseite.

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Skeleton } from "~/components/ui/skeleton";
import {
  KpiCard,
  formatSignedDuration,
} from "~/components/staff/staff-time-views";
import { getDeltaStatus } from "~/lib/staff-metrics-helpers";
import { formatDuration } from "~/lib/time-tracking-helpers";
import { useSWRAuth } from "~/lib/swr";
import {
  staffOverviewService,
  type DashboardPeriod,
} from "~/lib/staff-overview-api";

const periodLabels: Record<DashboardPeriod, string> = {
  week: "Diese Woche",
  month: "Dieser Monat",
};

/**
 * Warum das Delta nicht Ist minus Soll ist. Steht als title-Tooltip an der
 * Karte, weil genau diese Zahl sonst in der Leitungsrunde gegen die
 * Monatskarte gerechnet wird.
 */
const deltaExplanations: Record<DashboardPeriod, string> = {
  week: "Nicht Ist minus Soll: Krankheits-, Urlaubs- und Fortbildungstage werden mit dem Tagessoll gutgeschrieben. Die Zahl ist die Summe der Saldo-Veränderungen aller Mitarbeitenden in dieser Woche.",
  month:
    "Nicht Ist minus Soll: Krankheits-, Urlaubs- und Fortbildungstage werden mit dem Tagessoll gutgeschrieben. Die Zahl ist die Summe der Monatssalden aller Mitarbeitenden.",
};

export function SchoolOverviewSection() {
  const [period, setPeriod] = useState<DashboardPeriod>("month");

  const {
    data: summary,
    error,
    isLoading,
    isValidating,
    mutate,
  } = useSWRAuth(
    `staff-dashboard-summary-${period}`,
    () => staffOverviewService.getDashboardSummary(period),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  const dash = "–";
  const clockedInPct =
    summary && summary.expectedClockedIn > 0
      ? (summary.currentlyClockedIn / summary.expectedClockedIn) * 100
      : undefined;
  const istPct =
    summary && summary.sollMinutes > 0
      ? (summary.istMinutes / summary.sollMinutes) * 100
      : undefined;

  return (
    <section className="mb-6" aria-labelledby="schul-uebersicht-heading">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2
          id="schul-uebersicht-heading"
          className="text-base font-bold text-gray-800"
        >
          Schul-Übersicht
        </h2>
        <Tabs
          value={period}
          onValueChange={(value) => setPeriod(value as DashboardPeriod)}
        >
          <TabsList>
            <TabsTrigger value="week">Woche</TabsTrigger>
            <TabsTrigger value="month">Monat</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      {error && (
        <div className="mb-3 space-y-2">
          <Alert
            type="error"
            message={
              summary
                ? "Die Schul-Übersicht konnte nicht aktualisiert werden. Die zuletzt geladenen Werte bleiben sichtbar."
                : "Die Schul-Übersicht konnte nicht geladen werden."
            }
          />
          <Button
            type="button"
            size="compact"
            variant="outline"
            isLoading={isValidating}
            loadingText="Wird geladen..."
            onClick={() => void mutate()}
          >
            Erneut laden
          </Button>
        </div>
      )}

      {!summary && error ? null : isLoading && !summary ? (
        <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-3 xl:grid-cols-6">
          {Array.from({ length: 6 }, (_, index) => (
            <Skeleton key={index} className="h-28 rounded-3xl" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-3 xl:grid-cols-6">
          <KpiCard
            label="Aktive Mitarbeitende"
            primary={summary ? String(summary.activeStaffCount) : dash}
            secondary="im laufenden Betrieb"
          />
          <KpiCard
            label="Eingestempelt"
            primary={
              summary
                ? `${summary.currentlyClockedIn} / ${summary.expectedClockedIn}`
                : dash
            }
            secondary="von jetzt erwarteten"
            progressPct={clockedInPct}
            color={
              summary && summary.currentlyClockedIn >= summary.expectedClockedIn
                ? "green"
                : "amber"
            }
          />
          <KpiCard
            label="Krank heute"
            primary={summary ? String(summary.sickToday) : dash}
            secondary="gemeldet"
            color={summary && summary.sickToday > 0 ? "amber" : "gray"}
          />
          <KpiCard
            label="Urlaub heute"
            primary={summary ? String(summary.vacationToday) : dash}
            secondary="genehmigt"
          />
          <KpiCard
            label={periodLabels[period]}
            primary={summary ? formatDuration(summary.istMinutes) : dash}
            secondary={
              summary
                ? `von ${formatDuration(summary.sollMinutes)} Soll`
                : undefined
            }
            progressPct={istPct}
            compactPrimary
          />
          <div title={deltaExplanations[period]}>
            <KpiCard
              label="Saldo-Veränderung"
              primary={
                summary ? formatSignedDuration(summary.deltaMinutes) : dash
              }
              compactPrimary
              secondary="inkl. Gutschrift für Krank und Urlaub"
              color={summary ? getDeltaStatus(summary.deltaMinutes) : "gray"}
            />
          </div>
        </div>
      )}

      {summary && (
        // Kontostand, kein Zeitraumwert: für Woche und Monat identisch und
        // deshalb aus der Zeitraum-Reihe herausgenommen.
        <div className="moto-content-surface mt-4 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 rounded-2xl border border-gray-200 p-4 shadow-sm">
          <div>
            <p className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
              Stundenkonto der Schule
            </p>
            <p className="mt-1 text-xs text-gray-500">
              Kontostand über alle Mitarbeitenden, unabhängig vom gewählten
              Zeitraum
            </p>
          </div>
          <p
            className={`text-2xl font-bold ${
              summary.saldoSchoolTotalMinutes < 0
                ? "text-red-600"
                : "text-gray-700"
            }`}
          >
            {formatSignedDuration(summary.saldoSchoolTotalMinutes)}
          </p>
        </div>
      )}
    </section>
  );
}
