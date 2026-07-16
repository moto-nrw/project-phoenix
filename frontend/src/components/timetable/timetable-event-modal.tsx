"use client";

import { ChevronDown, Repeat, Trash2 } from "lucide-react";

import { useModal } from "~/components/dashboard/modal-context";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { Input } from "~/components/ui/input";
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
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import {
  getActivityColor,
  getGermanWeekdayShort,
} from "~/lib/timetable-helpers";
import { Field } from "./event-form/field";
import { isoWeekday } from "./event-form/form-model";
import type { RepeatMode } from "./event-form/form-model";
import { MultiSelectField } from "./event-form/multi-select-field";
import { useEventForm, WEEKDAYS } from "./event-form/use-event-form";
import type { TimetableEventModalResult } from "./event-form/use-event-form";
import {
  timetableRequiredMark,
  timetableSelectClass,
  timetableTextAreaClass,
} from "./timetable-style";
import type {
  ActivityType,
  EditedChange,
  EnrichedInstance,
  TargetGroupType,
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
  deleted: "Gelöschter Termin",
};

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
   * "quick" starts collapsed with only Titel, Datum/Zeiten, Raum and a
   * plain-language repeat select (US-1 quick create). "Benutzerdefiniert"
   * in that select expands to the full form. Default "full".
   */
  variant?: "full" | "quick";
  defaultStartTime?: string;
  defaultEndTime?: string;
  canCheckShiftCoverage: boolean;
}

const FORM_SELECT_CLASS = timetableSelectClass;

const TYPE_OPTIONS: Array<{
  value: ActivityType;
  label: string;
  hint: string;
}> = [
  { value: "care", label: "Betreuung", hint: "Mensa, Lernzeit, Freispiel" },
  { value: "activity", label: "AG", hint: "Yoga, Bouldern, …" },
  { value: "external", label: "Extern", hint: "DAZ, Musikschule" },
];

const REPEAT_OPTIONS: Array<{ value: RepeatMode; label: string }> = [
  { value: "none", label: "Nie" },
  { value: "weekly", label: "Jede Woche" },
  { value: "biweekly", label: "Alle 2 Wochen" },
];

/** Zielgruppe (target group) tab options, issue #1838. */
const TARGET_GROUP_OPTIONS: Array<{ value: TargetGroupType; label: string }> = [
  { value: "none", label: "Keine" },
  { value: "jahrgang", label: "Jahrgang" },
  { value: "klasse", label: "Klasse" },
  { value: "gruppe", label: "Gruppe" },
  { value: "angebot", label: "Angebotsauswahl" },
];

function weekdayLabel(iso: number): string {
  const ref = new Date(2024, 0, 1);
  ref.setDate(ref.getDate() + (iso - 1));
  return getGermanWeekdayShort(ref);
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
}: TimetableEventModalProps) {
  const { isModalOpen } = useModal();
  const {
    form,
    update,
    updateRepeat,
    toggleWeekday,
    changeTargetGroupType,
    fieldErrors,
    validationError,
    rooms,
    categories,
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
    setMoreOpen,
    moreOpen,
    choiceDialogOpen,
    setPendingSeriesEdit,
    handleScopeSelect,
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
  });

  let studentRosterField: React.ReactNode;
  if (loadingStudents) {
    studentRosterField = (
      <Alert
        type="info"
        message="Kinderliste wird geladen … Die bestehende Kinderzuordnung bleibt beim Speichern unverändert."
      />
    );
  } else if (studentLoadError) {
    studentRosterField = (
      <div className="flex flex-col gap-2">
        <Alert type="warning" message={studentLoadError} />
        <Button
          type="button"
          variant="outline"
          size="compact"
          className="self-start"
          onClick={() => void retryStudentLoad()}
        >
          Kinder erneut laden
        </Button>
      </div>
    );
  } else {
    studentRosterField = (
      <MultiSelectField
        label="Kinder"
        options={students}
        value={form.studentIds}
        onChange={(ids) => update("studentIds", ids)}
        metadata="student"
        bulkOptions={studentBulkOptions}
      />
    );
  }

  let staffRosterField: React.ReactNode;
  if (loadingStaff) {
    staffRosterField = (
      <Alert
        type="info"
        message="Personalliste wird geladen … Die bestehende Personalzuordnung bleibt beim Speichern unverändert."
      />
    );
  } else if (staffLoadError) {
    staffRosterField = (
      <div className="flex flex-col gap-2">
        <Alert type="warning" message={staffLoadError} />
        <Button
          type="button"
          variant="outline"
          size="compact"
          className="self-start"
          onClick={() => void retryStaffLoad()}
        >
          Personal erneut laden
        </Button>
      </div>
    );
  } else {
    staffRosterField = (
      <>
        <MultiSelectField
          label="Personal"
          options={staff}
          value={form.staffIds}
          onChange={(ids) => {
            staffRosterTouched.current = true;
            update("staffIds", ids);
            if (form.primaryStaffId && !ids.includes(form.primaryStaffId)) {
              update("primaryStaffId", "");
            }
          }}
          metadata="staff"
        />

        {isSeriesFlow && form.staffIds.length > 0 && (
          <Field label="Zuständige Person" htmlFor="event_primary_staff">
            <select
              id="event_primary_staff"
              value={form.primaryStaffId}
              onChange={(event) => {
                staffRosterTouched.current = true;
                update("primaryStaffId", event.target.value);
              }}
              className={FORM_SELECT_CLASS}
            >
              <option value="">Keine Auswahl</option>
              {staff
                .filter((person) => form.staffIds.includes(person.id))
                .map((person) => (
                  <option key={person.id} value={person.id}>
                    {person.name}
                  </option>
                ))}
            </select>
          </Field>
        )}
      </>
    );
  }

  // Personal renders before Kinder in every mode (Streichliste 8); the
  // quick variant tucks all of this behind the "Weitere Optionen" row.
  const peopleFields = (
    <>
      {staffRosterField}

      <Field label="Benötigtes Personal" htmlFor="event_required_staff">
        <Input
          id="event_required_staff"
          type="number"
          min={0}
          step={1}
          inputMode="numeric"
          value={form.requiredStaff}
          onChange={(event) => {
            requiredStaffTouched.current = true;
            update("requiredStaff", event.target.value);
          }}
          placeholder="automatisch aus Betreuungsschlüssel"
          controlSize="compact"
        />
        <p className="mt-1 text-xs text-gray-500">
          Leer = automatisch: Es gilt der Wert der Terminreihe, sonst die
          Berechnung aus dem Betreuungsschlüssel (Kinderzahl). Eine Zahl legt
          den Bedarf fest und überschreibt beides.
        </p>
      </Field>

      {studentRosterField}

      {isSeriesFlow ? (
        <Field label="Wochennotiz" htmlFor="event_series_notes">
          <textarea
            id="event_series_notes"
            value={form.seriesNotes}
            onChange={(event) => update("seriesNotes", event.target.value)}
            rows={3}
            className={timetableTextAreaClass}
            placeholder="z. B. Raum erst ab 14 Uhr offen"
          />
          <p className="mt-1 text-xs text-gray-500">
            Gilt dauerhaft für die ganze Terminreihe und erscheint an jedem
            Termin. Bleibt bei Re-Plan und Serienänderungen erhalten.
          </p>
        </Field>
      ) : (
        <>
          {form.seriesNotes.trim() !== "" && (
            <div className="rounded-lg border border-[#5080D8]/30 bg-[#5080D8]/10 p-3">
              <div className="flex items-center gap-1.5 text-xs font-medium text-[#5080D8]">
                <Repeat className="h-3.5 w-3.5" aria-hidden="true" />
                Wochennotiz der Terminreihe
              </div>
              <p className="mt-1 text-sm whitespace-pre-wrap text-gray-700">
                {form.seriesNotes}
              </p>
              <p className="mt-1 text-xs text-gray-500">
                Wird über den Regeltermin gepflegt und gilt für alle Termine.
              </p>
            </div>
          )}
          <Field label="Tagesnotiz" htmlFor="event_notes">
            <textarea
              id="event_notes"
              value={form.notes}
              onChange={(event) => update("notes", event.target.value)}
              rows={3}
              className={timetableTextAreaClass}
              placeholder="Nur für diesen einen Termin"
            />
          </Field>
        </>
      )}
    </>
  );

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

        <form
          id="timetable-event-form"
          noValidate
          onSubmit={(event) => void handleSubmit(event)}
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

            <Field label="Titel" htmlFor="event_title" required>
              <Input
                id="event_title"
                value={form.title}
                onChange={(event) => update("title", event.target.value)}
                placeholder="z. B. Mensa, Lernzeit 1a, Yoga AG"
                maxLength={255}
                controlSize="compact"
                error={fieldErrors.title}
                autoFocus
                required
              />
            </Field>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Field label="Datum" htmlFor="event_date" required>
                <Input
                  id="event_date"
                  type="date"
                  value={form.date}
                  controlSize="compact"
                  error={fieldErrors.date}
                  onChange={(event) => {
                    const nextDate = event.target.value;
                    const nextWeekday = isoWeekday(nextDate);
                    update("date", nextDate);
                    // One-off events follow the date; the quick preset
                    // "Wöchentlich am <Tag>" retargets to the new weekday.
                    const followsDate =
                      !isSeriesFlow ||
                      (!expanded && quickPreset === "woechentlich-am");
                    if (followsDate && nextWeekday >= 1 && nextWeekday <= 5) {
                      update("weekdays", [nextWeekday]);
                    }
                  }}
                  required
                />
              </Field>
              <Field label="Start" htmlFor="event_start" required>
                <Input
                  id="event_start"
                  type="time"
                  value={form.startTime}
                  controlSize="compact"
                  error={fieldErrors.startTime}
                  onChange={(event) => update("startTime", event.target.value)}
                  required
                />
              </Field>
              <Field label="Ende" htmlFor="event_end" required>
                <Input
                  id="event_end"
                  type="time"
                  value={form.endTime}
                  controlSize="compact"
                  error={fieldErrors.endTime}
                  onChange={(event) => update("endTime", event.target.value)}
                  required
                />
              </Field>
            </div>

            <Field
              label="Raum"
              htmlFor="event_room"
              required
              error={fieldErrors.roomId}
            >
              <select
                id="event_room"
                value={form.roomId}
                onChange={(event) => update("roomId", event.target.value)}
                disabled={loadingRefs}
                required
                aria-invalid={fieldErrors.roomId ? true : undefined}
                aria-describedby={
                  fieldErrors.roomId ? "event_room_error" : undefined
                }
                className={FORM_SELECT_CLASS}
              >
                <option value="">
                  {loadingRefs ? "Lade Räume …" : "Raum auswählen …"}
                </option>
                {rooms.map((room) => (
                  <option key={room.id} value={room.id}>
                    {room.building
                      ? `${room.building} - ${room.name}`
                      : room.name}
                  </option>
                ))}
              </select>
            </Field>

            {expanded ? (
              <div className="flex flex-col gap-1">
                <span className="text-xs font-semibold text-gray-700">
                  Wiederholt sich
                </span>
                <Tabs
                  value={form.repeat}
                  onValueChange={(value) => {
                    const nextRepeat = value as RepeatMode;
                    updateRepeat(nextRepeat);
                    if (nextRepeat !== "none" && form.weekdays.length === 0) {
                      const weekday = isoWeekday(form.date);
                      update(
                        "weekdays",
                        weekday >= 1 && weekday <= 5 ? [weekday] : [1],
                      );
                    }
                  }}
                >
                  <TabsList aria-label="Wiederholung" className="w-fit">
                    {REPEAT_OPTIONS.map((option) => (
                      <TabsTrigger
                        key={option.value}
                        value={option.value}
                        disabled={
                          (isEditingSeries && option.value === "none") ||
                          (!isEditingSeries &&
                            option.value === "biweekly" &&
                            biweeklyUnavailable)
                        }
                      >
                        {option.label}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                </Tabs>
              </div>
            ) : (
              <Field label="Wiederholt sich" htmlFor="event_quick_repeat">
                <select
                  id="event_quick_repeat"
                  value={quickPreset}
                  onChange={(event) =>
                    handleQuickPresetChange(event.target.value)
                  }
                  className={FORM_SELECT_CLASS}
                >
                  <option value="einmalig">Einmalig</option>
                  {/* On Sa/So a weekly preset would silently save a Monday
                      series (weekdays are Mo–Fr) — omit it instead. */}
                  {dateWeekday >= 1 && dateWeekday <= 5 && (
                    <option value="woechentlich-am">
                      {`Wöchentlich am ${dateWeekdayName}`}
                    </option>
                  )}
                  <option value="jeden-wochentag">
                    Jeden Wochentag (Mo–Fr)
                  </option>
                  <option value="benutzerdefiniert">Benutzerdefiniert …</option>
                </select>
              </Field>
            )}

            {expanded && form.repeat === "biweekly" && (
              <div className="flex flex-col gap-1">
                <span className="text-xs font-semibold text-gray-700">
                  Wochenrhythmus
                </span>
                <Tabs
                  value={form.weekPattern === 1 ? "A" : "B"}
                  onValueChange={(value) => {
                    manualWeekPattern.current = value === "A" ? 1 : 2;
                    update("weekPattern", value === "A" ? 1 : 2);
                  }}
                >
                  <TabsList aria-label="A/B-Woche" className="w-fit">
                    <TabsTrigger value="A">Woche A</TabsTrigger>
                    <TabsTrigger value="B">Woche B</TabsTrigger>
                  </TabsList>
                </Tabs>
                <p className="text-xs text-gray-500">{abWeekHint}</p>
                {fieldErrors.weekPattern && (
                  <p role="alert" className="mt-1 text-xs text-red-600">
                    {fieldErrors.weekPattern}
                  </p>
                )}
              </div>
            )}

            {expanded && isSeriesFlow && (
              <>
                <div className="flex flex-col gap-1">
                  <span className="text-xs font-semibold text-gray-700">
                    Typ <span className={timetableRequiredMark}>*</span>
                  </span>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                    {TYPE_OPTIONS.map((option) => {
                      const isActive = form.type === option.value;
                      const color = getActivityColor(option.value);
                      return (
                        <button
                          key={option.value}
                          type="button"
                          onClick={() => update("type", option.value)}
                          className={`flex flex-col items-start gap-0.5 rounded-lg border border-l-[3px] px-3 py-2 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                            isActive
                              ? "border-gray-300 bg-white"
                              : "border-gray-200 bg-white hover:bg-gray-50"
                          }`}
                          style={{ borderLeftColor: color }}
                        >
                          <span
                            className="text-sm font-semibold"
                            style={{ color: isActive ? color : "#374151" }}
                          >
                            {option.label}
                          </span>
                          <span className="text-[10px] text-gray-500">
                            {option.hint}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </div>

                <div className="flex flex-col gap-1">
                  <span className="text-xs font-semibold text-gray-700">
                    Wochentage <span className={timetableRequiredMark}>*</span>
                  </span>
                  <div className="flex flex-wrap gap-1.5">
                    {WEEKDAYS.map((iso) => {
                      const isActive = form.weekdays.includes(iso);
                      return (
                        <Button
                          key={iso}
                          type="button"
                          variant={isActive ? "primary" : "outline"}
                          size="compact"
                          className="min-w-[44px]"
                          onClick={() => toggleWeekday(iso)}
                          aria-pressed={isActive}
                        >
                          {weekdayLabel(iso)}
                        </Button>
                      );
                    })}
                  </div>
                  {fieldErrors.weekdays && (
                    <p role="alert" className="mt-1 text-xs text-red-600">
                      {fieldErrors.weekdays}
                    </p>
                  )}
                </div>

                <Field
                  label="Kategorie"
                  htmlFor="event_category"
                  required
                  error={fieldErrors.categoryId}
                >
                  <select
                    id="event_category"
                    value={form.categoryId}
                    onChange={(event) =>
                      update("categoryId", event.target.value)
                    }
                    required
                    disabled={loadingRefs}
                    aria-invalid={fieldErrors.categoryId ? true : undefined}
                    aria-describedby={
                      fieldErrors.categoryId
                        ? "event_category_error"
                        : undefined
                    }
                    className={FORM_SELECT_CLASS}
                  >
                    <option value="">
                      {loadingRefs ? "Lade Kategorien …" : "Kategorie wählen …"}
                    </option>
                    {categories.map((category) => (
                      <option key={category.id} value={category.id}>
                        {category.name}
                      </option>
                    ))}
                  </select>
                </Field>

                <div className="flex flex-col gap-1">
                  <span className="text-xs font-semibold text-gray-700">
                    Zielgruppe
                  </span>
                  <Tabs
                    value={form.targetGroupType}
                    onValueChange={(value) =>
                      changeTargetGroupType(value as TargetGroupType)
                    }
                  >
                    <TabsList aria-label="Zielgruppe" className="w-fit">
                      {TARGET_GROUP_OPTIONS.map((option) => (
                        <TabsTrigger key={option.value} value={option.value}>
                          {option.label}
                        </TabsTrigger>
                      ))}
                    </TabsList>
                  </Tabs>

                  {form.targetGroupType === "jahrgang" && (
                    <div className="mt-1">
                      <Field
                        label="Jahrgang"
                        htmlFor="event_target_grade_level"
                        required
                        error={fieldErrors.targetGradeLevel}
                      >
                        <select
                          id="event_target_grade_level"
                          value={form.targetGradeLevel}
                          onChange={(event) =>
                            update("targetGradeLevel", event.target.value)
                          }
                          required
                          aria-invalid={
                            fieldErrors.targetGradeLevel ? true : undefined
                          }
                          aria-describedby={
                            fieldErrors.targetGradeLevel
                              ? "event_target_grade_level_error"
                              : undefined
                          }
                          className={FORM_SELECT_CLASS}
                        >
                          <option value="">Jahrgang wählen …</option>
                          {targetGradeOptions.map((option) => (
                            <option
                              key={option.value}
                              value={option.value}
                              disabled={option.disabled}
                            >
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </Field>
                      {preservesGradeAboveTenantCap ? (
                        <p className="mt-1 text-xs text-gray-500" role="status">
                          Jahrgang {form.targetGradeLevel} liegt über der
                          aktuell konfigurierten Höchststufe {gradeLevelMax}.
                          Die bestehende Zielgruppe bleibt beim Speichern
                          erhalten.
                        </p>
                      ) : null}
                    </div>
                  )}

                  {form.targetGroupType === "klasse" && (
                    <div className="mt-1">
                      <Field
                        label="Klasse"
                        htmlFor="event_target_school_class"
                        required
                        error={fieldErrors.targetSchoolClass}
                      >
                        <select
                          id="event_target_school_class"
                          value={form.targetSchoolClass}
                          onChange={(event) =>
                            update("targetSchoolClass", event.target.value)
                          }
                          disabled={
                            loadingStudents || studentLoadError !== null
                          }
                          required
                          aria-invalid={
                            fieldErrors.targetSchoolClass ? true : undefined
                          }
                          aria-describedby={
                            targetClassDescriptionIDs || undefined
                          }
                          className={FORM_SELECT_CLASS}
                        >
                          <option value="">Klasse wählen …</option>
                          {targetClassOptions.map((schoolClass) => (
                            <option key={schoolClass} value={schoolClass}>
                              {schoolClass}
                            </option>
                          ))}
                        </select>
                      </Field>
                      {loadingStudents || studentLoadError ? (
                        <p
                          id="event_target_school_class_availability"
                          className="mt-1 text-xs text-gray-500"
                          role="status"
                        >
                          {studentLoadError
                            ? "Die Klassenliste ist nicht verfügbar. Eine bestehende Klassen-Zielgruppe bleibt unverändert."
                            : "Klassenliste wird geladen …"}
                        </p>
                      ) : null}
                    </div>
                  )}

                  {form.targetGroupType === "gruppe" && (
                    <div className="mt-1">
                      <Field
                        label="Gruppe"
                        htmlFor="event_target_gruppe"
                        required
                        error={fieldErrors.educationGroupId}
                      >
                        <select
                          id="event_target_gruppe"
                          value={form.educationGroupId}
                          onChange={(event) =>
                            update("educationGroupId", event.target.value)
                          }
                          disabled={loadingRefs}
                          required
                          aria-invalid={
                            fieldErrors.educationGroupId ? true : undefined
                          }
                          aria-describedby={
                            fieldErrors.educationGroupId
                              ? "event_target_gruppe_error"
                              : undefined
                          }
                          className={FORM_SELECT_CLASS}
                        >
                          <option value="">Gruppe wählen …</option>
                          {groups.map((group) => (
                            <option key={group.id} value={group.id}>
                              {group.name}
                            </option>
                          ))}
                        </select>
                      </Field>
                    </div>
                  )}

                  {form.targetGroupType === "angebot" && (
                    <p className="mt-1 text-xs text-gray-500">
                      Kinder kommen automatisch über ein Betreuungsangebot
                      hinzu. Die Verknüpfung wird unter „Angebote“ beim
                      jeweiligen Angebot gepflegt (Feld „Regeltermin“).
                    </p>
                  )}

                  {targetCohort.label &&
                    !loadingStudents &&
                    !studentLoadError && (
                      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 rounded-xl border border-gray-200 bg-white/70 p-3">
                        <p className="min-w-0 flex-1 text-xs leading-5 text-gray-600">
                          Die Zielgruppe beschreibt den Regeltermin. Übernimm
                          die passenden Kinder mit einem Klick; die Auswahl
                          bleibt danach anpassbar.
                        </p>
                        <Button
                          type="button"
                          variant="outline"
                          size="compact"
                          onClick={addTargetCohort}
                          disabled={
                            targetCohort.memberIds.length === 0 ||
                            missingTargetCohortCount === 0
                          }
                        >
                          {targetCohortButtonLabel}
                        </Button>
                      </div>
                    )}
                </div>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  {showPeriodField ? (
                    <Field
                      label="Planungszeitraum"
                      htmlFor="event_period"
                      required
                      error={fieldErrors.calendarPeriodId}
                    >
                      <select
                        id="event_period"
                        value={form.calendarPeriodId}
                        onChange={(event) =>
                          update("calendarPeriodId", event.target.value)
                        }
                        required
                        aria-invalid={
                          fieldErrors.calendarPeriodId ? true : undefined
                        }
                        aria-describedby={
                          fieldErrors.calendarPeriodId
                            ? "event_period_error"
                            : undefined
                        }
                        className={FORM_SELECT_CLASS}
                      >
                        <option value="">Zeitraum auswählen …</option>
                        {calendarPeriods.map((period) => (
                          <option key={period.id} value={period.id}>
                            {period.name}
                          </option>
                        ))}
                      </select>
                    </Field>
                  ) : (
                    <div className="flex flex-col justify-end gap-1">
                      <span className="text-xs font-semibold text-gray-700">
                        Planungszeitraum
                      </span>
                      <div className="flex h-10 items-center rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm text-gray-600">
                        <span className="truncate">
                          Gilt in{" "}
                          <span className="font-semibold text-gray-800">
                            {calendarPeriods.find(
                              (p) => p.id === form.calendarPeriodId,
                            )?.name ?? "dem aktuellen Planungszeitraum"}
                          </span>
                        </span>
                      </div>
                      {fieldErrors.calendarPeriodId && (
                        <p role="alert" className="mt-1 text-xs text-red-600">
                          {fieldErrors.calendarPeriodId}
                        </p>
                      )}
                    </div>
                  )}
                </div>
              </>
            )}

            {expanded ? (
              peopleFields
            ) : (
              <div className="flex flex-col gap-5">
                <button
                  type="button"
                  onClick={() => setMoreOpen((open) => !open)}
                  aria-expanded={moreOpen}
                  className="flex w-fit items-center gap-1.5 rounded-lg px-2 py-1.5 text-xs font-semibold text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                >
                  <ChevronDown
                    className={`h-4 w-4 transition-transform ${moreOpen ? "rotate-180" : ""}`}
                    aria-hidden
                  />
                  Weitere Optionen
                </button>
                {moreOpen && peopleFields}
              </div>
            )}

            {isSeriesFlow && calendarPeriods.length === 0 && (
              <Alert
                type="error"
                message="Für diese Woche gibt es keinen aktiven Planungszeitraum. Lege zuerst oben im Plan einen Zeitraum an."
              />
            )}

            {validationError && (
              <Alert type="error" message={validationError} />
            )}
          </div>
        </form>

        {(conflictWarnings.length > 0 ||
          coverageWarnings.length > 0 ||
          coverageCheckError) && (
          // Advisory pre-save hints (QA M7): visible in quick and expanded
          // mode, pinned above the footer. Never disables Speichern.
          <div className="flex max-h-[40vh] flex-col gap-2 overflow-y-auto overscroll-contain border-t border-gray-200 px-5 py-3">
            <p className="sr-only" aria-live="polite">
              {`${conflictWarnings.length} Terminüberschneidungen und ${coverageWarningCount} Dienstplan-Lücken gefunden. Speichern ist weiterhin möglich.`}
            </p>
            {conflictWarnings.map((warning, index) => (
              <Alert
                key={`${warning.kind}-${warning.resourceId}-${index}`}
                type="warning"
                message={`Hinweis: ${warning.message}`}
                announce="off"
              />
            ))}
            {coverageWarningCount > 0 && (
              <Alert
                type="warning"
                message={`${coverageWarningCount} Dienstplan-${coverageWarningCount === 1 ? "Lücke" : "Lücken"} gefunden. Speichern ist weiterhin möglich.`}
                announce="off"
              />
            )}
            {coverageWarnings.slice(0, 3).map((warning, index) => (
              <p
                key={`shift-coverage-example-${warning.staffId}-${warning.date}-${warning.uncoveredStartTime}-${index}`}
                className="rounded-lg border border-yellow-100 bg-yellow-50/50 px-3 py-2 text-sm text-yellow-800"
              >
                {warning.message}
              </p>
            ))}
            {coverageWarningCount > 3 && (
              <details className="rounded-lg border border-yellow-100 bg-yellow-50/40 px-3 py-2 text-sm text-yellow-800">
                <summary className="cursor-pointer font-medium focus-visible:ring-2 focus-visible:ring-yellow-600 focus-visible:outline-none">
                  {coverageWarningCount - 3} weitere Lücken anzeigen
                </summary>
                <div className="mt-2 max-h-48 space-y-2 overflow-y-auto overscroll-contain pr-1">
                  {coverageWarnings.slice(3).map((warning, index) => (
                    <p
                      key={`shift-coverage-detail-${warning.staffId}-${warning.date}-${warning.uncoveredStartTime}-${index}`}
                      className="border-t border-yellow-100 pt-2 first:border-t-0 first:pt-0"
                    >
                      {warning.message}
                    </p>
                  ))}
                  {coverageWarningCount > coverageWarnings.length && (
                    <p className="border-t border-yellow-100 pt-2 font-medium">
                      Es werden höchstens 100 Beispiele angezeigt.
                    </p>
                  )}
                </div>
              </details>
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
                (isEditingInstance && initialInstance?.status !== "planned")
              }
            >
              Speichern
            </Button>
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
            confirmButtonClass="bg-red-600 hover:bg-red-700"
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
                <Input
                  id="series_delete_effective_date"
                  type="date"
                  value={deleteEffectiveDate}
                  min={berlinTodayISO()}
                  controlSize="compact"
                  aria-invalid={deleteError ? true : undefined}
                  disabled={deletingSeries}
                  onChange={(event) => {
                    setDeleteEffectiveDate(event.target.value);
                    setDeleteError(null);
                  }}
                  required
                />
              </Field>
            </div>
          </ConfirmationModal>
        )}

        {initialInstance && (
          <ChoiceModal
            isOpen={choiceDialogOpen}
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
      </SlideOverContent>
    </SlideOver>
  );
}
