"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Field } from "./field";
import type {
  EventFormState,
  PersonOption,
  WeekdayRosterState,
} from "./form-model";
import { MultiSelectField } from "./multi-select-field";
import { WeekdayRosterSection } from "./weekday-roster-section";
import type { GroupOption } from "./use-event-form";
import type {
  ConflictWarningItem,
  ShiftCoverageWarningItem,
  TargetGroupType,
} from "~/lib/timetable-types";

/** Zielgruppe (target group) tab options, issue #1838. */
const TARGET_GROUP_OPTIONS: Array<{ value: TargetGroupType; label: string }> = [
  { value: "none", label: "Keine" },
  { value: "jahrgang", label: "Jahrgang" },
  { value: "klasse", label: "Klasse" },
  { value: "gruppe", label: "Gruppe" },
  { value: "angebot", label: "Angebotsauswahl" },
];

export interface StepPersonalKinderProps {
  form: EventFormState;
  update: <K extends keyof EventFormState>(
    key: K,
    value: EventFormState[K],
  ) => void;
  changeTargetGroupType: (nextType: TargetGroupType) => void;
  fieldErrors: Record<string, string>;
  groups: GroupOption[];
  students: PersonOption[];
  staff: PersonOption[];
  loadingRefs: boolean;
  loadingStudents: boolean;
  studentLoadError: string | null;
  loadingStaff: boolean;
  staffLoadError: string | null;
  retryStudentLoad: () => Promise<void>;
  retryStaffLoad: () => Promise<void>;
  expanded: boolean;
  isSeriesFlow: boolean;
  gradeLevelMax: number | undefined;
  targetGradeOptions: Array<{
    value: string;
    label: string;
    disabled: boolean;
  }>;
  preservesGradeAboveTenantCap: boolean;
  studentBulkOptions: Array<{
    key: string;
    label: string;
    memberIds: string[];
  }>;
  targetClassOptions: string[];
  targetClassDescriptionIDs: string;
  targetCohort: { label: string | null; memberIds: string[] };
  missingTargetCohortCount: number;
  targetCohortButtonLabel: string;
  addTargetCohort: () => void;
  conflictWarnings: ConflictWarningItem[];
  coverageWarnings: ShiftCoverageWarningItem[];
  coverageWarningCount: number;
  coverageCheckError: string | null;
  requiredStaffTouched: React.RefObject<boolean>;
  staffRosterTouched: React.RefObject<boolean>;
  /** Per-weekday roster controls (#2129); only rendered in the series flow. */
  activeRosterWeekday: number;
  setActiveRosterWeekday: (weekday: number) => void;
  setPerWeekdayRoster: (enabled: boolean) => void;
  setWeekdayRoster: (weekday: number, roster: WeekdayRosterState) => void;
  applyActiveWeekdayRosterToAll: () => void;
}

/**
 * Wizard step 3 "Personal und Kinder": Zielgruppe, the staff and student
 * rosters, the Personalbedarf override, plus the advisory conflict and
 * Dienstplan-coverage panel. JSX moved verbatim from the previous single-page
 * form; Personal still renders before Kinder (Streichliste 8).
 */
export function StepPersonalKinder({
  form,
  update,
  changeTargetGroupType,
  fieldErrors,
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
  expanded,
  isSeriesFlow,
  gradeLevelMax,
  targetGradeOptions,
  preservesGradeAboveTenantCap,
  studentBulkOptions,
  targetClassOptions,
  targetClassDescriptionIDs,
  targetCohort,
  missingTargetCohortCount,
  targetCohortButtonLabel,
  addTargetCohort,
  conflictWarnings,
  coverageWarnings,
  coverageWarningCount,
  coverageCheckError,
  requiredStaffTouched,
  staffRosterTouched,
  activeRosterWeekday,
  setActiveRosterWeekday,
  setPerWeekdayRoster,
  setWeekdayRoster,
  applyActiveWeekdayRosterToAll,
}: Readonly<StepPersonalKinderProps>) {
  // Per-weekday rosters only make sense for a recurring series, and only once
  // the people lists have actually loaded — otherwise a save would write the
  // empty placeholder lists onto every weekday.
  const rosterWeekdays = [...form.weekdays].sort((a, b) => a - b);
  const showWeekdayRoster =
    isSeriesFlow &&
    !loadingStaff &&
    !staffLoadError &&
    !loadingStudents &&
    !studentLoadError;
  const usePerWeekdayRoster =
    showWeekdayRoster && form.perWeekdayRoster && rosterWeekdays.length >= 2;

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
            <CustomSelect
              id="event_primary_staff"
              ariaLabel="Zuständige Person"
              value={form.primaryStaffId}
              options={[
                { value: "", label: "Keine Auswahl" },
                ...staff
                  .filter((person) => form.staffIds.includes(person.id))
                  .map((person) => ({ value: person.id, label: person.name })),
              ]}
              onChange={(next) => {
                staffRosterTouched.current = true;
                update("primaryStaffId", next);
              }}
            />
          </Field>
        )}
      </>
    );
  }

  return (
    <>
      {expanded && isSeriesFlow && (
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
                <CustomSelect
                  id="event_target_grade_level"
                  ariaLabel="Jahrgang"
                  ariaDescribedBy={
                    fieldErrors.targetGradeLevel
                      ? "event_target_grade_level_error"
                      : undefined
                  }
                  value={form.targetGradeLevel}
                  options={[
                    { value: "", label: "Jahrgang wählen …" },
                    ...targetGradeOptions,
                  ]}
                  onChange={(next) => update("targetGradeLevel", next)}
                  required
                  invalid={Boolean(fieldErrors.targetGradeLevel)}
                  placeholder="Jahrgang wählen …"
                />
              </Field>
              {preservesGradeAboveTenantCap ? (
                <p className="mt-1 text-xs text-gray-500" role="status">
                  Jahrgang {form.targetGradeLevel} liegt über der aktuell
                  konfigurierten Höchststufe {gradeLevelMax}. Die bestehende
                  Zielgruppe bleibt beim Speichern erhalten.
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
                <CustomSelect
                  id="event_target_school_class"
                  ariaLabel="Klasse"
                  ariaDescribedBy={targetClassDescriptionIDs || undefined}
                  value={form.targetSchoolClass}
                  options={[
                    { value: "", label: "Klasse wählen …" },
                    ...targetClassOptions.map((schoolClass) => ({
                      value: schoolClass,
                      label: schoolClass,
                    })),
                  ]}
                  onChange={(next) => update("targetSchoolClass", next)}
                  disabled={loadingStudents || studentLoadError !== null}
                  required
                  invalid={Boolean(fieldErrors.targetSchoolClass)}
                  placeholder="Klasse wählen …"
                />
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
                <CustomSelect
                  id="event_target_gruppe"
                  ariaLabel="Gruppe"
                  ariaDescribedBy={
                    fieldErrors.educationGroupId
                      ? "event_target_gruppe_error"
                      : undefined
                  }
                  value={form.educationGroupId}
                  options={[
                    { value: "", label: "Gruppe wählen …" },
                    ...groups.map((group) => ({
                      value: group.id,
                      label: group.name,
                    })),
                  ]}
                  onChange={(next) => update("educationGroupId", next)}
                  disabled={loadingRefs}
                  required
                  invalid={Boolean(fieldErrors.educationGroupId)}
                  placeholder="Gruppe wählen …"
                />
              </Field>
            </div>
          )}

          {form.targetGroupType === "angebot" && (
            <p className="mt-1 text-xs text-gray-500">
              Kinder kommen automatisch über ein Betreuungsangebot hinzu. Die
              Verknüpfung wird unter „Angebote“ beim jeweiligen Angebot gepflegt
              (Feld „Regeltermin“).
            </p>
          )}

          {targetCohort.label && !loadingStudents && !studentLoadError && (
            <div className="mt-2 flex flex-wrap items-center justify-between gap-2 rounded-xl border border-gray-200 bg-white/70 p-3">
              <p className="min-w-0 flex-1 text-xs leading-5 text-gray-600">
                Die Zielgruppe beschreibt den Regeltermin. Übernimm die
                passenden Kinder mit einem Klick; die Auswahl bleibt danach
                anpassbar.
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
      )}

      {showWeekdayRoster && (
        <WeekdayRosterSection
          form={form}
          weekdays={rosterWeekdays}
          activeWeekday={activeRosterWeekday}
          setActiveWeekday={setActiveRosterWeekday}
          setPerWeekdayRoster={setPerWeekdayRoster}
          setWeekdayRoster={setWeekdayRoster}
          applyActiveWeekdayToAll={applyActiveWeekdayRosterToAll}
          staff={staff}
          students={students}
          studentBulkOptions={studentBulkOptions}
        />
      )}

      {!usePerWeekdayRoster && staffRosterField}

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

      {!usePerWeekdayRoster && studentRosterField}

      {(conflictWarnings.length > 0 ||
        coverageWarnings.length > 0 ||
        coverageCheckError) && (
        // Advisory pre-save hints (QA M7): never disables Speichern.
        <div className="flex flex-col gap-2 border-t border-gray-200 pt-3">
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
              message={`${coverageWarningCount} Dienstplan-${coverageWarningCount === 1 ? "Lücke" : "Lücken"} gefunden. Speichern ist weiterhin möglich.`}
              announce="off"
            />
          )}
          {coverageWarnings.slice(0, 3).map((warning) => (
            <p
              key={`shift-coverage-example-${warning.staffId}-${warning.date}-${warning.uncoveredStartTime}-${warning.uncoveredEndTime}`}
              className="rounded-lg border border-[#EAB308]/20 bg-[#EAB308]/5 px-3 py-2 text-sm text-gray-700"
            >
              {warning.message}
            </p>
          ))}
          {coverageWarningCount > 3 && (
            <details className="rounded-lg border border-[#EAB308]/20 bg-[#EAB308]/5 px-3 py-2 text-sm text-gray-700">
              <summary className="cursor-pointer font-medium focus-visible:ring-2 focus-visible:ring-[#EAB308] focus-visible:outline-none">
                {coverageWarningCount - 3} weitere Lücken anzeigen
              </summary>
              <div className="mt-2 max-h-48 space-y-2 overflow-y-auto overscroll-contain pr-1">
                {coverageWarnings.slice(3).map((warning) => (
                  <p
                    key={`shift-coverage-detail-${warning.staffId}-${warning.date}-${warning.uncoveredStartTime}-${warning.uncoveredEndTime}`}
                    className="border-t border-[#EAB308]/20 pt-2 first:border-t-0 first:pt-0"
                  >
                    {warning.message}
                  </p>
                ))}
                {coverageWarningCount > coverageWarnings.length && (
                  <p className="border-t border-[#EAB308]/20 pt-2 font-medium">
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
    </>
  );
}
