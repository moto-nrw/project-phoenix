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
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
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

/**
 * Die Kennzahl-Kachel kommt aus dem Kit. Diese Datei hatte vorher ihre eigene
 * („StatTile"): eigene Versalien, eigene Tonwert-Tabelle, eigener Balken —
 * dieselbe Aussage in einer zweiten Bauart, direkt neben der Startseite, die
 * schon StatCard benutzt. Das Kit sagt ausdrücklich: es gibt im Tenant-Portal
 * keine zweite Kennzahl-Kachel.
 */
const toneForStatCard = (tone: "green" | "amber" | "gray" | "red") =>
  tone === "amber" ? ("orange" as const) : tone;

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
    // Überschrift und Zeitraum stehen IN der Karte, wie in jedem anderen
    // Abschnitt; vorher schwebten sie über der Fläche.
    // Nicht `bare`: sonst stehen Überschrift, Zeitraumwahl und der
    // Stundenkonto-Streifen direkt auf dem gemusterten Grund. Es gibt zwei
    // Ebenen — Grund und Fläche — und Text gehört auf eine Fläche.
    <SectionCard
      title="Einrichtungs-Übersicht"
      actions={
        <SegmentedControl
          ariaLabel="Zeitraum"
          value={period}
          onChange={(next) => setPeriod(next as DashboardPeriod)}
          items={[
            { value: "week", label: "Woche" },
            { value: "month", label: "Monat" },
          ]}
        />
      }
    >
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
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-6">
            <StatCard
              label="Aktive Mitarbeitende"
              value={summary ? String(summary.activeStaffCount) : dash}
              hint="im laufenden Betrieb"
              compactValue
            />
            <StatCard
              label="Eingestempelt"
              value={
                summary
                  ? `${summary.currentlyClockedIn} / ${summary.expectedClockedIn}`
                  : dash
              }
              hint="von jetzt erwarteten"
              progressPct={clockedInPct}
              compactValue
              tone={
                summary &&
                summary.currentlyClockedIn >= summary.expectedClockedIn
                  ? "green"
                  : "orange"
              }
            />
            <StatCard
              label="Krank heute"
              value={summary ? String(summary.sickToday) : dash}
              hint="gemeldet"
              compactValue
              tone={summary && summary.sickToday > 0 ? "orange" : "gray"}
            />
            <StatCard
              label="Urlaub heute"
              value={summary ? String(summary.vacationToday) : dash}
              hint="genehmigt"
              compactValue
            />
            <StatCard
              label={periodLabels[period]}
              value={summary ? formatDuration(summary.istMinutes) : dash}
              hint={
                summary
                  ? `von ${formatDuration(summary.sollMinutes)} Soll`
                  : undefined
              }
              progressPct={istPct}
              compactValue
            />
            <StatCard
              label="Saldo-Veränderung"
              value={
                summary ? formatSignedDuration(summary.deltaMinutes) : dash
              }
              hint="inkl. Gutschrift Krank/Urlaub"
              compactValue
              tone={
                summary
                  ? toneForStatCard(getDeltaStatus(summary.deltaMinutes))
                  : "gray"
              }
              title={deltaExplanations[period]}
            />
          </div>

          {summary && (
            // Kontostand, kein Zeitraumwert: für Woche und Monat identisch und
            // deshalb aus der Zeitraum-Reihe herausgenommen. Als ruhiger
            // Streifen unter den Kacheln, nicht als siebte Kachel.
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 rounded-xl bg-gray-50 px-4 py-3">
              <div>
                <p className="text-sm font-semibold text-gray-900">
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
    </SectionCard>
  );
}
