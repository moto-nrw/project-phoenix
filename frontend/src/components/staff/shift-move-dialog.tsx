"use client";

import { useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DatePicker } from "~/components/ui/date-picker";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { ShiftApiError, staffShiftService } from "~/lib/shift-api";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

const logger = createLogger({ component: "ShiftMoveDialog" });
const STAFF_SHIFT_MAX_BREAK_MINUTES = 300;

// "Verschieben nach" (docs/05-dienstplan.md Abschnitt 2.7, US-D6): move one
// materialized shift to another person and/or day in a single confirmed
// operation. Two call strategies, decided by whether the person changes
// (Spec-Abgleich Inkrement 3, Abschnitt B7):
//
//  - Same person, only day/time/type change → ONE atomic `PUT /staff-shifts/{id}`
//    (date and times are updatable; the backend ignores staff_id on update,
//    which is harmless here because the person stays the same). No delete/create.
//  - Person changes → the backend has NO atomic re-hang endpoint (a PUT cannot
//    move a shift to another person). Transitional two-call solution: DELETE the
//    source shift, then POST it for the target person. For a series-backed row
//    the single DELETE records a `staff_shift_series_exceptions` row and leaves
//    the series intact — never a series DELETE. If the POST fails after the
//    DELETE already succeeded the move is half-applied (not atomic); the dialog
//    stays open and shows a recovery instruction with the full source data so
//    the admin can re-create the original shift by hand.

interface ShiftMoveDialogProps {
  readonly isOpen: boolean;
  /** The concrete shift being moved (source). */
  readonly shift: StaffShift;
  /** The shift's current owner — prefills the target person and labels the
   *  confirmation / recovery views. */
  readonly sourceMember: StaffScheduleStaff;
  /** All staff, offered as move targets. */
  readonly staff: readonly StaffScheduleStaff[];
  /** All shift types (Schichtarten) — the picker offers active ones plus the
   *  one currently attached to this shift (even if inactive). */
  readonly shiftTypes: readonly ShiftType[];
  readonly onClose: () => void;
  /** Fired whenever server data changed — after a successful move, and also
   *  when a cross-person move is left half-applied (DELETE succeeded, POST
   *  failed) — so the caller revalidates its Dienstplan caches instead of
   *  keeping the already-deleted source shift on screen. */
  readonly onDataChanged: () => void;
}

function moveErrorMessage(err: unknown): string {
  if (err instanceof ShiftApiError) {
    const detail = err.detail.toLowerCase();
    if (err.status === 409 || detail.includes("overlap")) {
      return "Diese Schicht überschneidet sich mit einer bestehenden Schicht.";
    }
    if (err.status === 400) {
      return "Ungültige Schichtdaten. Bitte prüfen Sie Beginn, Ende und Pause.";
    }
    if (err.status === 401) {
      return "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
    }
    if (err.status === 403) {
      return "Sie haben keine Berechtigung, diese Schicht zu verschieben.";
    }
    return err.detail || "Die Schicht konnte nicht verschoben werden.";
  }
  return getApiErrorMessage(
    err,
    "verschieben",
    "Schicht",
    "Verschieben fehlgeschlagen.",
  );
}

export function ShiftMoveDialog({
  isOpen,
  shift,
  sourceMember,
  staff,
  shiftTypes,
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
  const [error, setError] = useState<string | null>(null);
  // Set only when the source DELETE succeeded but the target POST failed: the
  // form is replaced by the manual-restore instruction.
  const [recovery, setRecovery] = useState(false);

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
  const canSubmit =
    targetStaffId !== "" && targetDate !== "" && timesValid && breakValid;

  const targetMember = staff.find((member) => member.id === targetStaffId);
  const sourceName = `${sourceMember.firstName} ${sourceMember.lastName}`;
  const targetName = targetMember
    ? `${targetMember.firstName} ${targetMember.lastName}`
    : sourceName;
  const sourceTypeName =
    shift.shiftTypeName ??
    shiftTypes.find((type) => type.id === shift.shiftTypeId)?.name ??
    "Keine Schichtart";

  const handleSubmit = () => {
    setError(null);
    if (targetStaffId === "") {
      setError("Bitte eine Zielperson auswählen.");
      return;
    }
    if (targetDate === "") {
      setError("Bitte einen Zieltag auswählen.");
      return;
    }
    if (!timesValid) {
      setError("Ende muss nach Beginn liegen.");
      return;
    }
    if (!breakValid) {
      setError(`Pause muss eine ganze Zahl zwischen 0 und ${breakMax} sein.`);
      return;
    }
    setConfirmOpen(true);
  };

  const finishSuccess = () => {
    onDataChanged();
    onClose();
  };

  const moveSamePerson = async (resolvedShiftTypeId: string | null) => {
    try {
      await staffShiftService.updateShift(shift.id, {
        // Same person → staff_id is unchanged (and ignored on update anyway);
        // only date / times / break / type move.
        staffId: shift.staffId,
        date: targetDate,
        startTime,
        endTime,
        breakMinutes: breakMinutes ?? 0,
        shiftTypeId: resolvedShiftTypeId,
      });
      finishSuccess();
    } catch (err: unknown) {
      logger.error("shift_move_update_failed", {
        shift_id: shift.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(moveErrorMessage(err));
      setConfirmOpen(false);
      setIsMoving(false);
    }
  };

  const moveOtherPerson = async (resolvedShiftTypeId: string | null) => {
    try {
      // A series-backed row's single DELETE records an exception and leaves the
      // series intact — never a series DELETE.
      await staffShiftService.deleteShift(shift.id);
    } catch (err: unknown) {
      // The source shift still exists → a plain error, no recovery needed.
      logger.error("shift_move_delete_failed", {
        shift_id: shift.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(moveErrorMessage(err));
      setConfirmOpen(false);
      setIsMoving(false);
      return;
    }
    try {
      await staffShiftService.createShift({
        staffId: targetStaffId,
        date: targetDate,
        startTime,
        endTime,
        breakMinutes: breakMinutes ?? 0,
        shiftTypeId: resolvedShiftTypeId,
      });
      finishSuccess();
    } catch (err: unknown) {
      // The DELETE already removed the source shift but the POST failed: the
      // move is half-applied (the atomic re-hang endpoint is the missing piece).
      // Keep the dialog open and show the manual-restore instruction.
      logger.error("shift_move_create_failed", {
        shift_id: shift.id,
        target_staff_id: targetStaffId,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(moveErrorMessage(err));
      setRecovery(true);
      setConfirmOpen(false);
      setIsMoving(false);
      // The DELETE went through, so the grid must stop showing the source
      // shift while the admin follows the restore instruction.
      onDataChanged();
    }
  };

  const executeMove = async () => {
    setIsMoving(true);
    setError(null);
    const resolvedShiftTypeId = shiftTypeId === "" ? null : shiftTypeId;
    if (targetStaffId === shift.staffId) {
      await moveSamePerson(resolvedShiftTypeId);
    } else {
      await moveOtherPerson(resolvedShiftTypeId);
    }
  };

  const breakSuffix =
    breakMinutes && breakMinutes > 0 ? `, Pause ${breakMinutes} min` : "";

  const footer = recovery ? (
    <Button type="button" variant="primary" size="md" onClick={onClose}>
      Schließen
    </Button>
  ) : (
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
        {recovery ? (
          <div className="space-y-3 text-sm">
            {error && <Alert type="error" message={error} />}
            <p className="text-gray-700">
              Die ursprüngliche Schicht wurde gelöscht, aber die verschobene
              Schicht konnte nicht angelegt werden. Bitte legen Sie die
              ursprüngliche Schicht manuell neu an:
            </p>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 rounded-md border border-gray-200 bg-gray-50 p-3">
              <dt className="font-semibold text-gray-500">Person</dt>
              <dd className="text-gray-900">{sourceName}</dd>
              <dt className="font-semibold text-gray-500">Tag</dt>
              <dd className="text-gray-900">{formatLongDate(shift.date)}</dd>
              <dt className="font-semibold text-gray-500">Zeit</dt>
              <dd className="text-gray-900 tabular-nums">
                {shift.startTime}–{shift.endTime}
              </dd>
              <dt className="font-semibold text-gray-500">Pause</dt>
              <dd className="text-gray-900">
                {shift.breakMinutes > 0 ? `${shift.breakMinutes} min` : "keine"}
              </dd>
              <dt className="font-semibold text-gray-500">Schichtart</dt>
              <dd className="text-gray-900">{sourceTypeName}</dd>
            </dl>
          </div>
        ) : (
          <div className="space-y-4 text-sm">
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
            </FieldGroup>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Field label="Beginn">
                <input
                  type="time"
                  value={startTime}
                  onChange={(e) => setStartTime(e.target.value)}
                  className="w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:border-[#83CD2D] focus:outline-none"
                />
              </Field>
              <Field label="Ende">
                <input
                  type="time"
                  value={endTime}
                  onChange={(e) => setEndTime(e.target.value)}
                  className="w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:border-[#83CD2D] focus:outline-none"
                />
              </Field>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Field label="Pause (Minuten)">
                <input
                  type="number"
                  min={0}
                  max={breakMax}
                  inputMode="numeric"
                  value={breakMinutesStr}
                  onChange={(e) => setBreakMinutesStr(e.target.value)}
                  className="w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:border-[#83CD2D] focus:outline-none"
                />
              </Field>
            </div>
            {typeOptions.length > 1 && (
              <Field label="Schichtart">
                <CustomSelect
                  value={shiftTypeId}
                  options={typeOptions}
                  onChange={setShiftTypeId}
                  ariaLabel="Schichtart"
                  placeholder="Keine Schichtart"
                />
              </Field>
            )}
            {error && <Alert type="error" message={error} />}
          </div>
        )}
      </Modal>
      <ConfirmationModal
        isOpen={isOpen && confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => void executeMove()}
        title="Verschieben bestätigen"
        confirmText="Verschieben"
        isConfirmLoading={isMoving}
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
