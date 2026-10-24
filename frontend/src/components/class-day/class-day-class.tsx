"use client";

// Eine Klasse an einem Tag ("moto schule", #1772/#2294).
//
// Die Seite beantwortet genau eine Frage: wer aus dieser Klasse bleibt heute
// in der Betreuung, wer geht nach Hause, und bei wem ist es heute anders als
// sonst. Abweichungen stehen am Kind selbst, nicht in einem eigenen Block —
// jedes Kind kommt genau einmal vor. Heute trägt die Lehrkraft am Kind auch
// ein, ob es am Vormittag da ist (#4790). Hausaufgaben trägt sie für den Tag
// ein; die Rückmeldung der OGS steht am Kind (#4796).

import { ChevronLeft } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import NavigationLink from "~/components/ui/navigation-link";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { Skeleton } from "~/components/ui/skeleton";
import { Textarea } from "~/components/ui/textarea";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import type {
  ClassDayReport,
  ClassDayRow,
  MorningAttendanceStatus,
} from "~/lib/class-day-api";
import { formatDate, parseISODate, todayISO } from "~/lib/date-helpers";
import {
  HOMEWORK_DESCRIPTION_MAX_LENGTH,
  HOMEWORK_SUBJECT_MAX_LENGTH,
} from "~/lib/homework-feedback";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import type {
  ClassDayClasses,
  MorningAttendanceMark,
} from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { normalizeSchoolClass } from "~/lib/timetable-helpers";
import { ClassArrivalExceptionDialog } from "./class-arrival-exception-dialog";
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
import { type ClassHomeworkApi, useClassHomework } from "./use-class-homework";

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
   * Speichert die Anwesenheit am Vormittag (#4790). Ohne sie bleibt die
   * Anwesenheit eine Anzeige.
   */
  readonly recordMorningAttendance?: (
    schoolClass: string,
    date: string,
    marks: readonly MorningAttendanceMark[],
  ) => Promise<void>;
  /**
   * Trägt Hausaufgaben ein und ändert oder löscht eigene (#4796). Ohne sie
   * bleiben die Hausaufgaben eine Anzeige.
   */
  readonly homework?: ClassHomeworkApi;
  /** Nur für Tests: fixiert den Vergleichstag für "Heute gemeldet". */
  readonly now?: Date;
}

export function ClassDayClass({
  schoolClass,
  fetchClassDay,
  fetchClasses,
  recordMorningAttendance,
  homework,
  now,
}: ClassDayClassProps) {
  // Kein Default-Prop: eine `new Date()` in der Signatur bricht die
  // referenzielle Gleichheit bei jedem Render.
  const at = now ?? new Date();
  const searchParams = useSearchParams();
  const dateISO = classDayDateParam(searchParams.get("tag"));
  const weekendDay = isWeekendISO(dateISO);
  const [exceptionDialogOpen, setExceptionDialogOpen] = useState(false);

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
  // Hausaufgaben (#4796): nur mit Berechtigung und Schreibweg ein Knopf.
  const homeworkEntries = report?.homework ?? [];
  const canWriteHomework =
    Boolean(homework) && report?.can_write_homework === true && !weekend;
  const homeworkForm = useClassHomework({
    api: homework,
    schoolClass,
    dateISO,
    onChanged: () => refetchReport(),
  });
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

        {/* Hausaufgaben des Tages (#4796). Ein Satz sagt, wer sie sieht. */}
        {schoolClass && !weekend && report ? (
          <div className="mt-4 space-y-2 border-t border-gray-100 pt-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="min-w-0">
                <h3 className="text-sm font-semibold text-gray-900">
                  Hausaufgaben
                </h3>
                <p className="text-sm text-gray-600">
                  Die OGS sieht sie in der Lernzeit und meldet je Kind zurück.
                </p>
              </div>
              {canWriteHomework ? (
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={homeworkForm.openNew}
                >
                  Hausaufgaben eintragen
                </Button>
              ) : null}
            </div>
            {homeworkEntries.length === 0 ? (
              <p className="text-sm text-gray-500">
                Für diesen Tag sind keine Hausaufgaben eingetragen.
              </p>
            ) : (
              <ul className="space-y-1.5">
                {homeworkEntries.map((entry) => (
                  <li
                    key={entry.id}
                    className="moto-content-surface flex min-w-0 items-start justify-between gap-3 rounded-xl border px-3 py-2.5 shadow-sm"
                  >
                    <div className="min-w-0 flex-1">
                      {entry.subject ? (
                        <p className="text-sm font-medium text-gray-900">
                          {entry.subject}
                        </p>
                      ) : null}
                      <p className="text-sm break-words whitespace-pre-line text-gray-700">
                        {entry.description}
                      </p>
                    </div>
                    {canWriteHomework && entry.own ? (
                      <OverflowMenu
                        ariaLabel={`Aktionen zur Hausaufgabe ${entry.subject ?? ""}`.trim()}
                        triggerSize="sm"
                        items={[
                          {
                            label: "Ändern",
                            onClick: () => homeworkForm.openEdit(entry),
                          },
                          {
                            label: "Löschen",
                            destructive: true,
                            onClick: () => homeworkForm.askDelete(entry),
                          },
                        ]}
                      />
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
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

      {canWriteHomework ? (
        <FormModal
          isOpen={homeworkForm.draft !== null}
          onClose={homeworkForm.close}
          title={
            homeworkForm.draft?.editing
              ? "Hausaufgabe ändern"
              : `Hausaufgaben für ${formatDate(dateISO)}`
          }
          size="md"
          mobilePosition="bottom"
          closeDisabled={homeworkForm.saving}
          isBackdropDismissDisabled
          error={homeworkForm.formError}
          footer={
            <EditActions
              className="p-4"
              onCancel={homeworkForm.close}
              onSave={() => void homeworkForm.save()}
              saving={homeworkForm.saving}
            />
          }
        >
          <div className="space-y-4 p-4">
            <Input
              id="homework-subject"
              name="subject"
              label="Fach (freiwillig)"
              controlSize="compact"
              maxLength={HOMEWORK_SUBJECT_MAX_LENGTH}
              value={homeworkForm.draft?.subject ?? ""}
              onChange={(event) =>
                homeworkForm.update("subject", event.target.value)
              }
            />
            <Textarea
              id="homework-description"
              name="description"
              label="Was ist zu tun?"
              rows={4}
              maxLength={HOMEWORK_DESCRIPTION_MAX_LENGTH}
              value={homeworkForm.draft?.description ?? ""}
              error={homeworkForm.fieldError("description")}
              onChange={(event) =>
                homeworkForm.update("description", event.target.value)
              }
            />
          </div>
        </FormModal>
      ) : null}

      {canWriteHomework ? (
        <ConfirmDeleteModal
          isOpen={homeworkForm.deleting !== null}
          title="Hausaufgabe löschen?"
          description="Die OGS sieht diese Hausaufgabe danach nicht mehr."
          gate={{ mode: "twoStep" }}
          onConfirm={() => homeworkForm.confirmDelete()}
          onClose={homeworkForm.closeDelete}
          loading={homeworkForm.removing}
          error={homeworkForm.deleteError}
        />
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
