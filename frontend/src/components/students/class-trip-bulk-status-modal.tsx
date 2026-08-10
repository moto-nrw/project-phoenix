"use client";

import { useEffect, useState } from "react";
import { format } from "date-fns";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import type { Student } from "~/lib/api";
import { formatDate as formatCalendarDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  bulkCreateStudentStatusDays,
  type StudentStatusDay,
  type StudentStatusKind,
  StudentStatusDayConflictError,
} from "~/lib/student-status-days-api";

const logger = createLogger({ component: "ClassTripBulkStatusModal" });

interface ClassTripBulkStatusModalProps {
  isOpen: boolean;
  onClose: () => void;
  targetLabel: string;
  students: Student[];
  onSuccess?: () => void;
}

function todayKey(): string {
  return format(new Date(), "yyyy-MM-dd");
}

export function ClassTripBulkStatusModal({
  isOpen,
  onClose,
  targetLabel,
  students,
  onSuccess,
}: ClassTripBulkStatusModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [from, setFrom] = useState(todayKey);
  const [to, setTo] = useState(todayKey);
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [conflicts, setConflicts] = useState<StudentStatusDay[]>([]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const today = todayKey();
    setFrom(today);
    setTo(today);
    setReason("");
    setConflicts([]);
  }, [isOpen]);

  const handleSubmit = async () => {
    if (!from || !to || to < from) {
      toastError("Bitte einen gültigen Zeitraum wählen");
      return;
    }

    setSaving(true);
    setConflicts([]);
    try {
      await bulkCreateStudentStatusDays(
        students.map((student) => student.id),
        "class_trip",
        from,
        to,
        reason.trim() || undefined,
      );
      toastSuccess(`Klassenfahrt für ${students.length} Schüler gespeichert`);
      onSuccess?.();
      onClose();
    } catch (err) {
      if (err instanceof StudentStatusDayConflictError) {
        setConflicts(err.conflicts);
        toastError(
          "Bestehende Status-Tage verhindern die Speicherung. Es wurde nichts überschrieben.",
        );
        return;
      }
      const message = err instanceof Error ? err.message : String(err);
      logger.error("failed to bulk create class trip", {
        targetLabel,
        error: message,
      });
      toastError(`Fehler beim Speichern: ${message}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`Klassenfahrt planen: ${targetLabel}`}
      size="md"
      mobilePosition="bottom"
      footer={
        <div className="flex items-center justify-end gap-2 p-4">
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Abbrechen
          </Button>
          <Button
            variant="success"
            onClick={handleSubmit}
            disabled={
              saving || students.length === 0 || !from || !to || to < from
            }
          >
            {saving
              ? "Speichern..."
              : `Für ${students.length} Schüler speichern`}
          </Button>
        </div>
      }
    >
      <div className="space-y-4 p-4">
        <div className="border-moto-blue/25 bg-moto-blue/10 flex items-start gap-3 rounded-lg border p-3 text-sm text-gray-700">
          <MotoConceptIcon concept="classTrip" size={18} className="mt-0.5" />
          <p>
            Der Status wird für alle {students.length} Schüler in{" "}
            <strong>{targetLabel}</strong> gesetzt und über Nacht nicht
            zurückgesetzt.
          </p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <label
              htmlFor="class-trip-bulk-from"
              className="text-sm font-medium text-gray-700"
            >
              Von
            </label>
            <ISODatePicker
              id="class-trip-bulk-from"
              value={from}
              onChange={setFrom}
              calendarLayout="popover"
              className="mt-1"
            />
          </div>
          <div>
            <label
              htmlFor="class-trip-bulk-to"
              className="text-sm font-medium text-gray-700"
            >
              Bis
            </label>
            <ISODatePicker
              id="class-trip-bulk-to"
              value={to}
              min={from || undefined}
              onChange={setTo}
              calendarLayout="popover"
              className="mt-1"
            />
          </div>
        </div>
        <label className="block text-sm font-medium text-gray-700">
          Hinweis (optional)
          <textarea
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={2}
            maxLength={2000}
            placeholder="z. B. Ziel oder Kontakt vor Ort"
            className="mt-1 w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-500 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
          />
        </label>
        {conflicts.length > 0 ? (
          <Alert
            type="warning"
            message={formatBulkConflictMessage(conflicts, students)}
          />
        ) : null}
      </div>
    </FormModal>
  );
}

const MAX_BULK_CONFLICT_DETAILS = 8;

function formatBulkConflictMessage(
  conflicts: StudentStatusDay[],
  students: Student[],
): string {
  const studentNameById = new Map(
    students.map((student) => [student.id, studentDisplayName(student)]),
  );
  const total = conflicts.length;
  const shown = conflicts.slice(0, MAX_BULK_CONFLICT_DETAILS);
  const details = shown
    .map((day) => {
      const studentName =
        studentNameById.get(day.student_id) ?? `Schüler ${day.student_id}`;
      return `${studentName}: ${formatCalendarDate(day.date)} (${statusKindLabel(day.status).toLowerCase()})`;
    })
    .join("; ");
  const remaining = total - shown.length;
  const more = remaining > 0 ? ` und ${remaining} weitere` : "";
  const countLabel = total === 1 ? "1 Konflikt" : `${total} Konflikte`;
  return `${countLabel} — ${details}${more}. Diese Status-Tage wurden nicht überschrieben.`;
}

function studentDisplayName(student: Student): string {
  const parts = [student.first_name, student.second_name].filter(
    (part): part is string => Boolean(part?.trim()),
  );
  if (parts.length > 0) {
    return parts.join(" ");
  }
  if (student.name?.trim()) {
    return student.name.trim();
  }
  return `Schüler ${student.id}`;
}

function statusKindLabel(status: StudentStatusKind): string {
  if (status === "sick") return "Krank";
  if (status === "class_trip") return "Klassenfahrt";
  return "Entschuldigt";
}
