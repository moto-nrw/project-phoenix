"use client";

// Eine Klasse an einem Tag ("moto schule", #1772/#2294).
//
// Die Seite beantwortet genau eine Frage: wer aus dieser Klasse bleibt heute
// in der Betreuung, wer geht nach Hause, und bei wem ist es heute anders als
// sonst. Abweichungen stehen am Kind selbst, nicht in einem eigenen Block —
// jedes Kind kommt genau einmal vor.

import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useMemo } from "react";
import type { CSSProperties } from "react";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import type { ClassDayReport } from "~/lib/class-day-api";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { countDayChanges } from "./day-changes";
import {
  classDayDateParam,
  classDayOverviewPath,
  isWeekendISO,
} from "./routes";
import { Section } from "./student-row";

const logger = createLogger({ component: "ClassDayClass" });

const REPORT_FOCUS_THROTTLE_MS = 60 * 1000;

/** Die Zahlen des Tages als eine Zeile unter dem Klassennamen. */
function summaryLine(report: ClassDayReport): string {
  const totals = report.totals;
  if (!report.enrollment_known) {
    return `${totals.students} Kinder im Klassenverband`;
  }
  const parts = [
    `${totals.students} Kinder`,
    `${totals.staying} bleiben`,
    `${totals.leaving} gehen heim`,
  ];
  if (totals.absent > 0) parts.push(`${totals.absent} abgemeldet`);
  return parts.join(" · ");
}

export interface ClassDayClassProps {
  readonly schoolClass: string;
  readonly fetchClassDay: (
    schoolClass: string,
    date: string,
  ) => Promise<ClassDayReport>;
  /** Nur für Tests: fixiert den Vergleichstag für "Heute gemeldet". */
  readonly now?: Date;
}

export function ClassDayClass({
  schoolClass,
  fetchClassDay,
  now,
}: ClassDayClassProps) {
  // Kein Default-Prop: eine `new Date()` in der Signatur bricht die
  // referenzielle Gleichheit bei jedem Render.
  const at = now ?? new Date();
  const searchParams = useSearchParams();
  const dateISO = classDayDateParam(searchParams.get("tag"));
  const weekend = isWeekendISO(dateISO);

  const {
    data: report,
    error,
    isLoading,
  } = useSWRAuth(
    schoolClass && !weekend ? `class-day-${schoolClass}-${dateISO}` : null,
    async () => {
      try {
        return await fetchClassDay(schoolClass, dateISO);
      } catch (err) {
        logger.error("class_day_fetch_failed", {
          date: dateISO,
          error: err instanceof Error ? err.message : String(err),
        });
        throw err;
      }
    },
    {
      revalidateOnFocus: true,
      focusThrottleInterval: REPORT_FOCUS_THROTTLE_MS,
      keepPreviousData: false,
    },
  );

  const rows = useMemo(() => report?.rows ?? [], [report]);
  const staying = useMemo(() => rows.filter((row) => row.stays_today), [rows]);
  const leaving = useMemo(
    () =>
      rows.filter((row) => !row.stays_today && !row.status && !row.list_entry),
    [rows],
  );
  // Klassenlisteneinträge (#2382): "Keine Betreuung" ist eine neutrale
  // Verbands-Aussage, kein "geht nach Hause" — eigene Sektion statt der
  // Übergabe-Kategorien.
  const noCare = useMemo(() => rows.filter((row) => row.list_entry), [rows]);
  const absent = useMemo(
    () => rows.filter((row) => Boolean(row.status)),
    [rows],
  );
  // Ohne Anmeldedaten (keine abdeckende Phase) gibt es keinen
  // Bleiben/Gehen-Split — alle nicht abgemeldeten Kinder neutral listen.
  const unknownRows = useMemo(() => rows.filter((row) => !row.status), [rows]);
  const changes = countDayChanges(report ?? null);

  return (
    <div
      className="w-full"
      style={
        {
          "--class-day-blue": MOTO_COLOR_PALETTE.blue.base,
          "--class-day-danger": MOTO_COLOR_PALETTE.red.base,
        } as CSSProperties
      }
    >
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <Link
          href={schoolPath(classDayOverviewPath(dateISO))}
          className="inline-flex items-center gap-1 text-sm font-medium text-gray-600 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          Alle Klassen
        </Link>

        <div className="mt-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <h2 className="text-base font-semibold text-gray-900">
            {schoolClassLabel(schoolClass)}
          </h2>
          <p className="text-sm text-gray-500">{formatDate(dateISO)}</p>
        </div>
        {report && !weekend && (
          <p className="mt-1 text-sm text-gray-600">
            {summaryLine(report)}
            {changes > 0
              ? ` · ${changes === 1 ? "1 Kind" : `${changes} Kinder`} anders als sonst`
              : ""}
          </p>
        )}

        {weekend && (
          <EmptyState
            className="mt-4"
            title="Kein Schultag"
            description="Für Samstag und Sonntag gibt es keine Übergabe. Bitte einen Wochentag wählen."
          />
        )}

        {!weekend && isLoading && (
          <div className="mt-4 space-y-3">
            <Skeleton className="h-40 w-full" />
          </div>
        )}

        {!weekend && !isLoading && error && (
          <EmptyState
            className="mt-4"
            title={`${schoolClassLabel(schoolClass)} nicht verfügbar`}
            description="Diese Klasse konnte nicht geladen werden. Möglicherweise ist sie Ihnen nicht mehr zugewiesen. Bitte gehen Sie zurück zu allen Klassen."
          />
        )}

        {!weekend && !isLoading && report && rows.length === 0 && (
          <EmptyState
            className="mt-4"
            title="Keine Kinder gefunden"
            description={`Für die Klasse ${report.school_class} sind keine Kinder hinterlegt.`}
          />
        )}

        {/* Ohne abdeckende Anmeldephase ist Bleiben/Gehen unbekannt —
            neutraler Klassenverband statt "alle gehen nach Hause". */}
        {!weekend &&
          !isLoading &&
          report &&
          rows.length > 0 &&
          !report.enrollment_known && (
            <div className="mt-5 space-y-4 border-t border-gray-100 pt-4">
              <Alert
                type="info"
                message="Für diesen Tag liegt keine Anmeldephase vor. Wer in der Betreuung bleibt, kann deshalb nicht angezeigt werden."
              />
              <Section
                title="Klassenverband"
                count={unknownRows.length}
                rows={unknownRows}
                enrollmentKnown={false}
                now={at}
              />
              <Section
                title="Abgemeldet"
                count={absent.length}
                accent="text-[var(--class-day-danger)]"
                rows={absent}
                enrollmentKnown={false}
                now={at}
              />
            </div>
          )}

        {!weekend &&
          !isLoading &&
          report &&
          rows.length > 0 &&
          report.enrollment_known && (
            <div className="mt-5 space-y-4 border-t border-gray-100 pt-4">
              <Section
                title="Bleiben in der Betreuung"
                count={staying.length}
                accent="text-[var(--class-day-blue)]"
                rows={staying}
                now={at}
              />
              <Section
                title="Gehen nach Hause"
                count={leaving.length}
                rows={leaving}
                now={at}
              />
              {noCare.length > 0 && (
                <Section
                  title="Keine Betreuung"
                  count={noCare.length}
                  rows={noCare}
                  now={at}
                />
              )}
              <Section
                title="Abgemeldet"
                count={absent.length}
                accent="text-[var(--class-day-danger)]"
                rows={absent}
                now={at}
              />
            </div>
          )}
      </section>
    </div>
  );
}
