"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Field } from "./field";
import type {
  EventFormState,
  PersonOption,
  SourceFilterMode,
  WeekdayRosterState,
} from "./form-model";
import { MultiSelectField } from "./multi-select-field";
import { WeekdayRosterSection } from "./weekday-roster-section";
import type { GroupOption } from "./use-event-form";
import { Checkbox } from "~/components/ui/checkbox";
import type {
  ConflictWarningItem,
  OfferingSourceOption,
  ShiftCoverageWarningItem,
  TargetGroupType,
} from "~/lib/timetable-types";
import { normalizeSchoolClass } from "~/lib/timetable-helpers";

/** Zielgruppe (target group) tab options, issue #1838. */
const TARGET_GROUP_OPTIONS: Array<{ value: TargetGroupType; label: string }> = [
  { value: "none", label: "Keine" },
  { value: "jahrgang", label: "Jahrgang" },
  { value: "klasse", label: "Klasse" },
  { value: "gruppe", label: "Gruppe" },
  { value: "angebot", label: "Angebot" },
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
  targetCohort: { label: string | null; memberIds: string[] };
  missingTargetCohortCount: number;
  targetCohortButtonLabel: string;
  addTargetCohort: () => void;
  offeringSources: OfferingSourceOption[] | null;
  offeringSourcesError: string | null;
  sourcePhaseLockId: string | null;
  sourceGradeOptions: number[];
  sourceGradeCounts: Record<number, number>;
  /** Klassenfilter (#2482): Optionen, Zahlen und der laufende Abgleich. */
  sourceClassOptions: string[];
  sourceClassCounts: Record<string, number>;
  sourceFilteredCount: number;
  /** Die genaue Kinderzahl steht noch aus (Anfrage läuft). */
  sourceCountsPending: boolean;
  /** Die Kinder der Quelle konnten nicht geladen werden. */
  sourceCountsError: string | null;
  /** Namentliche Abweichung zur bisher manuell gepflegten Kinderliste. */
  sourceRosterDiff: { added: string[]; removed: string[] } | null;
  /** Advisory when series start is before the offering phase service start. */
  sourcePhaseKidsFromWarning: string | null;
  sourceOverlapWarnings: string[];
  changeSourceOfferings: (offeringIds: string[]) => void;
  toggleSourceGradeLevel: (grade: number) => void;
  toggleSourceSchoolClass: (schoolClass: string) => void;
  changeSourceFilterMode: (mode: SourceFilterMode) => void;
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
  targetCohort,
  missingTargetCohortCount,
  targetCohortButtonLabel,
  addTargetCohort,
  offeringSources,
  offeringSourcesError,
  sourcePhaseLockId,
  sourceGradeOptions,
  sourceGradeCounts,
  sourceClassOptions,
  sourceClassCounts,
  sourceFilteredCount,
  sourceCountsPending,
  sourceCountsError,
  sourceRosterDiff,
  sourcePhaseKidsFromWarning,
  sourceOverlapWarnings,
  changeSourceOfferings,
  toggleSourceGradeLevel,
  toggleSourceSchoolClass,
  changeSourceFilterMode,
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
  const hasOfferingSource =
    isSeriesFlow &&
    form.targetGroupType === "angebot" &&
    form.sourceCareOfferingIds.length > 0;
  // Stored sources missing from the fetched list (deactivated offering,
  // other calendar period, load error) must stay removable: rendered as
  // checked entries, they keep the select enabled and uncheckable, so a
  // template can always be cleared back to a manual roster.
  const knownOfferingIds = new Set(
    (offeringSources ?? []).map((offering) => offering.id),
  );
  const unknownSelectedOfferingOptions = form.sourceCareOfferingIds
    .filter((id) => !knownOfferingIds.has(id))
    .map((id) => ({
      value: id,
      label: `Nicht mehr verfügbares Angebot (ID ${id})`,
    }));
  // Per-weekday rosters only make sense for a recurring series, and only once
  // the people lists have actually loaded — otherwise a save would write the
  // empty placeholder lists onto every weekday. An offering-sourced template
  // (#2137) hides the whole section: its child roster is server-managed, and
  // the backend rejects weekday_assignments next to a source.
  const rosterWeekdays = [...form.weekdays].sort((a, b) => a - b);
  const showWeekdayRoster =
    isSeriesFlow &&
    !hasOfferingSource &&
    !loadingStaff &&
    !staffLoadError &&
    !loadingStudents &&
    !studentLoadError;
  const usePerWeekdayRoster =
    showWeekdayRoster && form.perWeekdayRoster && rosterWeekdays.length >= 2;
  const preserveUnavailableWeekdayRoster =
    isSeriesFlow &&
    !hasOfferingSource &&
    form.perWeekdayRoster &&
    rosterWeekdays.length >= 2 &&
    !showWeekdayRoster;

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
        protectedValues={form.protectedStudentAssignments.flatMap(
          (assignment) => assignment.studentIds,
        )}
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
            {/* Five pills exceed narrow viewports; wrap instead of bleeding
                out of the dialog (h-auto overrides the kit's fixed h-9). */}
            <TabsList
              aria-label="Zielgruppe"
              className="h-auto w-fit max-w-full flex-wrap justify-start"
            >
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
                <MultiCheckboxSelect
                  id="event_target_grade_level"
                  ariaLabel="Jahrgang"
                  ariaInvalid={Boolean(fieldErrors.targetGradeLevel)}
                  ariaDescribedBy={
                    fieldErrors.targetGradeLevel
                      ? "event_target_grade_level_error"
                      : undefined
                  }
                  value={form.targetGradeLevels}
                  options={targetGradeOptions}
                  onChange={(next) => update("targetGradeLevels", next)}
                  emptyLabel="Jahrgänge wählen ..."
                />
              </Field>
              {preservesGradeAboveTenantCap ? (
                <p className="mt-1 text-xs text-gray-500" role="status">
                  Eine bestehende Auswahl liegt über der aktuell konfigurierten
                  Höchststufe {gradeLevelMax} und bleibt erhalten.
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
                <MultiCheckboxSelect
                  id="event_target_school_class"
                  ariaLabel="Klasse"
                  ariaInvalid={Boolean(fieldErrors.targetSchoolClass)}
                  ariaDescribedBy={
                    fieldErrors.targetSchoolClass
                      ? "event_target_school_class_error"
                      : undefined
                  }
                  value={form.targetSchoolClasses}
                  options={targetClassOptions.map((schoolClass) => ({
                    value: schoolClass,
                    label: schoolClass,
                  }))}
                  onChange={(next) => update("targetSchoolClasses", next)}
                  disabled={loadingStudents || studentLoadError !== null}
                  emptyLabel="Klassen wählen ..."
                  searchable
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
                <MultiCheckboxSelect
                  id="event_target_gruppe"
                  ariaLabel="Gruppe"
                  ariaInvalid={Boolean(fieldErrors.educationGroupId)}
                  ariaDescribedBy={
                    fieldErrors.educationGroupId
                      ? "event_target_gruppe_error"
                      : undefined
                  }
                  value={form.educationGroupIds}
                  options={groups.map((group) => ({
                    value: group.id,
                    label: group.name,
                  }))}
                  onChange={(next) => update("educationGroupIds", next)}
                  disabled={loadingRefs}
                  emptyLabel="Gruppen wählen ..."
                  searchable
                />
              </Field>
            </div>
          )}

          {form.targetGroupType === "angebot" && (
            <div className="mt-1 flex flex-col gap-2">
              <Field
                label="Angebote als Quelle"
                htmlFor="event_source_offerings"
              >
                <MultiCheckboxSelect
                  id="event_source_offerings"
                  ariaLabel="Angebote als Quelle"
                  value={form.sourceCareOfferingIds}
                  options={[
                    ...unknownSelectedOfferingOptions,
                    ...(offeringSources ?? []).map((offering) => ({
                      value: offering.id,
                      label: `${offering.name} (${offering.phaseName || "ohne Phase"}, ${offering.totalCount} ${offering.totalCount === 1 ? "Kind" : "Kinder"})`,
                      // The union requires one shared enrollment phase; the
                      // first selection locks the rest of the list to it.
                      disabled:
                        sourcePhaseLockId !== null &&
                        offering.phaseId !== sourcePhaseLockId &&
                        !form.sourceCareOfferingIds.includes(offering.id),
                    })),
                  ]}
                  onChange={changeSourceOfferings}
                  disabled={offeringSources === null}
                  emptyLabel="Kein Angebot als Quelle"
                />
                <p className="mt-1 text-xs text-gray-500">
                  Mehrere Angebote lassen sich kombinieren – die Kinder aller
                  gewählten Angebote werden zusammengeführt, jedes Kind zählt
                  dabei nur einmal. Alle Angebote müssen zur selben Anmeldephase
                  gehören. Kinder erscheinen nur an den Wochentagen, an denen
                  sie im Angebot angemeldet sind – nicht an jedem Tag des
                  Regeltermins.
                </p>
                {sourcePhaseLockId !== null &&
                  (offeringSources ?? []).some(
                    (offering) =>
                      offering.phaseId !== sourcePhaseLockId &&
                      !form.sourceCareOfferingIds.includes(offering.id),
                  ) && (
                    <p className="mt-1 text-xs text-gray-500">
                      Angebote anderer Anmeldephasen sind ausgegraut, solange
                      eine Quelle gewählt ist.
                    </p>
                  )}
              </Field>
              {offeringSourcesError ? (
                <Alert type="warning" message={offeringSourcesError} />
              ) : null}
              {form.sourceCareOfferingIds.length === 0 && (
                <Alert
                  type="info"
                  message="Mit Angeboten als Quelle übernimmt der Regeltermin die angemeldeten Kinder automatisch, auch bei späteren An- und Abmeldungen. Ohne Quelle gilt weiterhin die Verknüpfung, die unter „Angebote“ beim jeweiligen Angebot gepflegt wird (Feld „Regeltermin“)."
                />
              )}
              {/* Gate on the FORM selection, matching hasOfferingSource: a
                  stored source missing from the fetched list still hides the
                  manual picker, so the filter fieldset must stay visible and
                  editable — otherwise neither roster control is reachable. */}
              {form.sourceCareOfferingIds.length > 0 && (
                <>
                  <fieldset className="flex flex-col gap-1">
                    <legend className="text-xs font-semibold text-gray-700">
                      Kinder eingrenzen
                    </legend>
                    {/* Erklärtext gehört direkt unter die Legend, Legend und
                        Text bilden einen Kopf. */}
                    <p className="mt-1 text-xs leading-5 text-gray-500">
                      Sie können die Kinder nach Jahrgang oder nach einzelnen
                      Klassen eingrenzen. Beides zusammen geht nicht.
                    </p>
                    <div className="mt-1">
                      <SegmentedControl<SourceFilterMode>
                        ariaLabel="Kinder eingrenzen"
                        fullWidth
                        value={form.sourceFilterMode}
                        onChange={changeSourceFilterMode}
                        items={[
                          { value: "alle", label: "Alle Kinder" },
                          { value: "jahrgang", label: "Nach Jahrgang" },
                          { value: "klasse", label: "Nach Klasse" },
                        ]}
                      />
                    </div>
                    {form.sourceFilterMode === "jahrgang" && (
                      <div className="mt-2 flex flex-wrap gap-3">
                        {sourceGradeOptions.map((grade) => (
                          <label
                            key={grade}
                            className="flex cursor-pointer items-center gap-1.5 text-sm text-gray-700"
                          >
                            <Checkbox
                              checked={form.sourceGradeLevels.includes(grade)}
                              onChange={() => toggleSourceGradeLevel(grade)}
                            />
                            Jahrgang {grade} ({sourceGradeCounts[grade] ?? 0})
                          </label>
                        ))}
                        {sourceGradeOptions.length === 0 && (
                          <p className="text-xs text-gray-500">
                            Für die gewählten Angebote sind noch keine Jahrgänge
                            ableitbar.
                          </p>
                        )}
                      </div>
                    )}
                    {form.sourceFilterMode === "klasse" && (
                      <div className="mt-2 flex flex-wrap gap-3">
                        {sourceClassOptions.map((schoolClass) => (
                          <label
                            key={schoolClass}
                            className="flex cursor-pointer items-center gap-1.5 text-sm text-gray-700"
                          >
                            <Checkbox
                              checked={form.sourceSchoolClasses.some(
                                (item) =>
                                  normalizeSchoolClass(item) ===
                                  normalizeSchoolClass(schoolClass),
                              )}
                              onChange={() =>
                                toggleSourceSchoolClass(schoolClass)
                              }
                            />
                            Klasse {schoolClass} (
                            {sourceClassCounts[
                              normalizeSchoolClass(schoolClass)
                            ] ?? 0}
                            )
                          </label>
                        ))}
                        {sourceClassOptions.length === 0 && (
                          <p className="text-xs text-gray-500">
                            {sourceCountsPending
                              ? "Klassen werden geladen ..."
                              : sourceCountsError
                                ? "Die Klassen konnten nicht geladen werden."
                                : "Für die gewählten Angebote ist bei keinem Kind eine Klasse hinterlegt."}
                          </p>
                        )}
                      </div>
                    )}
                    {(form.sourceFilterMode === "jahrgang" &&
                      form.sourceGradeLevels.length === 0) ||
                    (form.sourceFilterMode === "klasse" &&
                      form.sourceSchoolClasses.length === 0) ? (
                      <p className="mt-1 text-xs text-gray-500">
                        Ohne Auswahl gelten alle Kinder der gewählten Angebote.
                      </p>
                    ) : null}
                  </fieldset>
                  {sourceCountsError ? (
                    <Alert
                      type="warning"
                      message={sourceCountsError}
                      announce="polite"
                    />
                  ) : sourceCountsPending ? (
                    <p className="text-xs text-gray-600" role="status">
                      Die Kinderzahl wird ermittelt ...
                    </p>
                  ) : sourceFilteredCount === 0 ? (
                    <Alert
                      type="warning"
                      message="Der aktuelle Filter erfasst keine Kinder. Der Regeltermin würde ohne Kinder eingeplant."
                      announce="polite"
                    />
                  ) : (
                    <p className="text-xs text-gray-600" role="status">
                      Der aktuelle Filter erfasst {sourceFilteredCount}{" "}
                      {sourceFilteredCount === 1 ? "Kind" : "Kinder"}. Neue,
                      geänderte oder beendete Anmeldungen wirken sich
                      automatisch auf zukünftige Planungen aus.
                    </p>
                  )}
                  {sourceRosterDiff ? (
                    <div className="rounded-md border border-gray-200 bg-gray-50 p-3">
                      <p className="text-xs font-semibold text-gray-700">
                        Das ändert sich gegenüber Ihrer bisherigen Kinderliste
                      </p>
                      {sourceRosterDiff.added.length > 0 && (
                        <p className="mt-1 text-xs text-gray-600">
                          Kommt neu dazu ({sourceRosterDiff.added.length}):{" "}
                          {sourceRosterDiff.added.join(", ")}
                        </p>
                      )}
                      {sourceRosterDiff.removed.length > 0 && (
                        <p className="mt-1 text-xs text-gray-600">
                          Fällt weg ({sourceRosterDiff.removed.length}):{" "}
                          {sourceRosterDiff.removed.join(", ")}
                        </p>
                      )}
                      <p className="mt-1 text-xs text-gray-500">
                        Beim Speichern übernimmt der Regeltermin die Kinder aus
                        dem Angebot. Ihre bisherige Auswahl wird ersetzt.
                      </p>
                    </div>
                  ) : null}
                  {sourcePhaseKidsFromWarning ? (
                    <Alert
                      type="warning"
                      message={sourcePhaseKidsFromWarning}
                      announce="polite"
                    />
                  ) : null}
                  {sourceOverlapWarnings.map((warning) => (
                    <Alert
                      key={warning}
                      type="warning"
                      message={`Hinweis: ${warning}`}
                      announce="polite"
                    />
                  ))}
                </>
              )}
            </div>
          )}

          {form.targetGroupType !== "none" &&
            form.targetGroupType !== "angebot" && (
              <div className="mt-2">
                <Alert
                  type="info"
                  message="Die Kinderliste wird bei der Planung aus allen ausgewählten Zielgruppen gebildet. Überschneidungen werden automatisch entfernt, spätere Gruppenwechsel werden bei einer Neuplanung berücksichtigt. Mit der Schaltfläche darunter können Sie die aktuell passenden Kinder zusätzlich fest auswählen."
                />
              </div>
            )}
          {targetCohort.label && !loadingStudents && !studentLoadError ? (
            <Button
              type="button"
              variant="outline"
              size="compact"
              className="mt-2 self-start"
              onClick={addTargetCohort}
              disabled={
                targetCohort.memberIds.length === 0 ||
                missingTargetCohortCount === 0
              }
            >
              {targetCohortButtonLabel}
            </Button>
          ) : null}
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

      {preserveUnavailableWeekdayRoster && (
        <div className="flex flex-col gap-2">
          <Alert
            type="info"
            message="Die wochentagsspezifischen Zuordnungen können erst bearbeitet werden, wenn Personal- und Kinderliste vollständig geladen sind. Die bestehenden Zuordnungen bleiben beim Speichern unverändert."
          />
          {(loadingStaff || staffLoadError) && staffRosterField}
          {(loadingStudents || studentLoadError) && studentRosterField}
        </div>
      )}

      {!usePerWeekdayRoster &&
        !preserveUnavailableWeekdayRoster &&
        staffRosterField}

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

      {isSeriesFlow && (
        <Field
          label="Maximale Teilnehmerzahl"
          htmlFor="event_max_participants"
          error={fieldErrors.maxParticipants}
        >
          <Input
            id="event_max_participants"
            type="number"
            min={1}
            step={1}
            inputMode="numeric"
            value={form.maxParticipants}
            onChange={(event) => update("maxParticipants", event.target.value)}
            placeholder="unbegrenzt"
            controlSize="compact"
            aria-invalid={Boolean(fieldErrors.maxParticipants)}
            aria-describedby={
              fieldErrors.maxParticipants
                ? "event_max_participants_error"
                : undefined
            }
          />
          <p className="mt-1 text-xs text-gray-500">
            Leer = unbegrenzt. Begrenzt, wie viele Kinder an jedem Termin der
            Reihe teilnehmen können.
          </p>
        </Field>
      )}

      {hasOfferingSource ? (
        <div className="flex flex-col gap-1">
          <span className="text-xs font-semibold text-gray-700">Kinder</span>
          <p className="text-xs text-gray-500">
            Die Kinderliste kommt aus den gewählten Betreuungsangeboten und wird
            automatisch aktuell gehalten. Eine manuelle Auswahl ist bei
            Angeboten als Quelle nicht nötig.
          </p>
        </div>
      ) : (
        !usePerWeekdayRoster &&
        !preserveUnavailableWeekdayRoster &&
        studentRosterField
      )}

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
