"use client";

import { useMemo, useRef, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { ShiftApiError, staffShiftService } from "~/lib/shift-api";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

const logger = createLogger({ component: "ShiftMoveDialog" });
const STAFF_SHIFT_MAX_BREAK_MINUTES = 300;
const INACTIVE_TYPE_MOVE_MESSAGE =
  "Diese Schichtart ist inaktiv. Bitte wählen Sie für die andere Person eine aktive Schichtart oder „Keine Schichtart“.";

// "Verschieben nach" (docs/05-dienstplan.md Abschnitt 2.7, US-D6): move one
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
  readonly onClose: () => void;
  /** Fired after a successful move so the caller revalidates plan caches. */
  readonly onDataChanged: () => void;
}

function moveErrorMessage(err: unknown): string {
  if (err instanceof ShiftApiError) {
    const detail = err.detail.toLowerCase();
    if (detail.includes("overlap")) {
      return "Diese Schicht überschneidet sich mit einer bestehenden Schicht.";
    }
    if (err.status === 409) {
      return "Die Schicht wurde zwischenzeitlich geändert. Bitte laden Sie den Dienstplan neu.";
    }
    // Origin-link rejections from validateOriginLink (all contain "replacement"):
    // a Vertretungs-Schicht must stay on the day of the cancelled shift it
    // covers, inside its window, and the origin must still be cancelled.
    if (err.status === 400 && detail.includes("replacement")) {
      return "Diese Vertretungs-Schicht kann nur innerhalb des Tages und Zeitfensters der ausgefallenen Schicht verschoben werden, die sie abdeckt.";
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
  // React state does not update synchronously. The ref closes the same-render
  // double-click / double-Enter window before the loading render commits.
  const moveInFlightRef = useRef(false);
  const [error, setError] = useState<string | null>(null);

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
  const canSubmit =
    targetStaffId !== "" &&
    targetDate !== "" &&
    timesValid &&
    breakValid &&
    !inactiveTypeBlocksMove;

  const targetMember = staff.find((member) => member.id === targetStaffId);
  const sourceName = `${sourceMember.firstName} ${sourceMember.lastName}`;
  const targetName = targetMember
    ? `${targetMember.firstName} ${targetMember.lastName}`
    : sourceName;
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
    if (inactiveTypeBlocksMove) {
      setError(INACTIVE_TYPE_MOVE_MESSAGE);
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
      setError(moveErrorMessage(err));
      setConfirmOpen(false);
      moveInFlightRef.current = false;
      setIsMoving(false);
    }
  };

  const executeMove = async () => {
    if (moveInFlightRef.current) return;
    moveInFlightRef.current = true;
    setIsMoving(true);
    setError(null);
    const resolvedShiftTypeId = shiftTypeId === "" ? null : shiftTypeId;
    await moveShift(resolvedShiftTypeId);
  };

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
              <Input
                type="time"
                value={startTime}
                onChange={(e) => setStartTime(e.target.value)}
                className="tabular-nums"
              />
            </Field>
            <Field label="Ende">
              <Input
                type="time"
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
                ariaLabel="Schichtart"
                placeholder="Keine Schichtart"
              />
            </Field>
          )}
          {inactiveTypeBlocksMove && (
            <Alert type="warning" message={INACTIVE_TYPE_MOVE_MESSAGE} />
          )}
          {error && <Alert type="error" message={error} />}
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
