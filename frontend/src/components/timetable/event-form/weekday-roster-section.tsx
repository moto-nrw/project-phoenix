"use client";

// Per-weekday staff and child rosters of a Regeltermin (#2129).
//
// A series that runs Mo–Fr used to carry ONE roster. Schools whose daily
// routine is identical but whose responsibilities rotate had to model five
// near-identical Regeltermine. This section lets the planner keep one series
// and differentiate the days: a mode switch, a weekday picker, and — while a
// weekday is selected — that day's own Personal / Zuständige Person / Kinder.
//
// The shared default stays visible as a concept: switching the mode on seeds
// every weekday from the series roster, and "Auf alle Tage übertragen" pushes
// the current day back onto all of them. Days that differ from the first
// weekday are called out explicitly so a deviation is never invisible.

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { Field } from "./field";
import {
  WEEKDAY_LABELS,
  WEEKDAY_SHORT_LABELS,
  rosterForWeekday,
  weekdayDeviatesFromBaseline,
  type EventFormState,
  type PersonOption,
  type WeekdayRosterState,
} from "./form-model";
import { MultiSelectField } from "./multi-select-field";

export interface WeekdayRosterSectionProps {
  form: EventFormState;
  /** Sorted ISO weekdays the series runs on. */
  weekdays: number[];
  activeWeekday: number;
  setActiveWeekday: (weekday: number) => void;
  setPerWeekdayRoster: (enabled: boolean) => void;
  setWeekdayRoster: (weekday: number, roster: WeekdayRosterState) => void;
  applyActiveWeekdayToAll: () => void;
  staff: PersonOption[];
  students: PersonOption[];
  studentBulkOptions: Array<{
    key: string;
    label: string;
    memberIds: string[];
  }>;
}

export function WeekdayRosterSection({
  form,
  weekdays,
  activeWeekday,
  setActiveWeekday,
  setPerWeekdayRoster,
  setWeekdayRoster,
  applyActiveWeekdayToAll,
  staff,
  students,
  studentBulkOptions,
}: Readonly<WeekdayRosterSectionProps>) {
  const deviatingWeekdays = weekdays.filter((weekday) =>
    weekdayDeviatesFromBaseline(form, weekday, weekdays),
  );

  return (
    <div className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-semibold text-gray-700">
          Personal und Kinder
        </span>
        <SegmentedControl
          items={[
            { value: "shared", label: "Für alle Tage gleich" },
            { value: "per-weekday", label: "Pro Wochentag" },
          ]}
          value={form.perWeekdayRoster ? "per-weekday" : "shared"}
          onChange={(next) => setPerWeekdayRoster(next === "per-weekday")}
          ariaLabel="Zuordnung von Personal und Kindern"
        />
      </div>

      {weekdays.length < 2 ? (
        <p className="text-xs text-gray-500">
          Der Regeltermin findet nur an einem Wochentag statt. Sobald weitere
          Wochentage ausgewählt sind, kann die Zuordnung pro Tag festgelegt
          werden.
        </p>
      ) : (
        <p className="text-xs leading-5 text-gray-600">
          {form.perWeekdayRoster
            ? "Jeder Wochentag hat eine eigene Zuordnung. Änderungen gelten nur für den ausgewählten Tag, bis du sie auf alle Tage überträgst."
            : "Alle Wochentage teilen sich eine Zuordnung. Wechsle zu „Pro Wochentag“, wenn montags andere Personen oder Kinder zuständig sind als dienstags."}
        </p>
      )}

      {form.perWeekdayRoster && weekdays.length >= 2 && (
        <>
          <SegmentedControl
            items={weekdays.map((weekday) => ({
              value: String(weekday),
              label: WEEKDAY_SHORT_LABELS[weekday] ?? String(weekday),
            }))}
            value={String(activeWeekday)}
            onChange={(next) => setActiveWeekday(Number(next))}
            ariaLabel="Wochentag für die Zuordnung"
            fullWidth
          />

          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium text-gray-900">
              {WEEKDAY_LABELS[activeWeekday] ?? ""}
            </span>
            {deviatingWeekdays.includes(activeWeekday) ? (
              <StatusBadge
                label="abweichend"
                tone="orange"
                title={`Die Zuordnung am ${WEEKDAY_LABELS[activeWeekday]} unterscheidet sich von ${WEEKDAY_LABELS[weekdays[0] ?? 1]}`}
              />
            ) : (
              <StatusBadge label="wie üblich" tone="gray" />
            )}
            <Button
              type="button"
              variant="outline"
              size="compact"
              className="ml-auto"
              onClick={applyActiveWeekdayToAll}
            >
              Auf alle Tage übertragen
            </Button>
          </div>

          {deviatingWeekdays.length > 0 && (
            <p className="text-xs text-gray-600" role="status">
              {`Abweichende Tage: ${deviatingWeekdays
                .map((weekday) => WEEKDAY_LABELS[weekday])
                .join(
                  ", ",
                )}. Verglichen mit ${WEEKDAY_LABELS[weekdays[0] ?? 1]}.`}
            </p>
          )}
        </>
      )}

      {form.perWeekdayRoster && weekdays.length >= 2 ? (
        <WeekdayRosterFields
          key={activeWeekday}
          roster={rosterForWeekday(form, activeWeekday)}
          weekday={activeWeekday}
          onChange={(roster) => setWeekdayRoster(activeWeekday, roster)}
          staff={staff}
          students={students}
          studentBulkOptions={studentBulkOptions}
          protectedStudentIds={
            form.protectedStudentAssignments.find(
              (assignment) => assignment.weekday === activeWeekday,
            )?.studentIds ?? []
          }
        />
      ) : null}
    </div>
  );
}

function WeekdayRosterFields({
  roster,
  weekday,
  onChange,
  staff,
  students,
  studentBulkOptions,
  protectedStudentIds,
}: Readonly<{
  roster: WeekdayRosterState;
  weekday: number;
  onChange: (roster: WeekdayRosterState) => void;
  staff: PersonOption[];
  students: PersonOption[];
  studentBulkOptions: Array<{
    key: string;
    label: string;
    memberIds: string[];
  }>;
  protectedStudentIds: readonly string[];
}>) {
  const dayLabel = WEEKDAY_LABELS[weekday] ?? "";
  return (
    <div className="flex flex-col gap-3 border-t border-gray-200 pt-3">
      <MultiSelectField
        label={`Personal am ${dayLabel}`}
        options={staff}
        value={roster.staffIds}
        onChange={(ids) =>
          onChange({
            ...roster,
            staffIds: ids,
            primaryStaffId: ids.includes(roster.primaryStaffId)
              ? roster.primaryStaffId
              : "",
          })
        }
        metadata="staff"
      />

      {roster.staffIds.length > 0 && (
        <Field
          label={`Zuständige Person am ${dayLabel}`}
          htmlFor={`event_primary_staff_${weekday}`}
        >
          <CustomSelect
            id={`event_primary_staff_${weekday}`}
            ariaLabel={`Zuständige Person am ${dayLabel}`}
            value={roster.primaryStaffId}
            options={[
              { value: "", label: "Keine Auswahl" },
              ...staff
                .filter((person) => roster.staffIds.includes(person.id))
                .map((person) => ({ value: person.id, label: person.name })),
            ]}
            onChange={(next) => onChange({ ...roster, primaryStaffId: next })}
          />
        </Field>
      )}

      <MultiSelectField
        label={`Kinder am ${dayLabel}`}
        options={students}
        value={roster.studentIds}
        onChange={(ids) => onChange({ ...roster, studentIds: ids })}
        metadata="student"
        bulkOptions={studentBulkOptions}
        protectedValues={protectedStudentIds}
      />

      {roster.staffIds.length === 0 && (
        <Alert
          type="info"
          message={`Am ${dayLabel} ist noch niemand eingeteilt. Der Termin wird trotzdem geplant und erscheint als Personallücke im Dienstplan.`}
        />
      )}
    </div>
  );
}
