"use client";

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { formatDayCount } from "~/lib/absence-helpers";
import type {
  AbsenceType,
  AbsenceTypeAllowanceSummary,
} from "~/lib/absence-type-api";
import { todayISO, toISODate } from "~/lib/date-helpers";
import { staffAbsenceService, type StaffAbsenceRow } from "~/lib/staff-api";
import type { SickReportStaff } from "~/components/staff/sick-report-modal";

type AllowanceEntry = {
  type: AbsenceType;
  summary: AbsenceTypeAllowanceSummary;
};

type BookingProps = {
  readonly staff: SickReportStaff;
  readonly year: number;
  readonly entries: readonly AllowanceEntry[];
  readonly absences: readonly StaffAbsenceRow[];
  readonly onClose: () => void;
  readonly onSaved: () => Promise<void>;
};

const ALLOWANCE_COUNTED_STATUSES = new Set([
  "reported",
  "approved",
  "requested",
  "question",
]);

function absenceCoverageOn(row: StaffAbsenceRow, date: string): number {
  if (date < row.date_start || date > row.date_end) return 0;
  if (row.date_start === row.date_end) return row.half_day ? 0.5 : 1;
  if (date === row.date_start && row.start_half_day) return 0.5;
  if (date === row.date_end && row.end_half_day) return 0.5;
  return 1;
}

function existingCoverage(
  absences: readonly StaffAbsenceRow[],
  typeId: string,
  date: string,
): number {
  return Math.max(
    0,
    ...absences
      .filter(
        (row) =>
          row.absence_type_id === typeId &&
          ALLOWANCE_COUNTED_STATUSES.has(row.status),
      )
      .map((row) => absenceCoverageOn(row, date)),
  );
}

function incrementalWorkingDays(
  from: string,
  to: string,
  halfDay: boolean,
  typeId: string,
  absences: readonly StaffAbsenceRow[],
): number {
  const start = new Date(`${from}T00:00:00Z`);
  const end = new Date(`${to}T00:00:00Z`);
  const calendarDays = Math.floor(
    (end.getTime() - start.getTime()) / 86_400_000,
  );
  let days = 0;
  for (let offset = 0; offset <= calendarDays; offset += 1) {
    const day = new Date(start);
    day.setUTCDate(start.getUTCDate() + offset);
    if (day.getUTCDay() === 0 || day.getUTCDay() === 6) continue;
    const currentCoverage = existingCoverage(absences, typeId, toISODate(day));
    days += Math.max(0, (halfDay ? 0.5 : 1) - currentCoverage);
  }
  return days;
}

type BookingDraft = {
  typeId: string;
  dateStart: string;
  dateEnd: string;
  halfDay: boolean;
  note: string;
};

function projectBooking(props: BookingProps, draft: BookingDraft) {
  const selected = props.entries.find(
    (entry) => entry.type.id === draft.typeId,
  );
  const requested = incrementalWorkingDays(
    draft.dateStart,
    draft.halfDay ? draft.dateStart : draft.dateEnd,
    draft.halfDay,
    draft.typeId,
    props.absences,
  );
  const remaining = (selected?.summary.remainingDays ?? 0) - requested;
  const overrun = remaining < 0;
  return {
    selected,
    remaining,
    overrun,
    blocked: overrun && selected?.type.overrunPolicy === "block",
  };
}

function useBookingSubmission(
  props: BookingProps,
  draft: BookingDraft,
  projection: ReturnType<typeof projectBooking>,
) {
  const [saving, setSaving] = useState(false);
  const toast = useToast();
  const save = async () => {
    if (!projection.selected || projection.blocked) return;
    setSaving(true);
    try {
      await staffAbsenceService.createAbsence(props.staff.id, {
        absence_type: "other",
        absence_type_id: projection.selected.type.id,
        date_start: draft.dateStart,
        date_end: draft.halfDay ? draft.dateStart : draft.dateEnd,
        half_day: draft.halfDay || undefined,
        note: draft.note.trim() || undefined,
      });
      toast.success("Abwesenheit eingetragen.");
      await props.onSaved();
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Abwesenheit konnte nicht eingetragen werden.",
      );
    } finally {
      setSaving(false);
    }
  };
  return { saving, save };
}

function useBookingState(props: BookingProps) {
  const [typeId, setTypeId] = useState(props.entries[0]?.type.id ?? "");
  const [dateStart, setDateStart] = useState(todayISO());
  const [dateEnd, setDateEnd] = useState(todayISO());
  const [halfDay, setHalfDay] = useState(false);
  const [note, setNote] = useState("");
  const draft = { typeId, dateStart, dateEnd, halfDay, note };
  const projection = projectBooking(props, draft);
  const submission = useBookingSubmission(props, draft, projection);
  return {
    draft,
    update: { setTypeId, setDateStart, setDateEnd, setHalfDay, setNote },
    ...projection,
    ...submission,
  };
}

type BookingState = ReturnType<typeof useBookingState>;

function BookingFooter({
  state,
  onClose,
}: {
  state: BookingState;
  onClose: () => void;
}) {
  const disabled =
    state.saving ||
    state.blocked ||
    !state.draft.dateStart ||
    !state.draft.dateEnd;
  return (
    <div className="flex w-full justify-end gap-2">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={state.saving}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => void state.save()}
        disabled={disabled}
      >
        Eintragen
      </Button>
    </div>
  );
}

function DateField({
  id,
  label,
  value,
  min,
  max,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  min: string;
  max: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div>
      <label
        htmlFor={id}
        className="mb-1 block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <ISODatePicker
        id={id}
        value={value}
        min={min}
        max={max}
        onChange={onChange}
        calendarLayout="popover"
        hideClearButton
        disabled={disabled}
      />
    </div>
  );
}

function BookingDates({ state, year }: { state: BookingState; year: number }) {
  return (
    <>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <DateField
          id="custom-absence-start"
          label="Von"
          value={state.draft.dateStart}
          min={`${year}-01-01`}
          max={`${year}-12-31`}
          onChange={(value) => {
            state.update.setDateStart(value);
            if (value > state.draft.dateEnd) state.update.setDateEnd(value);
          }}
        />
        <DateField
          id="custom-absence-end"
          label="Bis"
          value={
            state.draft.halfDay ? state.draft.dateStart : state.draft.dateEnd
          }
          min={state.draft.dateStart}
          max={`${year}-12-31`}
          disabled={state.draft.halfDay}
          onChange={state.update.setDateEnd}
        />
      </div>
      <label
        htmlFor="custom-absence-half"
        className="flex items-center gap-2 text-sm font-medium text-gray-700"
      >
        <Checkbox
          id="custom-absence-half"
          checked={state.draft.halfDay}
          onChange={(event) => {
            state.update.setHalfDay(event.target.checked);
            if (event.target.checked)
              state.update.setDateEnd(state.draft.dateStart);
          }}
        />
        Halber Tag
      </label>
    </>
  );
}

function BookingStatus({ state }: { state: BookingState }) {
  return (
    <>
      {state.selected ? (
        <p className="text-sm text-gray-600">
          Danach verbleiben {formatDayCount(state.remaining)}.
        </p>
      ) : null}
      {state.overrun ? (
        <Alert
          type={state.blocked ? "error" : "warning"}
          message={
            state.blocked
              ? "Das Kontingent reicht nicht aus. Die Buchung ist nicht möglich."
              : "Das Kontingent reicht nicht aus. Sie können trotzdem buchen."
          }
        />
      ) : null}
    </>
  );
}

function BookingFields({
  state,
  entries,
  year,
}: {
  state: BookingState;
  entries: readonly AllowanceEntry[];
  year: number;
}) {
  return (
    <div className="space-y-4">
      <div>
        <label
          htmlFor="custom-absence-type"
          className="mb-1 block text-sm font-medium text-gray-700"
        >
          Abwesenheitsart
        </label>
        <CustomSelect
          id="custom-absence-type"
          value={state.draft.typeId}
          onChange={state.update.setTypeId}
          options={entries.map((entry) => ({
            value: entry.type.id,
            label: entry.type.name,
          }))}
        />
      </div>
      <BookingDates state={state} year={year} />
      <BookingStatus state={state} />
      <Input
        name="custom-absence-note"
        label="Notiz (optional)"
        value={state.draft.note}
        onChange={(event) => state.update.setNote(event.target.value)}
      />
    </div>
  );
}

export function CustomAllowanceAbsenceModal(props: BookingProps) {
  const state = useBookingState(props);
  return (
    <Modal
      isOpen
      onClose={() => !state.saving && props.onClose()}
      title={`Abwesenheit eintragen: ${props.staff.firstName} ${props.staff.lastName}`}
      footer={<BookingFooter state={state} onClose={props.onClose} />}
    >
      <BookingFields state={state} entries={props.entries} year={props.year} />
    </Modal>
  );
}
