"use client";

import { useEffect, useMemo, useState } from "react";

import { CustomSelect } from "~/components/ui/custom-select";
import { Modal } from "~/components/ui/modal";
import { createLogger } from "~/lib/logger";
import type { StaffHistorySession } from "~/lib/staff-api";
import { staffSessionService } from "~/lib/staff-api";

const logger = createLogger({ component: "AdminSessionEditModal" });

// Admin edit / nachtragen modal. Two modes — derived from whether a session
// is passed in:
//   - "edit"        : pre-fill from the existing session, PUT to backend
//   - "nachtragen"  : empty defaults, POST to backend
//
// Both modes require a Begründung (BAG Verlässlichkeit). Cancel is always
// allowed. The form deliberately keeps to the same field set as the MA-side
// edit modal — check-in / check-out / pause / status — so admins and MA
// share a mental model of what's editable.
export type AdminEditMode = "edit" | "nachtragen";

interface AdminSessionEditModalProps {
  readonly isOpen: boolean;
  readonly mode: AdminEditMode;
  readonly staffId: string;
  readonly date: Date;
  readonly session: StaffHistorySession | null;
  readonly onClose: () => void;
  readonly onSaved: () => void;
}

export function AdminSessionEditModal({
  isOpen,
  mode,
  staffId,
  date,
  session,
  onClose,
  onSaved,
}: AdminSessionEditModalProps) {
  // Initial values come from the session in edit mode, sensible defaults in
  // nachtragen mode (08:00–16:00, 30min Pause, vor Ort).
  const initial = useMemo(() => {
    if (session) {
      return {
        checkIn: extractTime(session.check_in_time) ?? "08:00",
        checkOut: extractTime(session.check_out_time) ?? "16:00",
        breakMinutes: session.break_minutes ?? 30,
        status: (session.status ?? "present") as "present" | "home_office",
      };
    }
    return {
      checkIn: "08:00",
      checkOut: "16:00",
      breakMinutes: 30,
      status: "present" as const,
    };
  }, [session]);

  const [checkIn, setCheckIn] = useState(initial.checkIn);
  const [checkOut, setCheckOut] = useState(initial.checkOut);
  // Pause is kept as a raw string so the user can clear the field while
  // typing ("30" → "" → "45"). A typed `number` state would snap empty
  // input back to 0 because Number("") === 0, blocking the natural delete-
  // then-retype flow. We parse on submit.
  const [breakMinutesStr, setBreakMinutesStr] = useState(
    String(initial.breakMinutes),
  );
  const [status, setStatus] = useState<"present" | "home_office">(
    initial.status,
  );
  const [notes, setNotes] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset form when the modal opens or the target row changes — otherwise
  // closing the modal and opening a different day would leak the previous
  // values.
  useEffect(() => {
    if (!isOpen) return;
    setCheckIn(initial.checkIn);
    setCheckOut(initial.checkOut);
    setBreakMinutesStr(String(initial.breakMinutes));
    setStatus(initial.status);
    setNotes("");
    setError(null);
  }, [isOpen, initial]);

  const notesValid = notes.trim().length > 0;
  const timesValid = checkIn < checkOut;
  const breakMinutes = parseBreakMinutes(breakMinutesStr);
  const breakValid = breakMinutes !== null;

  const handleSubmit = async () => {
    setError(null);
    if (!notesValid) {
      setError("Begründung ist erforderlich.");
      return;
    }
    if (!timesValid) {
      setError("Check-out muss nach Check-in liegen.");
      return;
    }
    if (!breakValid || breakMinutes === null) {
      setError("Pause muss eine ganze Zahl ≥ 0 sein.");
      return;
    }

    setIsSaving(true);
    try {
      // The backend decodes all three time fields with the same time.Time
      // unmarshaller (RFC3339). For `date` we MUST send UTC midnight rather
      // than local midnight — otherwise Berlin→UTC shifts a day earlier
      // (14.05 00:00 Berlin = 13.05 22:00 UTC) and the DATE column ends up
      // on the wrong calendar day. Check-in/out are TIMESTAMPTZ so the
      // local-time conversion is fine for those.
      const dateISO = toUTCDateISO(date);
      const checkInISO = combineDateAndTime(date, checkIn);
      const checkOutISO = combineDateAndTime(date, checkOut);
      if (mode === "edit" && session?.id != null) {
        await staffSessionService.updateSession(staffId, String(session.id), {
          date: dateISO,
          check_in_time: checkInISO,
          check_out_time: checkOutISO,
          break_minutes: breakMinutes,
          status,
          notes: notes.trim(),
        });
      } else {
        await staffSessionService.createSession(staffId, {
          date: dateISO,
          check_in_time: checkInISO,
          check_out_time: checkOutISO,
          break_minutes: breakMinutes,
          status,
          notes: notes.trim(),
        });
      }
      onSaved();
      onClose();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      logger.error("admin_session_save_failed", {
        staff_id: staffId,
        mode,
        session_id: session?.id ?? null,
        error: msg,
      });
      setError(msg || "Speichern fehlgeschlagen.");
    } finally {
      setIsSaving(false);
    }
  };

  const title =
    mode === "edit"
      ? `Eintrag bearbeiten · ${formatLongDate(date)}`
      : `Eintrag nachtragen · ${formatLongDate(date)}`;

  const footer = (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <button
        type="button"
        onClick={onClose}
        disabled={isSaving}
        className="rounded-md border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={handleSubmit}
        disabled={isSaving || !notesValid || !timesValid || !breakValid}
        className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#70b525] disabled:cursor-not-allowed disabled:opacity-50"
      >
        {isSaving
          ? "Speichern…"
          : mode === "edit"
            ? "Änderungen speichern"
            : "Eintrag anlegen"}
      </button>
    </div>
  );

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} footer={footer}>
      <div className="space-y-4 text-sm">
        {mode === "edit" && session?.auto_checked_out && (
          <p className="rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800">
            Diese Sitzung wurde automatisch zum geplanten Dienstende
            ausgestempelt. Bitte die tatsächliche Endzeit prüfen.
          </p>
        )}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field label="Check-in">
            <input
              type="time"
              value={checkIn}
              onChange={(e) => setCheckIn(e.target.value)}
              className="w-full rounded-md border border-gray-200 px-3 py-2 tabular-nums focus:border-[#83CD2D] focus:outline-none"
            />
          </Field>
          <Field label="Check-out">
            <input
              type="time"
              value={checkOut}
              onChange={(e) => setCheckOut(e.target.value)}
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
          <Field label="Status">
            <CustomSelect
              ariaLabel="Status"
              value={status}
              onChange={(next) => setStatus(next as "present" | "home_office")}
              options={[
                { value: "present", label: "Vor Ort" },
                { value: "home_office", label: "Homeoffice" },
              ]}
            />
          </Field>
        </div>
        <Field label="Begründung *">
          <textarea
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            rows={3}
            placeholder="z. B. Mitarbeiter hatte vergessen einzustempeln"
            className="w-full rounded-md border border-gray-200 px-3 py-2 focus:border-[#83CD2D] focus:outline-none"
          />
        </Field>
        {error && (
          <p className="rounded-md bg-red-50 px-3 py-2 text-xs text-red-700">
            {error}
          </p>
        )}
      </div>
    </Modal>
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

function formatLongDate(date: Date): string {
  return date.toLocaleDateString("de-DE", {
    weekday: "long",
    day: "2-digit",
    month: "long",
    year: "numeric",
  });
}

function extractTime(iso: string | null): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

// Empty string is intentionally rejected (the field is required); a string
// that doesn't parse as a non-negative integer is also rejected. Returning
// null lets the caller surface a friendly inline error instead of letting
// NaN silently leak into the request body.
function parseBreakMinutes(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  const n = Number(trimmed);
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0 || n > 300) {
    return null;
  }
  return n;
}

// Returns "YYYY-MM-DDT00:00:00.000Z" for the given calendar day. Uses the
// date's local Y/M/D components so a Date created in Berlin still serialises
// to the same Y/M/D — we don't want the rendered day to drift across UTC.
function toUTCDateISO(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}T00:00:00.000Z`;
}

// Build an RFC3339 timestamp for the given calendar day + HH:MM (local time).
// We deliberately use local time so the admin types "08:00" and the backend
// receives a wall-clock 08:00, not 09:00 because of timezone drift.
function combineDateAndTime(date: Date, time: string): string {
  const [hh, mm] = time.split(":").map(Number);
  const result = new Date(date);
  result.setHours(hh ?? 0, mm ?? 0, 0, 0);
  return result.toISOString();
}
