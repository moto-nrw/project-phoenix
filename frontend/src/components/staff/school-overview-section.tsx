"use client";

// Sektion 2 des /staff-Dashboards (#1417 Tranche 2a): die aggregierte Sicht auf
// die ganze Einrichtung. Läuft mit users:read und enthält bewusst KEINE Zahlen
// zu einzelnen Personen — die stehen in der Zeitkonten-Tabelle, die
// time_tracking:manage verlangt.
//
// Bewusst kein Chart: das Issue nennt "Mini-Chart + Donut", aber die DSGVO-
// Ausschlüsse (kein Ranking, kein Vergleich zwischen Mitarbeitenden) lassen nur
// Aggregate übrig, und bei Einrichtungsgrößen um 20 Personen ist ein Donut über
// drei kleine ganze Zahlen schlechter lesbar als die Zahlen selbst. Die eine
// Visualisierung, die etwas hinzufügt, ist der Fortschrittsbalken (Ist gegen
// Soll, Eingestempelte gegen Erwartete) — dasselbe Muster wie auf der
// Detailseite.
//
// Warum hier eine eigene Kachel und nicht die KpiCard aus staff-time-views:
// die KpiCard ist eine freistehende Karte mit eigenem Rahmen und Schatten. Sechs
// davon nebeneinander ergeben eine unruhige Reihe schwebender Kästen. Diese
// Sektion ist EINE Karten-Fläche (moto-content-surface, rounded-2xl,
// border-gray-200, shadow-sm — das Kit-Standardmaß) mit Haarlinien-Rastern
// darin; der Kontostand sitzt als Fußzeile in derselben Fläche.

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Skeleton } from "~/components/ui/skeleton";
import { formatSignedDuration } from "~/components/staff/staff-time-views";
import { getDeltaStatus } from "~/lib/staff-metrics-helpers";
import {
  formatDuration,
  OPEN_MONTH_REFRESH_MS,
} from "~/lib/time-tracking-helpers";
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
 * Kachel, weil genau diese Zahl sonst in der Leitungsrunde gegen die
 * Monatskarte gerechnet wird.
 */
const deltaExplanations: Record<DashboardPeriod, string> = {
  week: "Nicht Ist minus Soll: Krankheits-, Urlaubs- und Fortbildungstage werden mit dem Tagessoll gutgeschrieben. Die Zahl ist die Summe der Saldo-Veränderungen aller Mitarbeitenden in dieser Woche.",
  month:
    "Nicht Ist minus Soll: Krankheits-, Urlaubs- und Fortbildungstage werden mit dem Tagessoll gutgeschrieben. Die Zahl ist die Summe der Monatssalden aller Mitarbeitenden.",
};

type StatTone = "green" | "amber" | "gray" | "red";

const valueTone: Record<StatTone, string> = {
  green: "text-[#4a7a15]",
  amber: "text-amber-600",
  gray: "text-gray-900",
  red: "text-moto-red",
};

const barTone: Record<StatTone, string> = {
  green: "bg-moto-green",
  amber: "bg-amber-500",
  gray: "bg-gray-300",
  red: "bg-moto-red",
};

function StatTile({
  label,
  value,
  hint,
  progressPct,
  tone = "gray",
  title,
}: {
  readonly label: string;
  readonly value: string;
  readonly hint?: string;
  readonly progressPct?: number;
  readonly tone?: StatTone;
  readonly title?: string;
}) {
  return (
    <div className="flex flex-col bg-white p-4" title={title}>
      {/* Feste Label-Höhe: bei sechs Kacheln nebeneinander bricht "Aktive
          Mitarbeitende" auf zwei Zeilen um, alle anderen nicht. Ohne die
          Mindesthöhe stehen die Werte dann auf verschiedenen Grundlinien. */}
      <p className="min-h-8 text-[11px] leading-4 font-semibold tracking-wide text-gray-500 uppercase">
        {label}
      </p>
      <p
        className={`mt-1 text-xl leading-tight font-bold tabular-nums ${valueTone[tone]}`}
      >
        {value}
      </p>
      {progressPct !== undefined && (
        <div className="mt-2 h-1 overflow-hidden rounded-full bg-gray-100">
          <div
            className={`h-full rounded-full ${barTone[tone]} transition-all`}
            style={{ width: `${Math.min(100, Math.max(0, progressPct))}%` }}
          />
        </div>
      )}
      {/* mt-auto: die Hinweiszeile sitzt auf gleicher Höhe, egal ob die Kachel
          einen Balken hat oder der Hinweis zweizeilig umbricht. */}
      <p className="mt-auto pt-2 text-xs leading-4 text-gray-500">
        {hint ?? "\u00a0"}
      </p>
    </div>
  );
}

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
    // The period label changes immediately. Previous-key data would therefore
    // appear as a summary for the wrong period while this request is pending.
    {
      keepPreviousData: false,
      refreshInterval: OPEN_MONTH_REFRESH_MS,
      revalidateOnFocus: false,
    },
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
    <section className="mb-6" aria-labelledby="einrichtungs-uebersicht-heading">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2
          id="einrichtungs-uebersicht-heading"
          className="text-base font-bold text-gray-800"
        >
          Einrichtungs-Übersicht
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
                ? "Die Einrichtungs-Übersicht konnte nicht aktualisiert werden. Die zuletzt geladenen Werte bleiben sichtbar."
                : "Die Einrichtungs-Übersicht konnte nicht geladen werden."
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
        <Skeleton className="h-44 rounded-2xl sm:h-36" />
      ) : (
        <div className="moto-content-surface overflow-hidden rounded-2xl border border-gray-200 shadow-sm">
          {/* gap-px auf grauem Grund: Haarlinien zwischen den Kacheln, die
              auch nach einem Umbruch sauber durchlaufen. */}
          <div className="grid grid-cols-2 gap-px bg-gray-200 sm:grid-cols-3 xl:grid-cols-6">
            <StatTile
              label="Aktive Mitarbeitende"
              value={summary ? String(summary.activeStaffCount) : dash}
              hint="im laufenden Betrieb"
            />
            <StatTile
              label="Eingestempelt"
              value={
                summary
                  ? `${summary.currentlyClockedIn} / ${summary.expectedClockedIn}`
                  : dash
              }
              hint="von jetzt erwarteten"
              progressPct={clockedInPct}
              tone={
                summary &&
                summary.currentlyClockedIn >= summary.expectedClockedIn
                  ? "green"
                  : "amber"
              }
            />
            <StatTile
              label="Krank heute"
              value={summary ? String(summary.sickToday) : dash}
              hint="gemeldet"
              tone={summary && summary.sickToday > 0 ? "amber" : "gray"}
            />
            <StatTile
              label="Urlaub heute"
              value={summary ? String(summary.vacationToday) : dash}
              hint="genehmigt"
            />
            <StatTile
              label={periodLabels[period]}
              value={summary ? formatDuration(summary.istMinutes) : dash}
              hint={
                summary
                  ? `von ${formatDuration(summary.sollMinutes)} Soll`
                  : undefined
              }
              progressPct={istPct}
            />
            <StatTile
              label="Saldo-Veränderung"
              value={
                summary ? formatSignedDuration(summary.deltaMinutes) : dash
              }
              hint="inkl. Gutschrift Krank/Urlaub"
              tone={summary ? getDeltaStatus(summary.deltaMinutes) : "gray"}
              title={deltaExplanations[period]}
            />
          </div>

          {summary && (
            // Kontostand, kein Zeitraumwert: für Woche und Monat identisch und
            // deshalb aus der Zeitraum-Reihe herausgenommen — aber in derselben
            // Fläche, nicht als zweiter schwebender Kasten darunter.
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-t border-gray-200 bg-gray-50/70 px-4 py-3">
              <div>
                <p className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
                  Stundenkonto der Einrichtung
                </p>
                <p className="text-xs text-gray-500">
                  Kontostand über alle Mitarbeitenden, unabhängig vom gewählten
                  Zeitraum
                </p>
              </div>
              <p
                className={`text-lg font-bold tabular-nums ${
                  summary.saldoSchoolTotalMinutes < 0
                    ? "text-moto-red"
                    : "text-gray-900"
                }`}
              >
                {formatSignedDuration(summary.saldoSchoolTotalMinutes)}
              </p>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
