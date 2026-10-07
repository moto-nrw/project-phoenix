"use client";

import { useLayoutEffect, useMemo, useRef, useState } from "react";

import { ClosingDayChip } from "~/components/planning/closing-day-marker";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DatePicker } from "~/components/ui/date-picker";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  findClosingDayReason,
  type ClosingDayRange,
} from "~/lib/closing-day-helpers";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useClosingDaysState } from "~/lib/hooks/use-closing-days";
import { createLogger } from "~/lib/logger";
import { staffShiftService } from "~/lib/shift-api";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

const logger = createLogger({ component: "ShiftMoveDialog" });
const STAFF_SHIFT_MAX_BREAK_MINUTES = 300;
const INACTIVE_TYPE_MOVE_MESSAGE =
  "Diese Schichtart ist inaktiv. Bitte wählen Sie für die andere Person eine aktive Schichtart oder „Keine Schichtart“.";

// "Verschieben nach": move one
// materialized shift to another person and/or day in one confirmed, atomic
// PUT. The backend keeps the row ID, so a retry after a lost response cannot
// create duplicates. A moved series occurrence consumes its original slot;
// cross-person targets become standalone because the series belongs to the
// original staff member.

interface ShiftMoveDialogProps {
  readonly isOpen: boolean;
  /** The concrete shift being moved (source). */
  readonly shift: StaffShift;
  /** The shift's current owner — prefills and labels the target person. */
  readonly sourceMember: StaffScheduleStaff;
  /** All staff, offered as move targets. */
  readonly staff: readonly StaffScheduleStaff[];
  /** All shift types (Schichtarten) — the picker offers active ones plus the
   *  one currently attached to this shift (even if inactive). */
  readonly shiftTypes: readonly ShiftType[];
  /** OGS-Schließtage des Tenants (#2032). Fällt der Zieltag auf einen davon,
   *  weist der Dialog darauf hin; die Bestätigung verschiebt trotzdem. */
  readonly closingDayRanges?: readonly ClosingDayRange[];
  readonly onClose: () => void;
  /** Fired after a successful move so the caller revalidates plan caches. */
  readonly onDataChanged: () => void;
}

export function ShiftMoveDialog({
  isOpen,
  shift,
  sourceMember,
  staff,
  shiftTypes,
  closingDayRanges,
  onClose,
  onDataChanged,
}: ShiftMoveDialogProps) {
  // Prefilled from the source shift. The dialog mounts fresh per open (the grid
  // renders it only while a shift is selected), so plain initializers suffice.
  const [targetStaffId, setTargetStaffId] = useState(shift.staffId);
  const [targetDate, setTargetDate] = useState(shift.date);
  const [startTime, setStartTime] = useState(shift.startTime);
  const [endTime, setEndTime] = useState(shift.endTime);
  const [breakMinutesStr, setBreakMinutesStr] = useState(
    String(shift.breakMinutes),
  );
  const [shiftTypeId, setShiftTypeId] = useState(shift.shiftTypeId ?? "");

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [isMoving, setIsMoving] = useState(false);
  // React state does not update synchronously. The ref closes the same-render
  // double-click / double-Enter window before the loading render commits.
  const moveInFlightRef = useRef(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  // „Wiederholen“ verschiebt mit dem aktuellen Formularstand.
  const latestMoveRef = useRef<() => void>(() => undefined);

  const personOptions = useMemo(
    () =>
      staff.map((member) => ({
        value: member.id,
        label: `${member.lastName}, ${member.firstName}`,
      })),
    [staff],
  );

  const typeOptions = useMemo(() => {
    const opts: { value: string; label: string }[] = [
      { value: "", label: "Keine Schichtart" },
    ];
    for (const type of shiftTypes) {
      if (type.isActive || type.id === shift.shiftTypeId) {
        opts.push({
          value: type.id,
          label: type.isActive ? type.name : `${type.name} (inaktiv)`,
        });
      }
    }
    return opts;
  }, [shiftTypes, shift.shiftTypeId]);

  const timesValid = startTime !== "" && endTime !== "" && startTime < endTime;
  const breakMax = timesValid
    ? Math.min(
        STAFF_SHIFT_MAX_BREAK_MINUTES,
        shiftDurationMinutes(startTime, endTime) ??
          STAFF_SHIFT_MAX_BREAK_MINUTES,
      )
    : STAFF_SHIFT_MAX_BREAK_MINUTES;
  const breakMinutes = parseBreakMinutes(breakMinutesStr, breakMax);
  const breakValid = breakMinutes !== null;
  const movingToOtherPerson = targetStaffId !== shift.staffId;
  const selectedShiftType = shiftTypes.find((type) => type.id === shiftTypeId);
  const inactiveTypeBlocksMove =
    movingToOtherPerson && selectedShiftType?.isActive === false;
  const targetClosingDayState = useClosingDaysState(targetDate, targetDate);
  const closingDayLookupLoading = targetClosingDayState.isLoading;
  const canSubmit =
    targetStaffId !== "" &&
    targetDate !== "" &&
    timesValid &&
    breakValid &&
    !inactiveTypeBlocksMove &&
    !closingDayLookupLoading;

  // Schließtag am Zieltag (#2032): Hinweis im Formular und in der ohnehin
  // vorhandenen Bestätigung — verschoben wird trotzdem, nichts wird gesperrt.
  const targetClosingReason =
    targetClosingDayState.closingDays.get(targetDate) ??
    findClosingDayReason(closingDayRanges, targetDate);

  const targetMember = staff.find((member) => member.id === targetStaffId);
  const sourceName = `${sourceMember.firstName} ${sourceMember.lastName}`;
  const targetName = targetMember
    ? `${targetMember.firstName} ${targetMember.lastName}`
    : sourceName;
  const handleSubmit = () => {
    formErrors.clear();
    if (closingDayLookupLoading) return;
    if (targetStaffId === "") {
      formErrors.invalid("Bitte wählen Sie eine Person aus.", {
        target_staff_id: "Bitte wählen Sie eine Person aus.",
      });
      return;
    }
    if (targetDate === "") {
      formErrors.invalid("Bitte wählen Sie einen Tag aus.");
      return;
    }
    if (!timesValid) {
      formErrors.invalid("Das Ende muss nach dem Beginn liegen.", {
        end_time: "Bitte eine spätere Zeit wählen.",
      });
      return;
    }
    if (!breakValid) {
      formErrors.invalid(
        `Die Pause muss zwischen 0 und ${breakMax} Minuten liegen.`,
        { break_minutes: `Bitte 0 bis ${breakMax} Minuten eintragen.` },
      );
      return;
    }
    if (inactiveTypeBlocksMove) {
      formErrors.invalid(INACTIVE_TYPE_MOVE_MESSAGE, {
        shift_type_id: "Bitte eine aktive Schichtart wählen.",
      });
      return;
    }
    setConfirmOpen(true);
  };

  const finishSuccess = () => {
    onDataChanged();
    onClose();
  };

  const moveShift = async (resolvedShiftTypeId: string | null) => {
    try {
      await staffShiftService.moveShift(shift.id, {
        sourceStaffId: shift.staffId,
        targetStaffId,
        date: targetDate,
        startTime,
        endTime,
        breakMinutes: breakMinutes ?? 0,
        shiftTypeId: resolvedShiftTypeId,
      });
      finishSuccess();
    } catch (err: unknown) {
      logger.error("shift_move_failed", {
        shift_id: shift.id,
        target_staff_id: targetStaffId,
        error: err instanceof Error ? err.message : String(err),
      });
      // Back to the form: the confirmation closes and the dialog shows why.
      setConfirmOpen(false);
      moveInFlightRef.current = false;
      setIsMoving(false);
      await formErrors.show(err, {
        object: "die Schicht",
        retry: () => latestMoveRef.current(),
      });
    }
  };

  const executeMove = async () => {
    if (moveInFlightRef.current) return;
    moveInFlightRef.current = true;
    setIsMoving(true);
    formErrors.clear();
    const resolvedShiftTypeId = shiftTypeId === "" ? null : shiftTypeId;
    await moveShift(resolvedShiftTypeId);
  };

  useLayoutEffect(() => {
    latestMoveRef.current = handleSubmit;
  });

  const breakSuffix =
    breakMinutes && breakMinutes > 0 ? `, Pause ${breakMinutes} min` : "";

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={isMoving}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={handleSubmit}
        disabled={isMoving || !canSubmit}
      >
        Verschieben
      </Button>
    </>
  );

  return (
    <>
      {/* Hidden while the confirmation is open — both share the same fixed
          z-index, so stacking them would bury this modal's footer. */}
      <Modal
        isOpen={isOpen && !confirmOpen}
        onClose={onClose}
        title="Schicht verschieben"
        footer={footer}
      >
        <div ref={formRef} className="space-y-4 text-sm">
          <FormErrorAlert message={formErrors.error} />
          <p className="text-gray-600">
            Verschiebt die Schicht von{" "}
            <span className="font-medium text-gray-900">{sourceName}</span> am{" "}
            {formatLongDate(shift.date)}.
          </p>
          <Field label="Zielperson">
            <CustomSelect
              value={targetStaffId}
              options={personOptions}
              onChange={setTargetStaffId}
              name="target_staff_id"
              invalid={!!formErrors.fieldError("target_staff_id")}
              ariaLabel="Zielperson"
              placeholder="Person auswählen"
            />
          </Field>
          {/* FieldGroup (div), not Field (label): the DatePicker trigger and
                calendar are buttons; inside a <label> a click would re-dispatch
                onto the first labelable descendant. */}
          <FieldGroup label="Zieltag">
            <DatePicker
              value={targetDate === "" ? null : parseISODate(targetDate)}
              onChange={(picked) =>
                setTargetDate(picked ? toISODate(picked) : "")
              }
              dropdownPlacement="down"
              placeholder="Datum auswählen"
            />
            {targetClosingReason !== undefined && (
              <ClosingDayChip reason={targetClosingReason} className="mt-1.5" />
            )}
          </FieldGroup>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="Beginn">
              <Input
                type="time"
                name="start_time"
                error={formErrors.fieldError("start_time")}
                value={startTime}
                onChange={(e) => setStartTime(e.target.value)}
                className="tabular-nums"
              />
            </Field>
            <Field label="Ende">
              <Input
                type="time"
                name="end_time"
                error={formErrors.fieldError("end_time")}
                value={endTime}
                onChange={(e) => setEndTime(e.target.value)}
                className="tabular-nums"
              />
            </Field>
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label="Pause (Minuten)">
              <Input
                type="number"
                name="break_minutes"
                error={formErrors.fieldError("break_minutes")}
                min={0}
                max={breakMax}
                inputMode="numeric"
                value={breakMinutesStr}
                onChange={(e) => setBreakMinutesStr(e.target.value)}
                className="tabular-nums"
              />
            </Field>
          </div>
          {typeOptions.length > 1 && (
            <Field label="Schichtart">
              <CustomSelect
                value={shiftTypeId}
                options={typeOptions}
                onChange={setShiftTypeId}
                name="shift_type_id"
                invalid={!!formErrors.fieldError("shift_type_id")}
                ariaLabel="Schichtart"
                placeholder="Keine Schichtart"
              />
            </Field>
          )}
          {inactiveTypeBlocksMove && (
            <Alert type="warning" message={INACTIVE_TYPE_MOVE_MESSAGE} />
          )}
        </div>
      </Modal>
      <ConfirmationModal
        isOpen={isOpen && confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => void executeMove()}
        title="Verschieben bestätigen"
        confirmText="Verschieben"
        isConfirmLoading={isMoving}
        // Keep the confirmation stable while the atomic request is in flight.
        isDismissDisabled={isMoving}
      >
        <div className="space-y-2 text-sm text-gray-700">
          <p>Die Schicht wird verschoben:</p>
          <ul className="space-y-1">
            <li>
              <span className="font-semibold text-gray-500">Von:</span>{" "}
              {sourceName}, {formatLongDate(shift.date)}
            </li>
            <li>
              <span className="font-semibold text-gray-500">Nach:</span>{" "}
              {targetName}, {formatLongDate(targetDate)}
            </li>
            <li className="tabular-nums">
              <span className="font-semibold text-gray-500">Zeit:</span>{" "}
              {startTime}–{endTime}
              {breakSuffix}
            </li>
          </ul>
          {targetClosingReason !== undefined && (
            <p className="text-sm leading-relaxed text-gray-600">
              Am Zieltag ist ein Schließtag hinterlegt
              {targetClosingReason === "" ? "" : `: ${targetClosingReason}`}.
              Verschieben ist trotzdem möglich.
            </p>
          )}
        </div>
      </ConfirmationModal>
    </>
  );
}

function Field({
  label,
  children,
}: {
  readonly label: string;
  readonly children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {label}
      </span>
      {children}
    </label>
  );
}

// Field variant for control groups whose children are buttons (DatePicker):
// wrapping them in a <label> would re-dispatch a click onto the first labelable
// descendant and toggle the wrong control (same rationale as shift-edit-modal).
function FieldGroup({
  label,
  children,
}: {
  readonly label: string;
  readonly children: React.ReactNode;
}) {
  return (
    <div>
      <span className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {label}
      </span>
      {children}
    </div>
  );
}

function formatLongDate(isoDate: string): string {
  if (isoDate === "") return "";
  return parseISODate(isoDate).toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    weekday: "long",
    day: "2-digit",
    month: "long",
    year: "numeric",
  });
}

function shiftDurationMinutes(
  startTime: string,
  endTime: string,
): number | null {
  const start = parseClockMinutes(startTime);
  const end = parseClockMinutes(endTime);
  if (start === null || end === null || end <= start) return null;
  return end - start;
}

function parseClockMinutes(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) return null;
  return hours * 60 + minutes;
}

function parseBreakMinutes(raw: string, maxMinutes: number): number | null {
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  const n = Number(trimmed);
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0 || n > maxMinutes) {
    return null;
  }
  return n;
}
