"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FieldLabel } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  berlinClockFromISO,
  berlinDateTimeISO,
  berlinWallClockProblem,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  STORED_END_NOT_CLEARABLE_HINT,
  TIME_REQUIRED_HINT,
} from "~/lib/time-tracking-helpers";
import type { StaffHistorySession } from "~/lib/staff-api";
import { staffSessionService } from "~/lib/staff-api";
import { FieldErrorHint } from "~/components/ui/field-error-hint";

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
  const formErrors = useApiFormError();
  const clearErrors = formErrors.clear;

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
    clearErrors();
  }, [isOpen, initial, clearErrors]);

  // A time is sent only when it differs from the stored one, so saving a
  // reason does not rewrite the stamps or cut off their seconds (#6293). A
  // default shown for a missing value counts as a change: it is saved as seen.
  const storedCheckIn = extractTime(session?.check_in_time ?? null);
  const storedCheckOut = extractTime(session?.check_out_time ?? null);
  const checkInChanged = mode !== "edit" || checkIn !== storedCheckIn;
  const checkOutChanged = mode !== "edit" || checkOut !== storedCheckOut;
  // Every reason the save stays blocked stands at its field (#6293).
  const checkInProblem = !checkIn
    ? TIME_REQUIRED_HINT
    : checkInChanged
      ? berlinWallClockProblem(date, checkIn)
      : null;
  const checkOutMissing = storedCheckOut
    ? STORED_END_NOT_CLEARABLE_HINT
    : TIME_REQUIRED_HINT;
  const checkOutProblem = !checkOut
    ? checkOutMissing
    : ((checkOutChanged ? berlinWallClockProblem(date, checkOut) : null) ??
      (checkIn && checkIn >= checkOut
        ? "Das Ende muss nach dem Beginn liegen."
        : null));

  const notesValid = notes.trim().length > 0;
  const timesValid = checkInProblem === null && checkOutProblem === null;
  const breakMinutes = parseBreakMinutes(breakMinutesStr);
  const breakValid = breakMinutes !== null;

  const handleSubmit = async () => {
    formErrors.clear();
    if (!notesValid) {
      formErrors.invalid("Bitte geben Sie eine Begründung ein.", {
        notes: "Bitte geben Sie eine Begründung ein.",
      });
      return;
    }
    if (!timesValid) {
      formErrors.invalid("Bitte prüfen Sie die Uhrzeiten.", {
        ...(checkInProblem ? { check_in_time: checkInProblem } : {}),
        ...(checkOutProblem ? { check_out_time: checkOutProblem } : {}),
      });
      return;
    }
    if (!breakValid || breakMinutes === null) {
      formErrors.invalid("Bitte prüfen Sie die Pause.", {
        break_minutes: "Bitte geben Sie volle Minuten von 0 bis 300 ein.",
      });
      return;
    }

    setIsSaving(true);
    try {
      // Check-in/out are school times on this day: Berlin wall clock with
      // that day's offset, whatever the browser's zone (#6293). An unchanged
      // stored time is not converted, so a stamp inside the repeated hour of
      // the autumn switch still saves.
      if (mode === "edit" && session?.id != null) {
        // The update ignores `date`; a block keeps its day.
        await staffSessionService.updateSession(staffId, session.id, {
          check_in_time: checkInChanged
            ? berlinDateTimeISO(date, checkIn)
            : undefined,
          check_out_time: checkOutChanged
            ? berlinDateTimeISO(date, checkOut)
            : undefined,
          break_minutes: breakMinutes,
          status,
          notes: notes.trim(),
        });
      } else {
        // The backend decodes `date` as an RFC3339 time and takes its
        // calendar day; UTC midnight keeps that day from shifting (14.05
        // 00:00 Berlin would be 13.05 22:00 UTC).
        await staffSessionService.createSession(staffId, {
          date: toUTCDateISO(date),
          check_in_time: berlinDateTimeISO(date, checkIn),
          check_out_time: berlinDateTimeISO(date, checkOut),
          break_minutes: breakMinutes,
          status,
          notes: notes.trim(),
        });
      }
      onSaved();
      onClose();
    } catch (err: unknown) {
      logger.error("admin_session_save_failed", {
        staff_id: staffId,
        mode,
        session_id: session?.id ?? null,
        error: err instanceof Error ? err.message : String(err),
      });
      // The overlap 409 carries workforce.work_session_overlap (#2402); the
      // catalog explains it, the raw backend text never reaches the screen.
      await formErrors.show(err, {
        object: "die Arbeitszeit",
        retry: () => void latestSubmit.current(),
      });
    } finally {
      setIsSaving(false);
    }
  };
  // „Wiederholen“ sendet den aktuellen Entwurf.
  const latestSubmit = useRef(handleSubmit);
  useLayoutEffect(() => {
    latestSubmit.current = handleSubmit;
  });

  const title =
    mode === "edit"
      ? `Eintrag bearbeiten · ${formatLongDate(date)}`
      : `Eintrag nachtragen · ${formatLongDate(date)}`;

  const footer = (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={isSaving}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="success"
        size="md"
        onClick={handleSubmit}
        disabled={isSaving || !notesValid || !timesValid || !breakValid}
      >
        {isSaving
          ? "Speichern…"
          : mode === "edit"
            ? "Änderungen speichern"
            : "Eintrag anlegen"}
      </Button>
    </div>
  );

  return (
    <Modal isOpen={isOpen} onClose={onClose} title={title} footer={footer}>
      <div className="space-y-4 text-sm">
        <FormErrorAlert message={formErrors.error} />
        {mode === "edit" && session?.auto_checked_out && (
          <Alert
            type="warning"
            size="compact"
            announce="off"
            message="Diese Sitzung wurde automatisch zum geplanten Dienstende ausgestempelt. Bitte die tatsächliche Endzeit prüfen."
          />
        )}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field
            label="Check-in"
            error={checkInProblem ?? formErrors.fieldError("check_in_time")}
          >
            <Input
              controlSize="compact"
              type="time"
              name="check_in_time"
              aria-invalid={
                (checkInProblem ?? formErrors.fieldError("check_in_time"))
                  ? true
                  : undefined
              }
              aria-describedby={describedBy(
                checkInProblem ?? formErrors.fieldError("check_in_time"),
                "check_in_time",
              )}
              value={checkIn}
              onChange={(e) => setCheckIn(e.target.value)}
              className="tabular-nums"
            />
          </Field>
          <Field
            label="Check-out"
            error={checkOutProblem ?? formErrors.fieldError("check_out_time")}
          >
            <Input
              controlSize="compact"
              type="time"
              name="check_out_time"
              aria-invalid={
                (checkOutProblem ?? formErrors.fieldError("check_out_time"))
                  ? true
                  : undefined
              }
              aria-describedby={describedBy(
                checkOutProblem ?? formErrors.fieldError("check_out_time"),
                "check_out_time",
              )}
              value={checkOut}
              onChange={(e) => setCheckOut(e.target.value)}
              className="tabular-nums"
            />
          </Field>
        </div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field
            label="Pause (Minuten)"
            error={formErrors.fieldError("break_minutes")}
          >
            <Input
              controlSize="compact"
              type="number"
              name="break_minutes"
              aria-invalid={
                formErrors.fieldError("break_minutes") ? true : undefined
              }
              aria-describedby={describedBy(
                formErrors.fieldError("break_minutes"),
                "break_minutes",
              )}
              min={0}
              max={300}
              inputMode="numeric"
              value={breakMinutesStr}
              onChange={(e) => setBreakMinutesStr(e.target.value)}
              className="tabular-nums"
            />
          </Field>
          <Field label="Status">
            <CustomSelect
              ariaLabel="Status"
              value={status}
              onChange={(next) => setStatus(next as "present" | "home_office")}
              options={[
                { value: "present", label: "In der OGS" },
                { value: "home_office", label: "Unterwegs" },
              ]}
            />
          </Field>
        </div>
        <Field
          label="Begründung"
          required
          error={formErrors.fieldError("notes")}
        >
          <Textarea
            name="notes"
            aria-required
            aria-invalid={formErrors.fieldError("notes") ? true : undefined}
            aria-describedby={describedBy(
              formErrors.fieldError("notes"),
              "notes",
            )}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            rows={3}
            placeholder="z. B. Mitarbeiter hatte vergessen einzustempeln"
          />
        </Field>
      </div>
    </Modal>
  );
}

function describedBy(error: string | undefined, name: string) {
  return error ? `admin-session-${name}-error` : undefined;
}

function Field({
  label,
  required = false,
  error,
  children,
}: {
  readonly label: string;
  readonly required?: boolean;
  readonly error?: string;
  readonly children: React.ReactElement<{ name?: string }>;
}) {
  return (
    <label className="block">
      <FieldLabel required={required} invalid={Boolean(error)}>
        {label}
      </FieldLabel>
      {children}
      <FieldErrorHint
        id={`admin-session-${children.props.name ?? ""}-error`}
        message={error}
        live
        as="span"
      />
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

// The stored instant as the Berlin wall clock the field shows (#6293).
function extractTime(iso: string | null): string | null {
  if (!iso) return null;
  return berlinClockFromISO(iso) || null;
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
