"use client";

import { useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { Modal } from "~/components/ui/modal";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { createLogger } from "~/lib/logger";
import { ShiftApiError, staffShiftService } from "~/lib/shift-api";
import type { StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

const logger = createLogger({ component: "ShiftEditModal" });
const STAFF_SHIFT_MAX_BREAK_MINUTES = 300;

function getShiftMutationErrorMessage(
  err: unknown,
  action: "speichern" | "löschen",
): string {
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
      return `Sie haben keine Berechtigung, diese Schicht zu ${action}.`;
    }
    return err.detail || `Schicht konnte nicht ${action} werden.`;
  }

  return getApiErrorMessage(
    err,
    action,
    "Schicht",
    action === "speichern"
      ? "Speichern fehlgeschlagen."
      : "Löschen fehlgeschlagen.",
  );
}

// Create/edit modal for one planned shift (Dienstplan). Times are plain
// "HH:MM" strings end-to-end — no Date/ISO conversion, which sidesteps the
// timezone pitfalls of the session edit modal.
export type ShiftEditMode = "create" | "edit";

interface ShiftEditModalProps {
  readonly isOpen: boolean;
  readonly mode: ShiftEditMode;
  readonly staffId: string;
  readonly staffName: string;
  /** Calendar day as "YYYY-MM-DD" */
  readonly date: string;
  readonly shift: StaffShift | null;
  /** All shift types (Schichtarten); the picker offers active ones plus the
   *  one currently attached to this shift (even if inactive). */
  readonly shiftTypes: readonly ShiftType[];
  readonly onClose: () => void;
  readonly onSaved: () => void;
}

export function ShiftEditModal({
  isOpen,
  mode,
  staffId,
  staffName,
  date,
  shift,
  shiftTypes,
  onClose,
  onSaved,
}: ShiftEditModalProps) {
  const initial = useMemo(() => {
    if (shift) {
      return {
        startTime: shift.startTime,
        endTime: shift.endTime,
        breakMinutes: shift.breakMinutes,
        shiftTypeId: shift.shiftTypeId ?? "",
      };
    }
    return {
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: "",
    };
  }, [shift]);

  const [startTime, setStartTime] = useState(initial.startTime);
  const [endTime, setEndTime] = useState(initial.endTime);
  // "" = no shift type. An inactive type still attached to this shift is kept
  // as an option so editing a shift doesn't silently drop it.
  const [shiftTypeId, setShiftTypeId] = useState(initial.shiftTypeId);
  // Raw string so the field can be cleared while typing; parsed on submit
  // (same rationale as AdminSessionEditModal).
  const [breakMinutesStr, setBreakMinutesStr] = useState(
    String(initial.breakMinutes),
  );
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setStartTime(initial.startTime);
    setEndTime(initial.endTime);
    setBreakMinutesStr(String(initial.breakMinutes));
    setShiftTypeId(initial.shiftTypeId);
    setError(null);
    setConfirmDeleteOpen(false);
  }, [isOpen, initial]);

  const timesValid = startTime !== "" && endTime !== "" && startTime < endTime;
  const breakMaxMinutes = timesValid
    ? Math.min(
        STAFF_SHIFT_MAX_BREAK_MINUTES,
        shiftDurationMinutes(startTime, endTime) ??
          STAFF_SHIFT_MAX_BREAK_MINUTES,
      )
    : STAFF_SHIFT_MAX_BREAK_MINUTES;
  const breakMinutes = parseBreakMinutes(breakMinutesStr, breakMaxMinutes);
  const breakValid = breakMinutes !== null;

  const typeOptions = useMemo(() => {
    const opts: { value: string; label: string }[] = [
      { value: "", label: "Keine Schichtart" },
    ];
    for (const type of shiftTypes) {
      if (type.isActive || type.id === shift?.shiftTypeId) {
        opts.push({
          value: type.id,
          label: type.isActive ? type.name : `${type.name} (inaktiv)`,
        });
      }
    }
    return opts;
  }, [shiftTypes, shift]);

  const handleSubmit = async () => {
    setError(null);
    if (!timesValid) {
      setError("Ende muss nach Beginn liegen.");
      return;
    }
    if (!breakValid || breakMinutes === null) {
      setError(
        `Pause muss eine ganze Zahl zwischen 0 und ${breakMaxMinutes} sein.`,
      );
      return;
    }

    setIsSaving(true);
    try {
      const payload = {
        staffId,
        date,
        startTime,
        endTime,
        breakMinutes,
        shiftTypeId: shiftTypeId === "" ? null : shiftTypeId,
      };
      if (mode === "edit" && shift) {
        await staffShiftService.updateShift(shift.id, payload);
      } else {
        await staffShiftService.createShift(payload);
      }
      onSaved();
      onClose();
    } catch (err: unknown) {
      logger.error("shift_save_failed", {
        staff_id: staffId,
        date,
        mode,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "speichern"));
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!shift) return;
    setIsDeleting(true);
    try {
      await staffShiftService.deleteShift(shift.id);
      setConfirmDeleteOpen(false);
      onSaved();
      onClose();
    } catch (err: unknown) {
      logger.error("shift_delete_failed", {
        shift_id: shift.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(getShiftMutationErrorMessage(err, "löschen"));
      setConfirmDeleteOpen(false);
    } finally {
      setIsDeleting(false);
    }
  };

  const title =
    mode === "edit"
      ? `Schicht bearbeiten · ${formatLongDate(date)}`
      : `Schicht anlegen · ${formatLongDate(date)}`;

  const footer = (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center">
      {mode === "edit" && shift && (
        <Button
          type="button"
          variant="outline_danger"
          size="md"
          className="sm:mr-auto"
          onClick={() => setConfirmDeleteOpen(true)}
          disabled={isSaving || isDeleting}
        >
          Schicht löschen
        </Button>
      )}
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={isSaving || isDeleting}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={handleSubmit}
        isLoading={isSaving}
        loadingText="Speichern…"
        disabled={isSaving || isDeleting || !timesValid || !breakValid}
      >
        {mode === "edit" ? "Änderungen speichern" : "Schicht anlegen"}
      </Button>
    </div>
  );

  return (
    <>
      {/* Hidden while the delete confirmation is open — both use the same
          fixed z-index, so stacking them puts the confirm dialog's buttons
          behind this modal's body. */}
      <Modal
        isOpen={isOpen && !confirmDeleteOpen}
        onClose={onClose}
        title={title}
        footer={footer}
      >
        <div className="space-y-4 text-sm">
          <p className="text-sm text-gray-600">{staffName}</p>
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
                max={breakMaxMinutes}
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
          {error && (
            <p className="rounded-md bg-red-50 px-3 py-2 text-xs text-red-700">
              {error}
            </p>
          )}
        </div>
      </Modal>
      <ConfirmDeleteModal
        isOpen={confirmDeleteOpen}
        title="Schicht löschen"
        description={
          <>
            Die geplante Schicht am <strong>{formatLongDate(date)}</strong> für{" "}
            <strong>{staffName}</strong> wird gelöscht.
          </>
        }
        gate={{ mode: "twoStep" }}
        loading={isDeleting}
        error=""
        onConfirm={handleDelete}
        onClose={() => setConfirmDeleteOpen(false)}
      />
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

function formatLongDate(isoDate: string): string {
  const [y, m, d] = isoDate.split("-").map(Number);
  const date = new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1);
  return date.toLocaleDateString("de-DE", {
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
