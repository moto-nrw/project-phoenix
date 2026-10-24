"use client";

// Eine Klasse an einem Tag ("moto schule", #1772/#2294).
//
// Die Seite beantwortet genau eine Frage: wer aus dieser Klasse bleibt heute
// in der Betreuung, wer geht nach Hause, und bei wem ist es heute anders als
// sonst. Abweichungen stehen am Kind selbst, nicht in einem eigenen Block —
// jedes Kind kommt genau einmal vor.

import { ChevronLeft } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import NavigationLink from "~/components/ui/navigation-link";
import { Skeleton } from "~/components/ui/skeleton";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import type { ClassDayHint, ClassDayReport } from "~/lib/class-day-api";
import { formatDate, parseISODate, todayISO } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  type ClassDayClasses,
  type ClassTripReportInput,
  type ClassTripReportResult,
  deleteDayHintSchool,
  saveDayHintSchool,
} from "~/lib/school-class-day-api";
import { schoolClassLabel } from "~/lib/school-class-label";
import { schoolPath } from "~/lib/school-url";
import { useSWRAuth } from "~/lib/swr";
import { normalizeSchoolClass } from "~/lib/timetable-helpers";
import { ClassTripBulkStatusModal } from "~/components/students/class-trip-bulk-status-modal";
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

/** Höchstlänge eines Hinweises an die OGS (#4795), wie im Backend. */
const DAY_HINT_MAX_LENGTH = 500;

/** Ziel des Hinweises: die ganze Klasse oder ein Kind (seine ID als Text). */
const CLASS_TARGET = "class";

/** Der Verfasser eines Hinweises, wenn er nicht von der Lehrkraft selbst ist. */
function dayHintAuthorSuffix(hint: ClassDayHint): string {
  if (hint.own || !hint.author_name) return "";
  return ` (von ${hint.author_name})`;
}

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
  /** Nur für Tests: fixiert den Vergleichstag für "Heute gemeldet". */
  readonly now?: Date;
}

export function ClassDayClass({
  schoolClass,
  fetchClassDay,
  fetchClasses,
  reportClassTrip,
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
  // Hinweis an die OGS (#4795): Ziel, Text und der Ablauf von Speichern und
  // Löschen. Fehler stehen im Dialog: das Schul-Portal hat keine Toasts.
  const [hintOpen, setHintOpen] = useState(false);
  const [hintTarget, setHintTarget] = useState(CLASS_TARGET);
  const [hintBody, setHintBody] = useState("");
  const [hintSaving, setHintSaving] = useState(false);
  const [hintDeleteOpen, setHintDeleteOpen] = useState(false);
  const [hintDeleting, setHintDeleting] = useState(false);
  const hintErrors = useApiFormError();
  const hintDeleteErrors = useApiFormError();

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
  const classHints = report && !weekend ? (report.class_day_hints ?? []) : [];
  const canWriteDayHints =
    report?.can_write_day_hints === true && isAssignedClass && !weekend;
  // Nur Kinder mit OGS-Datensatz: ein Klassenlisteneintrag hat keine
  // Kindakte, an der ein Hinweis stehen könnte.
  const hintChildren = rows.filter(
    (row) => !row.list_entry && row.student_id > 0,
  );
  const hintOptions = [
    {
      value: CLASS_TARGET,
      // schoolClassLabel beginnt immer mit „Klasse“: „Die ganze Klasse 3b“.
      label: `Die ganze ${schoolClassLabel(schoolClass)}`,
    },
    ...hintChildren.map((row) => ({
      value: String(row.student_id),
      label: `${row.last_name}, ${row.first_name}`,
    })),
  ];
  const ownHintFor = (target: string): ClassDayHint | undefined => {
    if (target === CLASS_TARGET) return classHints.find((hint) => hint.own);
    const row = hintChildren.find(
      (child) => String(child.student_id) === target,
    );
    return row?.day_hints?.find((hint) => hint.own);
  };
  const existingHint = ownHintFor(hintTarget);
  const trimmedHint = hintBody.trim();
  const hintStudentId =
    hintTarget === CLASS_TARGET ? undefined : Number(hintTarget);

  const chooseHintTarget = (target: string) => {
    setHintTarget(target);
    setHintBody(ownHintFor(target)?.body ?? "");
    hintErrors.clear();
  };
  const openHint = () => {
    chooseHintTarget(CLASS_TARGET);
    setHintOpen(true);
  };
  const closeHint = () => {
    setHintOpen(false);
    hintErrors.clear();
  };
  const saveHint = async () => {
    if (trimmedHint === "") {
      hintErrors.invalid("Bitte schreiben Sie einen Hinweis.", {
        day_hint: "Bitte schreiben Sie einen Hinweis.",
      });
      return;
    }
    setHintSaving(true);
    hintErrors.clear();
    try {
      await saveDayHintSchool(schoolClass, dateISO, trimmedHint, hintStudentId);
      setHintOpen(false);
      await refetchReport();
    } catch (err) {
      logger.error("class_day_hint_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await hintErrors.show(err, {
        object: "der Hinweis",
        retry: () => void saveHint(),
      });
    } finally {
      setHintSaving(false);
    }
  };
  const deleteHint = async () => {
    setHintDeleting(true);
    hintDeleteErrors.clear();
    try {
      await deleteDayHintSchool(schoolClass, dateISO, hintStudentId);
      setHintDeleteOpen(false);
      setHintOpen(false);
      await refetchReport();
    } catch (err) {
      logger.error("class_day_hint_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Der Dialog bleibt offen; ein zweiter Klick ist der neue Versuch.
      await hintDeleteErrors.show(err, { object: "der Hinweis" });
    } finally {
      setHintDeleting(false);
    }
  };

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
        {/* Hinweise an die OGS für die ganze Klasse (#4795); die Hinweise
            für ein Kind stehen am Kind. */}
        {classHints.map((hint) => (
          <p
            key={`${hint.author_name ?? ""}-${hint.updated_at}`}
            className="mt-1 text-sm break-words text-gray-900"
          >
            <span className="font-medium">Hinweis an die OGS:</span> {hint.body}
            {dayHintAuthorSuffix(hint)}
          </p>
        ))}
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
        {/* Hinweis an die OGS (#4795): nur heute und nur mit Freigabe. */}
        {schoolClass && canWriteDayHints ? (
          <div className="mt-3">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={openHint}
            >
              Hinweis für die OGS
            </Button>
          </div>
        ) : null}
        {tripSaved ? (
          <Alert type="success" message={tripSaved} className="mt-3" />
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

      {schoolClass && canWriteDayHints ? (
        <FormModal
          isOpen={hintOpen}
          suspended={hintDeleteOpen}
          onClose={closeHint}
          title="Hinweis für die OGS"
          size="md"
          mobilePosition="bottom"
          error={hintErrors.error}
          isBackdropDismissDisabled={trimmedHint !== ""}
          footer={
            <div className="flex flex-wrap items-center justify-between gap-2 p-4">
              {existingHint ? (
                <Button
                  type="button"
                  variant="outline_danger"
                  size="md"
                  onClick={() => {
                    hintDeleteErrors.clear();
                    setHintDeleteOpen(true);
                  }}
                  disabled={hintSaving}
                >
                  Hinweis löschen
                </Button>
              ) : (
                <span />
              )}
              <EditActions
                onCancel={closeHint}
                onSave={() => void saveHint()}
                saving={hintSaving}
              />
            </div>
          }
        >
          <div className="space-y-4 p-4">
            <p className="text-sm text-gray-600">
              Die OGS sieht den Hinweis heute am Kind, in Aufsicht, Gruppe und
              Kindersuche. Morgen ist er weg.
            </p>
            <div>
              <label
                id="day-hint-target-label"
                htmlFor="day-hint-target"
                className="mb-2 block text-sm font-medium text-gray-700"
              >
                Für wen ist der Hinweis?
              </label>
              <CustomSelect
                value={hintTarget}
                options={hintOptions}
                onChange={chooseHintTarget}
                id="day-hint-target"
                ariaLabelledBy="day-hint-target-label"
              />
            </div>
            <div>
              <Textarea
                id="day-hint-body"
                name="day_hint"
                label="Hinweis"
                rows={4}
                maxLength={DAY_HINT_MAX_LENGTH}
                value={hintBody}
                onChange={(event) => setHintBody(event.target.value)}
                placeholder="Zum Beispiel: Hat heute Bauchweh, bitte im Blick behalten."
                error={hintErrors.fieldError("day_hint")}
              />
              <p className="mt-1 text-right text-xs text-gray-500 tabular-nums">
                {hintBody.length} von {DAY_HINT_MAX_LENGTH} Zeichen
              </p>
            </div>
          </div>
        </FormModal>
      ) : null}

      <ConfirmDeleteModal
        isOpen={hintDeleteOpen}
        title="Hinweis löschen?"
        description={
          <p>
            Die OGS sieht den Hinweis danach nicht mehr. Sie können jederzeit
            einen neuen schreiben.
          </p>
        }
        gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
        confirmLabel="Endgültig löschen"
        loadingLabel="Wird gelöscht…"
        loading={hintDeleting}
        error={hintDeleteErrors.error?.message ?? ""}
        onConfirm={() => void deleteHint()}
        onClose={() => setHintDeleteOpen(false)}
        mobileSheet
      />
    </div>
  );
}
