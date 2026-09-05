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
import { useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Skeleton } from "~/components/ui/skeleton";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import type { ClassDayReport } from "~/lib/class-day-api";
import { formatDate, parseISODate, todayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { ClassDayClasses } from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { normalizeSchoolClass } from "~/lib/timetable-helpers";
import { ClassArrivalExceptionDialog } from "./class-arrival-exception-dialog";
import { countDayChanges } from "./day-changes";
import {
  classDayDateParam,
  classDayOverviewPath,
  isWeekendISO,
} from "./routes";
import { Section } from "./student-row";

const logger = createLogger({ component: "ClassDayClass" });

const REPORT_FOCUS_THROTTLE_MS = 60 * 1000;

/**
 * Die Klassen-Tagesausnahme als eine Zeile (#2962/#2970): "Heute kommt die
 * Klasse um 12:45 Uhr (Unterricht fällt aus)". Für andere Tage nennt sie den
 * Tag statt "Heute", damit die Zeile nicht als heutige Änderung gelesen wird.
 */
export function classArrivalExceptionLine(
  report: Pick<ClassDayReport, "class_arrival_exception">,
  dateISO: string,
  today: string,
): string | null {
  const exception = report.class_arrival_exception;
  if (!exception) return null;
  const when = dateISO === today ? "Heute" : `Am ${formatDate(dateISO)}`;
  const reason = exception.reason ? ` (${exception.reason})` : "";
  return `${when} kommt die Klasse um ${exception.arrival_time} Uhr${reason}`;
}

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
  if (totals.list_entries > 0) {
    parts.push(`${totals.list_entries} keine Betreuung`);
  }
  return parts.join(" · ");
}

export interface ClassDayClassProps {
  readonly schoolClass: string;
  readonly fetchClassDay: (
    schoolClass: string,
    date: string,
  ) => Promise<ClassDayReport>;
  /**
   * Zugewiesene Klassen mit dem Schreib-Flag (#2970). Ohne Abruf gibt es
   * keinen Knopf, die Ansicht bleibt rein lesend.
   */
  readonly fetchClasses?: () => Promise<ClassDayClasses>;
  /** Nur für Tests: fixiert den Vergleichstag für "Heute gemeldet". */
  readonly now?: Date;
}

export function ClassDayClass({
  schoolClass,
  fetchClassDay,
  fetchClasses,
  now,
}: ClassDayClassProps) {
  // Kein Default-Prop: eine `new Date()` in der Signatur bricht die
  // referenzielle Gleichheit bei jedem Render.
  const at = now ?? new Date();
  const searchParams = useSearchParams();
  const dateISO = classDayDateParam(searchParams.get("tag"));
  const weekend = isWeekendISO(dateISO);
  const [exceptionDialogOpen, setExceptionDialogOpen] = useState(false);

  // Das Schreib-Flag kommt von der Klassenliste (Berechtigung UND Freigabe
  // der OGS). Bei "nicht freigegeben" gibt es keinen Knopf und keinen
  // ausgegrauten Hinweis: nichts, was nach einer Aktion aussieht.
  const { data: classes } = useSWRAuth(
    fetchClasses ? "class-day-classes" : null,
    async () => {
      if (!fetchClasses) {
        return { classes: [], can_write_arrival_exception: false };
      }
      try {
        return await fetchClasses();
      } catch (err) {
        logger.error("class_day_classes_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        throw err;
      }
    },
    { revalidateOnFocus: false },
  );
  // Das Flag allein sagt nur, dass die Schule schreiben darf. Der Knopf und
  // der Dialog erscheinen nur für eine Klasse, die der Lehrkraft zugewiesen
  // ist (gleicher Vergleich wie das Backend, schoolclass.Normalize): für eine
  // fremde Klasse aus einer alten Adresse würde jeder Versuch mit 403 enden.
  const isAssignedClass =
    classes?.classes.some(
      (klass) =>
        normalizeSchoolClass(klass) === normalizeSchoolClass(schoolClass),
    ) === true;
  const canWriteArrivalException =
    classes?.can_write_arrival_exception === true && isAssignedClass;

  const {
    data: report,
    error,
    isLoading,
    mutate: refetchReport,
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
  const exceptionLine =
    report && !weekend
      ? classArrivalExceptionLine(report, dateISO, todayISO())
      : null;
  const isToday = dateISO === todayISO();

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
          {schoolClass && (
            <h2 className="text-base font-semibold text-gray-900">
              {schoolClassLabel(schoolClass)}
            </h2>
          )}
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
        {/* Die Klassen-Tagesausnahme steht über der Liste, auch wenn die OGS
            sie eingetragen hat und die Schule nichts ändern darf. */}
        {exceptionLine ? (
          <p className="mt-1 text-sm font-medium text-gray-900">
            {exceptionLine}
          </p>
        ) : null}
        {/* Nur mit Freigabe der OGS (#2970). Der Knopf nennt den Tag, den er
            vorbelegt: "heute" nur, wenn heute angezeigt wird. */}
        {schoolClass && !weekend && canWriteArrivalException ? (
          <div className="mt-3">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setExceptionDialogOpen(true)}
            >
              {isToday
                ? "Ankunft heute ändern"
                : "Ankunft an diesem Tag ändern"}
            </Button>
          </div>
        ) : null}

        {schoolClass && weekend && (
          <EmptyState
            className="mt-4"
            title="Kein Schultag"
            description="Für Samstag und Sonntag gibt es keine Übergabe. Bitte einen Wochentag wählen."
          />
        )}

        {/* Adresse ohne Klasse (abgeschnittener Link, alter Bookmark): sagen,
            was fehlt, statt eine Klasse ohne Namen zu laden. */}
        {!schoolClass && (
          <EmptyState
            className="mt-4"
            title="Keine Klasse ausgewählt"
            description="Diese Adresse nennt keine Klasse. Bitte gehen Sie zurück zu allen Klassen und wählen Sie eine aus."
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

      {schoolClass && canWriteArrivalException ? (
        <ClassArrivalExceptionDialog
          isOpen={exceptionDialogOpen}
          onClose={() => setExceptionDialogOpen(false)}
          schoolClass={schoolClass}
          defaultDate={weekend ? null : parseISODate(dateISO)}
          onChanged={() => void refetchReport()}
        />
      ) : null}
    </div>
  );
}
