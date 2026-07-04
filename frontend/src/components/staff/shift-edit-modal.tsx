"use client";

import { useEffect, useMemo, useState } from "react";

import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Modal } from "~/components/ui/modal";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { createLogger } from "~/lib/logger";
import { staffShiftService } from "~/lib/shift-api";
import type { StaffShift } from "~/lib/shift-helpers";

const logger = createLogger({ component: "ShiftEditModal" });

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
  onClose,
  onSaved,
}: ShiftEditModalProps) {
  const initial = useMemo(() => {
    if (shift) {
      return {
        startTime: shift.startTime,
        endTime: shift.endTime,
        breakMinutes: shift.breakMinutes,
      };
    }
    return { startTime: "08:00", endTime: "16:00", breakMinutes: 30 };
  }, [shift]);

  const [startTime, setStartTime] = useState(initial.startTime);
  const [endTime, setEndTime] = useState(initial.endTime);
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
    setError(null);
    setConfirmDeleteOpen(false);
  }, [isOpen, initial]);

  const timesValid = startTime !== "" && endTime !== "" && startTime < endTime;
  const breakMinutes = parseBreakMinutes(breakMinutesStr);
  const breakValid = breakMinutes !== null;

  const handleSubmit = async () => {
    setError(null);
    if (!timesValid) {
      setError("Ende muss nach Beginn liegen.");
      return;
    }
    if (!breakValid || breakMinutes === null) {
      setError("Pause muss eine ganze Zahl zwischen 0 und 300 sein.");
      return;
    }

    setIsSaving(true);
    try {
      const payload = { staffId, date, startTime, endTime, breakMinutes };
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
      setError(
        getApiErrorMessage(
          err,
          "speichern",
          "Schicht",
          "Speichern fehlgeschlagen.",
        ),
      );
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
      setError(
        getApiErrorMessage(
          err,
          "löschen",
          "Schicht",
          "Löschen fehlgeschlagen.",
        ),
      );
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
        <button
          type="button"
          onClick={() => setConfirmDeleteOpen(true)}
          disabled={isSaving || isDeleting}
          className="rounded-md border border-red-200 bg-white px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50 sm:mr-auto"
        >
          Schicht löschen
        </button>
      )}
      <button
        type="button"
        onClick={onClose}
        disabled={isSaving || isDeleting}
        className="rounded-md border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={handleSubmit}
        disabled={isSaving || isDeleting || !timesValid || !breakValid}
        className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#70b525] disabled:cursor-not-allowed disabled:opacity-50"
      >
        {isSaving
          ? "Speichern…"
          : mode === "edit"
            ? "Änderungen speichern"
            : "Schicht anlegen"}
      </button>
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
                max={300}
                inputMode="numeric"
                value={breakMinutesStr}
                onChange={(e) => setBreakMinutesStr(e.target.value)}
                className="w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:border-[#83CD2D] focus:outline-none"
              />
            </Field>
          </div>
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

function parseBreakMinutes(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  const n = Number(trimmed);
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0 || n > 300) {
    return null;
  }
  return n;
}
