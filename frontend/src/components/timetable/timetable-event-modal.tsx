"use client";

import { Trash2 } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { useModal } from "~/components/dashboard/modal-context";
import { ClosingDayConfirmModal } from "~/components/planning/closing-day-marker";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { ISODatePicker } from "~/components/ui/date-picker";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import {
  findFirstClosingDayConflict,
  type ClosingDayConflict,
  type ClosingDayRange,
} from "~/lib/closing-day-helpers";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { materializedRecurrenceDates } from "~/lib/timetable-helpers";
import { CategoryManageModal } from "./category-manage-modal";
import { PlanningTrackManageModal } from "./planning-track-manage-modal";
import { Field } from "./event-form/field";
import type { EventFormState, RepeatMode } from "./event-form/form-model";
import { StepPersonalKinder } from "./event-form/step-personal-kinder";
import { StepTermin } from "./event-form/step-termin";
import { StepWiederholung } from "./event-form/step-wiederholung";
import { useEventForm } from "./event-form/use-event-form";
import type { TimetableEventModalResult } from "./event-form/use-event-form";
import type {
  EditedChange,
  EnrichedInstance,
  TimetableTemplate,
} from "~/lib/timetable-types";

// #1875: German labels for the field categories a single-occurrence edit can
// change (backend EditedChange strings). Shown in the lost-edits warning.
const EDIT_CHANGE_LABELS: Record<EditedChange, string> = {
  title: "Titel",
  description: "Beschreibung",
  notes: "Notiz",
  room: "Raum",
  time: "Zeit/Datum",
  staff: "Personal",
  students: "Kinder",
  list_kind: "Listenart",
  deleted: "Gelöschter Termin",
};

const WIZARD_STEPS = ["Termin", "Wiederholung", "Personal und Kinder"] as const;

/**
 * Which validateForm() field errors belong to which wizard step. "Weiter" only
 * blocks on the current step's fields; a failed Speichern jumps to the first
 * step that actually shows the offending error. The rules themselves stay in
 * the hook's validateForm — this is only a mapping.
 */
const STEP_FIELDS: readonly (readonly (keyof EventFormState)[])[] = [
  ["title", "date", "startTime", "endTime", "roomId", "categoryId"],
  ["weekdays", "calendarPeriodId", "weekPattern"],
  ["targetGradeLevel", "targetSchoolClass", "educationGroupId"],
];

const LAST_STEP = WIZARD_STEPS.length - 1;

function getClosingDayWarningMessage(
  isSeriesFlow: boolean,
  conflict: ClosingDayConflict,
): string {
  let reason = "";
  if (conflict.reason !== "") reason = ` (${conflict.reason})`;

  const date = formatDate(conflict.dateISO);
  if (isSeriesFlow) {
    return `Hinweis: Der Regeltermin fällt am ${date} auf einen Schließtag${reason}. Planen ist weiterhin möglich.`;
  }
  return `Hinweis: Am ${date} ist ein Schließtag hinterlegt${reason}. Planen ist weiterhin möglich.`;
}

interface TimetableEventModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved: (result: TimetableEventModalResult) => void;
  defaultDate: string;
  weekFrom?: string;
  weekTo?: string;
  calendarPeriods: CalendarPeriod[];
  defaultCalendarPeriodId?: string | null;
  showPeriodField?: boolean;
  initialInstance?: EnrichedInstance | null;
  initialSeries?: TimetableTemplate | null;
  convertInstance?: EnrichedInstance | null;
  onDeleteSeries?: (
    template: TimetableTemplate,
    effectiveDate: string,
  ) => Promise<void>;
  defaultRepeat?: RepeatMode;
  /**
   * "quick" starts collapsed: the repeat step offers a plain-language preset
   * select instead of the full series controls, and Typ/Kategorie/Zielgruppe
   * stay hidden until "Benutzerdefiniert" expands the form. Default "full".
   */
  variant?: "full" | "quick";
  defaultStartTime?: string;
  defaultEndTime?: string;
  canCheckShiftCoverage: boolean;
  canManageCategories: boolean;
  canManagePlanningTracks?: boolean;
  /**
   * OGS-Schließtage des Tenants (#2032). Fällt das gewählte Datum auf einen
   * davon, fragt das Speichern einmal nach — angelegt wird trotzdem, sobald
   * bestätigt wurde. Ohne die Liste verhält sich der Dialog wie bisher.
   */
  closingDayRanges?: readonly ClosingDayRange[];
  /** Saving stays disabled until the range lookup completes, so an empty
   *  loading state cannot be mistaken for a conflict-free date. */
  closingDaysLoading?: boolean;
}

export function TimetableEventModal({
  isOpen,
  onClose,
  onSaved,
  defaultDate,
  weekFrom,
  weekTo,
  calendarPeriods,
  defaultCalendarPeriodId,
  showPeriodField = false,
  initialInstance = null,
  initialSeries = null,
  convertInstance = null,
  onDeleteSeries,
  defaultRepeat = "none",
  variant = "full",
  defaultStartTime,
  defaultEndTime,
  canCheckShiftCoverage,
  canManageCategories,
  canManagePlanningTracks = false,
  closingDayRanges,
  closingDaysLoading = false,
}: TimetableEventModalProps) {
  const { isModalOpen } = useModal();
  const [categoryDialog, setCategoryDialog] = useState<
    "list" | "create" | null
  >(null);
  const [planningTrackDialog, setPlanningTrackDialog] = useState<
    "list" | "create" | null
  >(null);
  const {
    form,
    update,
    updateRepeat,
    selectCalendarPeriod,
    toggleWeekday,
    changeTargetGroupType,
    fieldErrors,
    validationError,
    rooms,
    categories,
    refreshCategories,
    planningTracks,
    refreshPlanningTracks,
    groups,
    students,
    staff,
    loadingRefs,
    loadingStudents,
    studentLoadError,
    loadingStaff,
    staffLoadError,
    retryStudentLoad,
    retryStaffLoad,
    submitting,
    handleSubmit,
    validateForm,
    lastValidationErrors,
    deleteConfirmOpen,
    setDeleteConfirmOpen,
    deleteEffectiveDate,
    setDeleteEffectiveDate,
    deleteError,
    setDeleteError,
    deletingSeries,
    openSeriesDeleteConfirm,
    handleConfirmSeriesDelete,
    expanded,
    choiceDialogOpen,
    setPendingSeriesEdit,
    handleScopeSelect,
    scopeClosingDayWarning,
    setScopeClosingDayWarning,
    confirmScopeClosingDay,
    lostEdits,
    setLostEdits,
    confirmLostEdits,
    conflictWarnings,
    coverageWarnings,
    coverageWarningCount,
    coverageCheckError,
    isEditingInstance,
    isEditingSeries,
    isSeriesFlow,
    canDeleteSeries,
    gradeLevelMax,
    targetGradeOptions,
    preservesGradeAboveTenantCap,
    biweeklyUnavailable,
    abWeekHint,
    studentBulkOptions,
    targetClassOptions,
    targetClassDescriptionIDs,
    targetCohort,
    missingTargetCohortCount,
    targetCohortButtonLabel,
    addTargetCohort,
    dateWeekday,
    dateWeekdayName,
    quickPreset,
    handleQuickPresetChange,
    dateChanged,
    notesChanged,
    title,
    requiredStaffTouched,
    staffRosterTouched,
    listKindTouched,
    manualWeekPattern,
  } = useEventForm({
    isOpen,
    onClose,
    onSaved,
    defaultDate,
    weekFrom,
    weekTo,
    calendarPeriods,
    defaultCalendarPeriodId,
    initialInstance,
    initialSeries,
    convertInstance,
    onDeleteSeries,
    defaultRepeat,
    variant,
    defaultStartTime,
    defaultEndTime,
    canCheckShiftCoverage,
    closingDayRanges,
  });

  // Converting a one-off into a Regeltermin is a repeat decision — that entry
  // opens on step 2. Every other entry (quick create, "+ Neu → Regeltermin",
  // instance edit, series edit) starts at step 1 with all steps reachable.
  const [step, setStep] = useState(0);
  const submitAttempted = useRef(false);
  const formRef = useRef<HTMLFormElement>(null);
  useEffect(() => {
    if (isOpen) {
      setStep(convertInstance ? 1 : 0);
      submitAttempted.current = false;
      confirmedClosingConflict.current = null;
      setClosingDayPrompt(null);
    }
  }, [isOpen, convertInstance]);

  // Schließtag-Warnung (#2032). Bei Serien zählt nicht nur das Ankerdatum:
  // dieselben Wochentag-/A-B-Regeln wie beim Materialisieren werden über den
  // gewählten Zeitraum gelegt. Bestätigt wird die aktuelle Konfiguration;
  // ändert sie sich danach, fragt der Dialog erneut.
  const closingDayConflict = useMemo(() => {
    if (!isSeriesFlow) {
      return findFirstClosingDayConflict(closingDayRanges, [form.date]);
    }
    const period = calendarPeriods.find(
      (candidate) => candidate.id === form.calendarPeriodId,
    );
    if (!period) return null;
    const today = berlinTodayISO();
    // Serien-Edits schauen ab heute nach vorn. Neue und umgewandelte Serien
    // beginnen am gewählten Datum (#2135): frühere Slots werden nie
    // materialisiert und dürfen die Rückfrage nicht auslösen.
    // materializedRecurrenceDates klemmt beide Untergrenzen auf den
    // Periodenstart.
    const from = initialSeries ? today : form.date;
    const validity = initialSeries?.schedules[0];
    const dates = materializedRecurrenceDates({
      period,
      fromISO: from,
      weekdays: form.weekdays,
      weekPattern: form.weekPattern,
      validFrom: validity?.validFrom,
      validUntil: validity?.validUntil,
    });
    // Converting preserves the concrete seed occurrence even when its date is
    // outside the selected recurrence slots.
    if (convertInstance && form.date && !dates.includes(form.date)) {
      dates.push(form.date);
      dates.sort((left, right) => left.localeCompare(right));
    }
    return findFirstClosingDayConflict(closingDayRanges, dates);
  }, [
    calendarPeriods,
    closingDayRanges,
    convertInstance,
    form.calendarPeriodId,
    form.date,
    form.weekPattern,
    form.weekdays,
    initialSeries,
    isSeriesFlow,
  ]);
  const closingDayConfirmationKey =
    closingDayConflict === null
      ? null
      : JSON.stringify({
          conflict: closingDayConflict,
          repeat: form.repeat,
          weekdays: form.weekdays,
          calendarPeriodId: form.calendarPeriodId,
          weekPattern: form.weekPattern,
        });
  const [closingDayPrompt, setClosingDayPrompt] = useState<{
    conflict: ClosingDayConflict;
    confirmationKey: string;
  } | null>(null);
  const confirmedClosingConflict = useRef<string | null>(null);
  // Nach der Bestätigung ganz normal absenden: die Serienkonfiguration steht
  // dann im Ref, die Rückfrage greift also nicht erneut.
  const submitAfterConfirm = () => {
    if (!closingDayPrompt) return;
    confirmedClosingConflict.current = closingDayPrompt.confirmationKey;
    setClosingDayPrompt(null);
    formRef.current?.requestSubmit();
  };

  const stepHasError = (index: number, errors: Record<string, string>) =>
    (STEP_FIELDS[index] ?? []).some((field) => errors[field] !== undefined);

  const goNext = () => {
    // Reuses the hook's unchanged validateForm and only looks at the fields of
    // the current step — no separate rule set.
    validateForm();
    const errors = lastValidationErrors.current;
    if (stepHasError(step, errors)) return;
    // A repeat choice in step 2 retroactively makes step-1 fields required (a
    // Regeltermin needs a Kategorie). Once the current step is clean, block on
    // an earlier step's error too and jump to it, instead of waving the user
    // through to Speichern and only revealing the field there.
    const earlier = STEP_FIELDS.findIndex(
      (_, index) => index < step && stepHasError(index, errors),
    );
    if (earlier >= 0) {
      setStep(earlier);
      return;
    }
    setStep((current) => Math.min(current + 1, LAST_STEP));
  };

  // Speichern only exists on the last step, but the full validation covers
  // every step. When it fails on a field the user cannot see, surface it by
  // jumping to its step.
  useEffect(() => {
    if (!submitAttempted.current) return;
    submitAttempted.current = false;
    if (!stepHasError(step, fieldErrors)) {
      const target = STEP_FIELDS.findIndex((_, index) =>
        stepHasError(index, fieldErrors),
      );
      if (target >= 0) setStep(target);
    }
    // `step` is read, not tracked: only a fresh validation result may move it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fieldErrors]);

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SlideOverContent
        widthClass="sm:w-[760px]"
        // The ChoiceModal portals to document.body and lives outside the
        // drawer's DOM. Without these guards Vaul's DismissableLayer treats
        // every click inside the open dialog as an outside-click and closes
        // the slide-over, unmounting the dialog before its buttons can
        // fire. See issue #1358. `submitting` additionally blocks dismissal
        // mid-save (both the form submit and the scope flow set it).
        onInteractOutside={(event) => {
          if (isModalOpen || choiceDialogOpen || submitting) {
            event.preventDefault();
          }
        }}
        onEscapeKeyDown={(event) => {
          if (isModalOpen || choiceDialogOpen || submitting) {
            event.preventDefault();
          }
        }}
      >
        <SlideOverHeader>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <SlideOverTitle>{title}</SlideOverTitle>
              <SlideOverDescription>
                {isSeriesFlow
                  ? "Regelmäßigen Termin mit Kindern und Personal planen."
                  : isEditingInstance
                    ? "Termin im Betreuungsplan bearbeiten."
                    : "Einmaligen Termin im Betreuungsplan anlegen."}
              </SlideOverDescription>
            </div>
            <SlideOverCloseButton />
          </div>
        </SlideOverHeader>

        <div className="border-b border-gray-200 px-5 py-3">
          <WizardStepper steps={[...WIZARD_STEPS]} current={step} />
        </div>

        <form
          id="timetable-event-form"
          ref={formRef}
          noValidate
          onSubmit={(event) => {
            // Before the last step the submit button is "Weiter", so every
            // submit — the click and the implicit one Enter triggers in a
            // field — advances the wizard instead of saving. (#2025)
            if (step < LAST_STEP) {
              event.preventDefault();
              goNext();
              return;
            }
            if (closingDaysLoading) {
              event.preventDefault();
              return;
            }
            // Schließtag: erst nachfragen, dann speichern (#2032). Die Frage
            // kommt erst, wenn das Formular auch wirklich speichern würde —
            // sonst stünde sie vor den Pflichtfeld-Fehlern.
            if (
              closingDayConflict !== null &&
              closingDayConfirmationKey !== null &&
              confirmedClosingConflict.current !== closingDayConfirmationKey &&
              !submitting &&
              validateForm()
            ) {
              event.preventDefault();
              setClosingDayPrompt({
                conflict: closingDayConflict,
                confirmationKey: closingDayConfirmationKey,
              });
              return;
            }
            // Mirror handleSubmit's early-return guards: on those paths no
            // validation runs, so the flag would stay set and a later,
            // unrelated fieldErrors change could trigger a spurious step jump.
            if (
              !submitting &&
              !(isEditingInstance && initialInstance?.status !== "planned")
            ) {
              submitAttempted.current = true;
            }
            void handleSubmit(event);
          }}
          className="flex-1 overflow-y-auto px-5 py-4"
        >
          <div className="flex flex-col gap-5">
            {initialInstance && initialInstance.status !== "planned" && (
              <Alert
                type="error"
                message="Nur geplante Termine können bearbeitet werden."
              />
            )}

            {isEditingSeries && (
              <p className="text-xs text-gray-500">
                Änderungen gelten für alle Termine dieser Serie.
              </p>
            )}

            {step === 0 && (
              <StepTermin
                form={form}
                update={update}
                fieldErrors={fieldErrors}
                rooms={rooms}
                categories={categories}
                planningTracks={planningTracks}
                loadingRefs={loadingRefs}
                expanded={expanded}
                isSeriesFlow={isSeriesFlow}
                isEditingSeries={isEditingSeries}
                quickPreset={quickPreset}
                listKindTouched={listKindTouched}
                canManageCategories={canManageCategories}
                onManageCategories={setCategoryDialog}
                canManagePlanningTracks={canManagePlanningTracks}
                onManagePlanningTracks={setPlanningTrackDialog}
              />
            )}

            {step === 1 && (
              <StepWiederholung
                form={form}
                update={update}
                updateRepeat={updateRepeat}
                selectCalendarPeriod={selectCalendarPeriod}
                toggleWeekday={toggleWeekday}
                fieldErrors={fieldErrors}
                calendarPeriods={calendarPeriods}
                showPeriodField={showPeriodField}
                expanded={expanded}
                isSeriesFlow={isSeriesFlow}
                isEditingSeries={isEditingSeries}
                biweeklyUnavailable={biweeklyUnavailable}
                abWeekHint={abWeekHint}
                quickPreset={quickPreset}
                handleQuickPresetChange={handleQuickPresetChange}
                dateWeekday={dateWeekday}
                dateWeekdayName={dateWeekdayName}
                manualWeekPattern={manualWeekPattern}
              />
            )}

            {step === 2 && (
              <StepPersonalKinder
                form={form}
                update={update}
                changeTargetGroupType={changeTargetGroupType}
                fieldErrors={fieldErrors}
                groups={groups}
                students={students}
                staff={staff}
                loadingRefs={loadingRefs}
                loadingStudents={loadingStudents}
                studentLoadError={studentLoadError}
                loadingStaff={loadingStaff}
                staffLoadError={staffLoadError}
                retryStudentLoad={retryStudentLoad}
                retryStaffLoad={retryStaffLoad}
                expanded={expanded}
                isSeriesFlow={isSeriesFlow}
                gradeLevelMax={gradeLevelMax}
                targetGradeOptions={targetGradeOptions}
                preservesGradeAboveTenantCap={preservesGradeAboveTenantCap}
                studentBulkOptions={studentBulkOptions}
                targetClassOptions={targetClassOptions}
                targetClassDescriptionIDs={targetClassDescriptionIDs}
                targetCohort={targetCohort}
                missingTargetCohortCount={missingTargetCohortCount}
                targetCohortButtonLabel={targetCohortButtonLabel}
                addTargetCohort={addTargetCohort}
                conflictWarnings={conflictWarnings}
                coverageWarnings={coverageWarnings}
                coverageWarningCount={coverageWarningCount}
                coverageCheckError={coverageCheckError}
                requiredStaffTouched={requiredStaffTouched}
                staffRosterTouched={staffRosterTouched}
              />
            )}

            {/* Roster load failures must be visible right away, not only on
                step 3 (the old single-page form auto-expanded to reveal
                them). Step 3 renders its own detailed panels with retry
                buttons, so skip the duplicate alerts there. */}
            {step !== 2 && studentLoadError && (
              <Alert type="warning" message={studentLoadError} />
            )}
            {step !== 2 && staffLoadError && (
              <Alert type="warning" message={staffLoadError} />
            )}

            {/* Speichern works from every step, so conflict and coverage
                hints must too — otherwise a direct save from step 1 hides
                room/staff/student overlaps. Step 3 keeps its own detailed
                panels (with the expandable coverage list); here only the
                compact advisory form is shown. */}
            {step !== 2 &&
              (conflictWarnings.length > 0 ||
                coverageWarningCount > 0 ||
                coverageCheckError) && (
                <div className="flex flex-col gap-2">
                  <p className="sr-only" aria-live="polite">
                    {`${conflictWarnings.length} Terminüberschneidungen und ${coverageWarningCount} Dienstplan-Lücken gefunden. Speichern ist weiterhin möglich.`}
                  </p>
                  {conflictWarnings.map((warning) => (
                    <Alert
                      key={`${warning.kind}-${warning.resourceId}-${warning.conflictingInstanceId}`}
                      type="warning"
                      message={`Hinweis: ${warning.message}`}
                      announce="off"
                    />
                  ))}
                  {coverageWarningCount > 0 && (
                    <Alert
                      type="warning"
                      message={`${coverageWarningCount} Dienstplan-${coverageWarningCount === 1 ? "Lücke" : "Lücken"} gefunden. Details im Schritt „Personal und Kinder“. Speichern ist weiterhin möglich.`}
                      announce="off"
                    />
                  )}
                  {coverageCheckError && (
                    <Alert
                      type="warning"
                      message={`Hinweis: ${coverageCheckError}`}
                      announce="off"
                    />
                  )}
                </div>
              )}

            {/* Schließtag in der aktuellen Termin-/Serienkonfiguration (#2032)
                — Hinweis, keine Sperre. Beim Speichern folgt die Rückfrage. */}
            {closingDayConflict !== null && (
              <Alert
                type="warning"
                message={getClosingDayWarningMessage(
                  isSeriesFlow,
                  closingDayConflict,
                )}
                announce="off"
              />
            )}

            {validationError && (
              <Alert type="error" message={validationError} />
            )}
          </div>
        </form>

        <SlideOverFooter className="items-stretch gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="order-2 sm:order-1">
            {canDeleteSeries && (
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                onClick={openSeriesDeleteConfirm}
                disabled={submitting || deletingSeries}
                className="w-full sm:w-auto"
              >
                <span className="inline-flex items-center gap-2">
                  <Trash2 className="h-4 w-4" aria-hidden />
                  Löschen
                </span>
              </Button>
            )}
          </div>
          <div className="order-1 flex items-center justify-end gap-3 sm:order-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={onClose}
              disabled={submitting || deletingSeries}
            >
              Abbrechen
            </Button>
            {step > 0 && (
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => setStep((current) => Math.max(current - 1, 0))}
                disabled={submitting || deletingSeries}
              >
                Zurück
              </Button>
            )}
            {step < LAST_STEP ? (
              // #2025: "Weiter" is the primary action so the eye lands on the
              // path through the wizard; Speichern only exists on the last
              // step, so no step can be saved unseen. It is the form's submit
              // button (not just an onClick): a form without one suppresses
              // implicit submission, so Enter in a field would do nothing at
              // all. This way click and Enter take the same path — onSubmit,
              // which routes everything before the last step into goNext.
              <Button
                type="submit"
                form="timetable-event-form"
                variant="primary"
                size="md"
                disabled={submitting || deletingSeries}
              >
                Weiter
              </Button>
            ) : (
              <Button
                type="submit"
                form="timetable-event-form"
                variant="primary"
                size="md"
                isLoading={submitting}
                loadingText="Speichere …"
                disabled={
                  submitting ||
                  deletingSeries ||
                  closingDaysLoading ||
                  (isEditingInstance && initialInstance?.status !== "planned")
                }
              >
                Speichern
              </Button>
            )}
          </div>
        </SlideOverFooter>

        {initialSeries && (
          <ConfirmationModal
            isOpen={deleteConfirmOpen}
            onClose={() => {
              if (!deletingSeries) setDeleteConfirmOpen(false);
            }}
            onConfirm={() => void handleConfirmSeriesDelete()}
            title="Regeltermin löschen?"
            confirmText="Löschen"
            cancelText="Abbrechen"
            isConfirmLoading={deletingSeries}
            confirmButtonClass="bg-[#FF3130] hover:bg-[#CC2626]"
          >
            <div className="flex flex-col gap-4">
              <p className="text-sm leading-relaxed text-gray-600">
                Der Regeltermin wird ab dem gewählten Datum gelöscht. Frühere
                Termine bleiben erhalten.
              </p>
              <Field
                label="Ab Datum"
                htmlFor="series_delete_effective_date"
                required
                error={deleteError ?? undefined}
              >
                <ISODatePicker
                  id="series_delete_effective_date"
                  controlSize="md"
                  value={deleteEffectiveDate}
                  min={berlinTodayISO()}
                  invalid={Boolean(deleteError)}
                  disabled={deletingSeries}
                  calendarLayout="popover"
                  onChange={(next) => {
                    setDeleteEffectiveDate(next);
                    setDeleteError(null);
                  }}
                />
              </Field>
            </div>
          </ConfirmationModal>
        )}

        {initialInstance && (
          <ChoiceModal
            isOpen={choiceDialogOpen && scopeClosingDayWarning === null}
            onClose={() => setPendingSeriesEdit(null)}
            title="Wiederholenden Termin ändern"
            description={
              `Der Termin am ${formatDate(initialInstance.date)} gehört zu einem Regeltermin.` +
              (dateChanged || notesChanged
                ? " Geändertes Datum und Tagesnotiz gelten nur bei „Nur diese Woche“; die Wochennotiz der Terminreihe bleibt bei allen Optionen erhalten."
                : "")
            }
            options={[
              {
                value: "single",
                label: "Nur diese Woche",
                description: `Ändert nur den Termin am ${formatDate(form.date)}; der Regeltermin bleibt unverändert.`,
              },
              {
                value: "following",
                label: "Ab jetzt dauerhaft",
                description: `Ändert diesen und alle künftigen Termine ab dem ${formatDate(initialInstance.date)} dauerhaft; frühere Termine bleiben unverändert.`,
              },
              {
                value: "all",
                label: "Alle Termine der Serie",
                description:
                  "Ändert den Regeltermin und baut künftige geplante Termine neu auf.",
              },
            ]}
            onSelect={(value) => void handleScopeSelect(value)}
            isBusy={submitting}
          />
        )}

        {/* #2032: bestätigbare Warnung, bevor ein Termin auf einem Schließtag
            gespeichert wird. */}
        {closingDayPrompt !== null && (
          <ClosingDayConfirmModal
            dateISO={closingDayPrompt.conflict.dateISO}
            reason={closingDayPrompt.conflict.reason}
            subject="termin"
            onCancel={() => setClosingDayPrompt(null)}
            onConfirm={submitAfterConfirm}
          />
        )}

        {scopeClosingDayWarning !== null && (
          <ClosingDayConfirmModal
            dateISO={scopeClosingDayWarning.conflict.dateISO}
            reason={scopeClosingDayWarning.conflict.reason}
            subject="termin"
            onCancel={() => setScopeClosingDayWarning(null)}
            onConfirm={() => void confirmScopeClosingDay()}
          />
        )}

        {/* #1875: warn before a series edit discards single-occurrence edits. */}
        <ConfirmationModal
          isOpen={lostEdits !== null}
          onClose={() => {
            if (!submitting) setLostEdits(null);
          }}
          onConfirm={() => void confirmLostEdits()}
          title="Einzelanpassungen gehen verloren"
          confirmText="Trotzdem fortfahren"
          cancelText="Abbrechen"
          isConfirmLoading={submitting}
          confirmButtonClass="bg-[#F78C10] hover:bg-[#d97908]"
        >
          {lostEdits && (
            <div className="flex flex-col gap-3">
              <p className="text-sm leading-relaxed text-gray-600">
                {lostEdits.result.count === 1
                  ? "1 Termin dieser Serie wurde einzeln angepasst. "
                  : `${lostEdits.result.count} Termine dieser Serie wurden einzeln angepasst. `}
                {lostEdits.scope === "following"
                  ? "Wenn du diesen und alle folgenden Termine bearbeitest, gehen diese Anpassungen verloren:"
                  : "Wenn du alle Termine der Serie bearbeitest, gehen diese Anpassungen verloren:"}
              </p>
              <ul className="max-h-52 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-2 text-sm">
                {lostEdits.result.occurrences.map((occ) => (
                  <li
                    // Deleted occurrences share instanceId "0"; date+start keeps
                    // the key unique (one deletion per date, one slot per start).
                    key={`${occ.instanceId}-${occ.date}-${occ.startTime}`}
                    className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 px-1 py-1"
                  >
                    <span className="font-semibold text-gray-800">
                      {formatDate(occ.date)}
                    </span>
                    <span className="text-gray-500">
                      {occ.changes
                        .map((c) => EDIT_CHANGE_LABELS[c] ?? c)
                        .join(", ")}
                    </span>
                  </li>
                ))}
              </ul>
              <p className="text-xs leading-relaxed text-gray-500">
                Notiere dir diese Termine, um sie anschließend erneut
                anzupassen.
              </p>
            </div>
          )}
        </ConfirmationModal>

        {/* Kategorien verwalten (#2131): mounted only while open so its fetch
            and dialog context stay out of every test that never opens it. */}
        {canManageCategories && categoryDialog && (
          <CategoryManageModal
            isOpen
            initialView={categoryDialog}
            onClose={() => setCategoryDialog(null)}
            onChanged={async (created) => {
              await refreshCategories(created?.id);
              if (created) setCategoryDialog(null);
            }}
          />
        )}
        {canManagePlanningTracks && planningTrackDialog && (
          <PlanningTrackManageModal
            isOpen
            initialView={planningTrackDialog}
            onClose={() => setPlanningTrackDialog(null)}
            onChanged={async (created) => {
              await refreshPlanningTracks(created?.id);
              if (created) setPlanningTrackDialog(null);
            }}
          />
        )}
      </SlideOverContent>
    </SlideOver>
  );
}
