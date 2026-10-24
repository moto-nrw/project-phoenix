"use client";

// Eine Klasse an einem Tag ("moto schule", #1772/#2294).
//
// Die Seite beantwortet genau eine Frage: wer aus dieser Klasse bleibt heute
// in der Betreuung, wer geht nach Hause, und bei wem ist es heute anders als
// sonst. Abweichungen stehen am Kind selbst, nicht in einem eigenen Block —
// jedes Kind kommt genau einmal vor. Heute trägt die Lehrkraft am Kind auch
// ein, ob es am Vormittag da ist (#4790), und übergibt die Klasse nach dem
// Unterricht an die OGS (#4794).

import { ChevronLeft } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FormModal } from "~/components/ui/form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import NavigationLink from "~/components/ui/navigation-link";
import { Skeleton } from "~/components/ui/skeleton";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import type {
  ClassDayReport,
  ClassDayRow,
  MorningAttendanceStatus,
} from "~/lib/class-day-api";
import { formatDate, parseISODate, todayISO } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import type {
  ClassDayClasses,
  ClassTripReportInput,
  ClassTripReportResult,
  ClassHandoverChildren,
  MorningAttendanceMark,
} from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { normalizeSchoolClass } from "~/lib/timetable-helpers";
import { ClassTripBulkStatusModal } from "~/components/students/class-trip-bulk-status-modal";
import { ClassArrivalExceptionDialog } from "./class-arrival-exception-dialog";
import { handoverLine, handoverRows, initialAbsentIds } from "./class-handover";
import { countDayChanges } from "./day-changes";
import {
  draftMarks,
  markOpenPresent,
  morningRowKey,
  morningStatus,
  morningSummaryLine,
  type MorningAttendanceDraft,
} from "./morning-attendance";
import {
  classDayDateParam,
  classDayOverviewPath,
  isWeekendISO,
} from "./routes";
import { Section, type StudentRowMorning } from "./student-row";

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
  /**
   * Meldet einen Ausflug oder eine Klassenfahrt (#4799). Ohne Funktion oder
   * ohne Freigabe der OGS gibt es keinen Knopf.
   */
  readonly reportClassTrip?: (
    schoolClass: string,
    input: ClassTripReportInput,
  ) => Promise<ClassTripReportResult>;
  /**
   * Speichert die Anwesenheit am Vormittag (#4790). Ohne sie bleibt die
   * Anwesenheit eine Anzeige.
   */
  readonly recordMorningAttendance?: (
    schoolClass: string,
    date: string,
    marks: readonly MorningAttendanceMark[],
  ) => Promise<void>;
  /**
   * Übergibt die Klasse heute an die OGS (#4794). Ohne sie bleibt die
   * Übergabe eine Anzeige.
   */
  readonly handOverClass?: (
    schoolClass: string,
    date: string,
    children: ClassHandoverChildren,
  ) => Promise<void>;
  /** Nimmt die heutige Übergabe zurück (#4794). */
  readonly withdrawClassHandover?: (
    schoolClass: string,
    date: string,
  ) => Promise<void>;
  /** Nur für Tests: fixiert den Vergleichstag für "Heute gemeldet". */
  readonly now?: Date;
}

type HandoverChoice = "present" | "absent";

export function ClassDayClass({
  schoolClass,
  fetchClassDay,
  fetchClasses,
  reportClassTrip,
  recordMorningAttendance,
  handOverClass,
  withdrawClassHandover,
  now,
}: ClassDayClassProps) {
  // Kein Default-Prop: eine `new Date()` in der Signatur bricht die
  // referenzielle Gleichheit bei jedem Render.
  const at = now ?? new Date();
  const searchParams = useSearchParams();
  const dateISO = classDayDateParam(searchParams.get("tag"));
  const weekendDay = isWeekendISO(dateISO);
  const [exceptionDialogOpen, setExceptionDialogOpen] = useState(false);
  const [tripModalOpen, setTripModalOpen] = useState(false);
  const [tripSaved, setTripSaved] = useState<string | null>(null);

  // Das Schreib-Flag kommt von der Klassenliste (Berechtigung UND Freigabe
  // der OGS). Bei "nicht freigegeben" gibt es keinen Knopf und keinen
  // ausgegrauten Hinweis: nichts, was nach einer Aktion aussieht. Bewusst
  // still bei einem Ladefehler: dann fehlt nur dieser Knopf, die Klasse
  // selbst lädt und meldet sich unten.
  const { data: classes } = useSWRAuth(
    fetchClasses ? "class-day-classes" : null,
    async () => {
      if (!fetchClasses) {
        return {
          classes: [],
          can_write_arrival_exception: false,
          weekend_follows_friday: false,
        };
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
  const canReportClassTrip =
    reportClassTrip !== undefined &&
    classes?.can_report_class_trip === true &&
    isAssignedClass;
  // Eine Schule, die Sa/So nach dem Freitagsplan betreut (#3921), hat auch
  // am Wochenende einen Schultag; sonst wird das Wochenende gar nicht geladen.
  const weekendOpen = classes?.weekend_follows_friday === true;
  const weekend = weekendDay && !weekendOpen;

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

  // Ladefehler mit Katalogtext und Wiederholen (#2517); eine entzogene
  // Klasse meldet sich über ihren Code (fehlende Berechtigung).
  const loadError = useSwrLoadError(
    error,
    `die Klasse ${schoolClassLabel(schoolClass)}`,
    () => refetchReport(),
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
  // Nur Kinder mit OGS-Datensatz lassen sich melden; Klassenlisteneinträge
  // ("Keine Betreuung") hat die OGS nicht.
  const tripStudents = useMemo(
    () =>
      rows
        .filter((row) => !row.list_entry && row.student_id > 0)
        .map((row) => ({
          id: String(row.student_id),
          first_name: row.first_name,
          second_name: row.last_name,
          name: `${row.first_name} ${row.last_name}`.trim(),
        })),
    [rows],
  );
  const saveClassTrip = async (input: ClassTripReportInput) => {
    if (!reportClassTrip) return;
    const result = await reportClassTrip(schoolClass, input);
    const one = result.student_count === 1;
    const children = one ? "1 Kind" : `${result.student_count} Kinder`;
    setTripSaved(
      result.kind === "return_time" && input.returnTime
        ? `Gespeichert. ${children} ${one ? "kommt" : "kommen"} um ${input.returnTime} Uhr in die OGS.`
        : `Gespeichert. Die OGS sieht ${children} an diesen Tagen als unterwegs.`,
    );
  };

  // Anwesenheit am Vormittag (#4790): Markierungen sammeln sich im Entwurf
  // dieser Klasse und dieses Tages und gehen erst mit „Speichern“ hinaus.
  const reportKey = `${schoolClass}-${dateISO}`;
  const [draftState, setDraftState] = useState<{
    key: string;
    marks: MorningAttendanceDraft;
  }>({ key: reportKey, marks: {} });
  const draft = draftState.key === reportKey ? draftState.marks : {};
  const [savingMorning, setSavingMorning] = useState(false);
  const morningErrors = useApiFormError();
  const canRecordMorning =
    Boolean(recordMorningAttendance) &&
    report?.can_record_morning_attendance === true &&
    !weekend;
  const draftCount = Object.keys(draft).length;
  const openCount = rows.filter(
    (row) => morningStatus(row, draft) === null,
  ).length;
  const markRow = useCallback(
    (row: ClassDayRow, status: MorningAttendanceStatus) => {
      setDraftState((current) => ({
        key: reportKey,
        marks: {
          ...(current.key === reportKey ? current.marks : {}),
          [morningRowKey(row)]: status,
        },
      }));
    },
    [reportKey],
  );
  const discardMorning = () => {
    morningErrors.clear();
    setDraftState({ key: reportKey, marks: {} });
  };
  const saveMorning = async () => {
    if (!recordMorningAttendance || draftCount === 0) return;
    morningErrors.clear();
    setSavingMorning(true);
    try {
      await recordMorningAttendance(
        schoolClass,
        dateISO,
        draftMarks(rows, draft),
      );
      await refetchReport();
      // Die neu geladene Liste zeigt den gespeicherten Stand, der Fuß
      // verschwindet: das ist die Rückmeldung.
      setDraftState({ key: reportKey, marks: {} });
    } catch (cause) {
      logger.error("class_day_morning_attendance_save_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void morningErrors.show(cause, {
        object: "die Anwesenheit",
        retry: () => void saveMorning(),
      });
    } finally {
      setSavingMorning(false);
    }
  };
  // Übergabe an die OGS (#4794): bestätigt wird je Klasse, Ausnahmen je Kind.
  const canHandOver =
    Boolean(handOverClass && withdrawClassHandover) &&
    report?.can_hand_over === true &&
    !weekend;
  const ogsRows = useMemo(() => handoverRows(rows), [rows]);
  const [handoverOpen, setHandoverOpen] = useState(false);
  const [handoverAbsent, setHandoverAbsent] = useState<ReadonlySet<number>>(
    () => new Set(),
  );
  const [savingHandover, setSavingHandover] = useState(false);
  const handoverErrors = useApiFormError();
  const [withdrawOpen, setWithdrawOpen] = useState(false);
  const [withdrawing, setWithdrawing] = useState(false);
  const withdrawErrors = useApiFormError();
  const openHandover = () => {
    handoverErrors.clear();
    setHandoverAbsent(initialAbsentIds(rows, Boolean(report?.handover), draft));
    setHandoverOpen(true);
  };
  const markHandover = (studentId: number, choice: HandoverChoice) => {
    setHandoverAbsent((current) => {
      const next = new Set(current);
      if (choice === "absent") next.add(studentId);
      else next.delete(studentId);
      return next;
    });
  };
  const saveHandover = async () => {
    if (!handOverClass) return;
    handoverErrors.clear();
    setSavingHandover(true);
    try {
      await handOverClass(schoolClass, dateISO, {
        present: ogsRows
          .filter((row) => !handoverAbsent.has(row.student_id))
          .map((row) => row.student_id),
        absent: ogsRows
          .filter((row) => handoverAbsent.has(row.student_id))
          .map((row) => row.student_id),
      });
      await refetchReport();
      setHandoverOpen(false);
    } catch (cause) {
      logger.error("class_day_handover_save_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void handoverErrors.show(cause, {
        object: "die Übergabe",
        retry: () => void saveHandover(),
      });
    } finally {
      setSavingHandover(false);
    }
  };
  const confirmWithdraw = async () => {
    if (!withdrawClassHandover) return;
    withdrawErrors.clear();
    setWithdrawing(true);
    try {
      await withdrawClassHandover(schoolClass, dateISO);
      await refetchReport();
      setWithdrawOpen(false);
    } catch (cause) {
      logger.error("class_day_handover_withdraw_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void withdrawErrors.show(cause, {
        object: "die Übergabe",
        retry: () => void confirmWithdraw(),
      });
    } finally {
      setWithdrawing(false);
    }
  };

  const morning: StudentRowMorning = {
    draft,
    onChange: canRecordMorning ? markRow : undefined,
    disabled: savingMorning,
  };

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
        <NavigationLink
          href={schoolPath(classDayOverviewPath(dateISO))}
          className="inline-flex items-center gap-1 text-sm font-medium text-gray-600 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          Alle Klassen
        </NavigationLink>

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
        {/* Anwesenheit am Vormittag (#4790): nur heute, mit Freigabe der OGS.
            Ein Satz sagt, was zu tun ist; die Zahlen zeigen, was noch fehlt. */}
        {canRecordMorning && rows.length > 0 ? (
          <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="text-sm font-medium text-gray-900">
                Anwesenheit am Vormittag: {morningSummaryLine(rows, draft)}
              </p>
              <p className="text-sm text-gray-600">
                Tippen Sie bei jedem Kind auf „Da“ oder „Fehlt“. Gemeldete
                Kinder stehen schon auf „Fehlt“.
              </p>
            </div>
            {openCount > 0 ? (
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={savingMorning}
                onClick={() =>
                  setDraftState({
                    key: reportKey,
                    marks: markOpenPresent(rows, draft),
                  })
                }
              >
                Alle übrigen da
              </Button>
            ) : null}
          </div>
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
        {schoolClass && !weekend && canReportClassTrip && report ? (
          <div className="mt-3">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => {
                setTripSaved(null);
                setTripModalOpen(true);
              }}
              disabled={tripStudents.length === 0}
            >
              Ausflug oder Klassenfahrt melden
            </Button>
          </div>
        ) : null}
        {tripSaved ? (
          <Alert type="success" message={tripSaved} className="mt-3" />
        ) : null}

        {/* Übergabe an die OGS (#4794). Die Zeile steht auch ohne
            Schreibrecht, sobald die Klasse übergeben ist, dann ohne Knopf. */}
        {report && !weekend && (report.handover || canHandOver) ? (
          <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="text-sm font-medium text-gray-900">
                {report.handover
                  ? handoverLine(report.handover, ogsRows.length)
                  : "Die Klasse ist noch nicht an die OGS übergeben."}
              </p>
              {canHandOver && !report.handover ? (
                <p className="text-sm text-gray-600">
                  Übergeben Sie die Klasse nach dem Unterricht. Die OGS sieht
                  dann sofort, wer nicht da ist.
                </p>
              ) : null}
            </div>
            {canHandOver && report.handover ? (
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={openHandover}
                >
                  Übergabe ändern
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={() => {
                    withdrawErrors.clear();
                    setWithdrawOpen(true);
                  }}
                >
                  Übergabe zurücknehmen
                </Button>
              </div>
            ) : null}
            {canHandOver && !report.handover ? (
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={openHandover}
              >
                Klasse übergeben
              </Button>
            ) : null}
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

        {!weekend && !isLoading && error ? (
          <LoadErrorAlert error={loadError} className="mt-4" />
        ) : null}

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
                morning={morning}
              />
              <Section
                title="Abgemeldet"
                count={absent.length}
                accent="text-[var(--class-day-danger)]"
                rows={absent}
                enrollmentKnown={false}
                now={at}
                morning={morning}
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
                morning={morning}
              />
              <Section
                title="Gehen nach Hause"
                count={leaving.length}
                rows={leaving}
                now={at}
                morning={morning}
              />
              {noCare.length > 0 && (
                <Section
                  title="Keine Betreuung"
                  count={noCare.length}
                  rows={noCare}
                  now={at}
                  morning={morning}
                />
              )}
              <Section
                title="Abgemeldet"
                count={absent.length}
                accent="text-[var(--class-day-danger)]"
                rows={absent}
                now={at}
                morning={morning}
              />
            </div>
          )}

        {/* Der Entwurf geht erst mit „Speichern“ hinaus (Bauart 2, #3112). */}
        {canRecordMorning && draftCount > 0 ? (
          <div className="mt-4 space-y-3 border-t border-gray-100 pt-4">
            <FormErrorAlert message={morningErrors.error} />
            <EditActions
              onCancel={discardMorning}
              onSave={() => void saveMorning()}
              saving={savingMorning}
              saveLabel="Anwesenheit speichern"
              cancelLabel="Verwerfen"
            />
          </div>
        ) : null}
      </section>

      {schoolClass && canReportClassTrip ? (
        <ClassTripBulkStatusModal
          isOpen={tripModalOpen}
          onClose={() => setTripModalOpen(false)}
          targetLabel={schoolClassLabel(schoolClass)}
          students={tripStudents}
          onSuccess={() => void refetchReport()}
          schoolReport={{ save: saveClassTrip, defaultDate: dateISO }}
        />
      ) : null}

      {canHandOver ? (
        <FormModal
          isOpen={handoverOpen}
          onClose={() => setHandoverOpen(false)}
          closeDisabled={savingHandover}
          title={`${schoolClassLabel(schoolClass)} an die OGS übergeben`}
          size="md"
          mobilePosition="bottom"
          error={handoverErrors.error}
          footer={
            <EditActions
              className="p-4"
              onCancel={() => setHandoverOpen(false)}
              onSave={() => void saveHandover()}
              saving={savingHandover}
              saveLabel="Übergeben"
              savingLabel="Übergibt…"
            />
          }
        >
          <div className="space-y-3 p-4">
            <p className="text-sm text-gray-600">
              {ogsRows.length > 0
                ? "Diese Kinder gehen jetzt in die OGS. Tippen Sie bei jedem Kind, das nicht da ist, auf „Nicht da“."
                : "Heute geht kein Kind aus dieser Klasse in die OGS."}
            </p>
            {ogsRows.length > 0 ? (
              <ul className="space-y-1.5">
                {ogsRows.map((row) => (
                  <li
                    key={row.student_id}
                    className="flex flex-wrap items-center justify-between gap-2"
                  >
                    <span className="min-w-0 truncate text-sm font-medium text-gray-900">
                      {row.last_name}, {row.first_name}
                    </span>
                    <SegmentedControl<HandoverChoice>
                      items={[
                        {
                          value: "present",
                          label: "Da",
                          tone: "green",
                          disabled: savingHandover,
                        },
                        {
                          value: "absent",
                          label: "Nicht da",
                          tone: "red",
                          disabled: savingHandover,
                        },
                      ]}
                      value={
                        handoverAbsent.has(row.student_id)
                          ? "absent"
                          : "present"
                      }
                      onChange={(choice) =>
                        markHandover(row.student_id, choice)
                      }
                      variant="pills"
                      ariaLabel={`Übergabe von ${row.first_name} ${row.last_name}`}
                      className="shrink-0"
                    />
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        </FormModal>
      ) : null}

      {canHandOver ? (
        <ConfirmationModal
          isOpen={withdrawOpen}
          onClose={() => setWithdrawOpen(false)}
          onConfirm={() => void confirmWithdraw()}
          title="Übergabe zurücknehmen?"
          confirmText="Zurücknehmen"
          isConfirmLoading={withdrawing}
        >
          <div className="space-y-3">
            <FormErrorAlert message={withdrawErrors.error} />
            <p className="text-sm text-gray-600">
              Die OGS sieht die Übergabe dann nicht mehr. Sie können die Klasse
              danach neu übergeben.
            </p>
          </div>
        </ConfirmationModal>
      ) : null}

      {schoolClass && canWriteArrivalException ? (
        <ClassArrivalExceptionDialog
          isOpen={exceptionDialogOpen}
          onClose={() => setExceptionDialogOpen(false)}
          schoolClass={schoolClass}
          defaultDate={weekend ? null : parseISODate(dateISO)}
          onChanged={() => void refetchReport()}
          weekendOpen={weekendOpen}
        />
      ) : null}
    </div>
  );
}
