"use client";

import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { FieldLabel } from "~/components/ui/field-label";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { isWeekendISO } from "~/components/class-day/routes";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { MultiSelect } from "~/components/ui/multi-select";
import { TimeField } from "~/components/ui/time-field";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import type { Student } from "~/lib/student-helpers";
import {
  formatDate as formatCalendarDate,
  berlinTodayISO,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  classTripConflictsFromError,
  type ClassTripConflictDay,
  type ClassTripReportInput,
} from "~/lib/school-class-day-api";
import {
  bulkCreateStudentStatusDays,
  type StudentStatusKind,
  StudentStatusDayConflictError,
} from "~/lib/student-status-days-api";
import { classTripConcept } from "~/lib/concepts/classTrip";
import { LOCATION_COLORS } from "~/lib/location-helper";

const logger = createLogger({ component: "ClassTripBulkStatusModal" });

/** Ein Kind der Klassenfahrt: nur, was die Maske anzeigt und speichert. */
type ClassTripStudent = Pick<
  Student,
  "id" | "name" | "first_name" | "second_name"
>;

interface ClassTripBulkStatusModalProps {
  isOpen: boolean;
  onClose: () => void;
  targetLabel: string;
  students: ClassTripStudent[];
  onSuccess?: () => void;
  /**
   * moto schule (#4799): Die Lehrkraft wählt die Kinder (vorausgewählt die
   * ganze Klasse) und kann bei einem Tag eine Rückkehrzeit angeben.
   * Gespeichert wird über `save`; die Rückmeldung zeigt der Aufrufer.
   */
  schoolReport?: {
    save: (input: ClassTripReportInput) => Promise<void>;
    /** Der angezeigte Tag ("YYYY-MM-DD"); ein vergangener Tag wird heute. */
    defaultDate?: string;
  };
}

function todayKey(): string {
  return berlinTodayISO();
}

export function ClassTripBulkStatusModal({
  isOpen,
  onClose,
  targetLabel,
  students,
  onSuccess,
  schoolReport,
}: ClassTripBulkStatusModalProps) {
  const { success: toastSuccess } = useToast();
  const [from, setFrom] = useState(todayKey);
  const [to, setTo] = useState(todayKey);
  const [reason, setReason] = useState("");
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [returnTime, setReturnTime] = useState("");
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const [conflicts, setConflicts] = useState<ClassTripConflictDay[]>([]);
  const [conflictTotal, setConflictTotal] = useState(0);
  // Validation and save errors of the form (Bauart 2 Regel 5): shown in the
  // FormModal error slot, not as a toast.
  const formErrors = useApiFormError();
  const clearFormErrors = formErrors.clear;
  // „Wiederholen“ speichert die aktuelle Eingabe, nicht die vom Fehler.
  const latestSubmitRef = useRef<() => Promise<void>>(async () => undefined);
  // Die Vorauswahl gilt beim Öffnen; ein Nachladen der Klasse setzt die
  // Auswahl der Lehrkraft nicht zurück.
  const studentIdsRef = useRef<string[]>([]);
  const schoolReportDefaultRef = useRef<string | undefined>(undefined);
  const studentsFieldId = useId();
  useLayoutEffect(() => {
    studentIdsRef.current = students.map((student) => student.id);
    schoolReportDefaultRef.current = schoolReport?.defaultDate;
  });

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const today = todayKey();
    const shown = schoolReportDefaultRef.current;
    const start = shown && shown > today ? shown : today;
    setFrom(start);
    setTo(start);
    setReason("");
    setReturnTime("");
    setSelectedIds(studentIdsRef.current);
    setConflicts([]);
    setConflictTotal(0);
    clearFormErrors();
  }, [isOpen, clearFormErrors]);

  const singleDay = Boolean(from) && from === to;
  // Eine Rückkehrzeit gibt es nur für einen Tag von Montag bis Freitag.
  const returnTimeOffered = singleDay && !isWeekendISO(from);
  const chosenIds = schoolReport
    ? selectedIds
    : students.map((student) => student.id);

  const handleSubmit = async () => {
    formErrors.clear();
    if (!from || !to || to < from) {
      formErrors.invalid("Bitte wählen Sie einen gültigen Zeitraum.");
      return;
    }

    const trimmedReturnTime = returnTimeOffered ? returnTime.trim() : "";
    if (
      schoolReport &&
      trimmedReturnTime &&
      !/^\d{2}:\d{2}$/.test(trimmedReturnTime)
    ) {
      formErrors.invalid("Bitte geben Sie die Rückkehrzeit wie 13:30 ein.");
      return;
    }

    await save.run(async (op) => {
      setConflicts([]);
      setConflictTotal(0);
      if (schoolReport) {
        try {
          await schoolReport.save({
            studentIds: chosenIds,
            from,
            to,
            reason: reason.trim() || undefined,
            returnTime: trimmedReturnTime || undefined,
          });
          onSuccess?.();
          if (op.isCurrent()) onClose();
        } catch (err) {
          if (!op.isCurrent()) return;
          const blocked = classTripConflictsFromError(err);
          if (blocked) {
            // Wie bei der OGS stehen die betroffenen Kinder und Tage unter
            // dem Formular; der Satz oben kommt aus dem Katalog.
            setConflicts(blocked.conflicts);
            setConflictTotal(blocked.total);
          } else {
            logger.error("failed to report class trip", {
              error: err instanceof Error ? err.message : String(err),
            });
          }
          await formErrors.show(err, {
            object: "die Meldung",
            retry: () => void latestSubmitRef.current(),
          });
        }
        return;
      }
      try {
        await bulkCreateStudentStatusDays(
          students.map((student) => student.id),
          "class_trip",
          from,
          to,
          reason.trim() || undefined,
        );
        toastSuccess(
          `Die Klassenfahrt für ${students.length} Schüler ist gespeichert.`,
        );
        onSuccess?.();
        if (op.isCurrent()) onClose();
      } catch (err) {
        if (!op.isCurrent()) return;
        if (err instanceof StudentStatusDayConflictError) {
          // Die Liste der betroffenen Tage kommt strukturiert mit und steht
          // unter dem Formular; der Satz oben kommt aus dem Katalog.
          setConflicts(err.conflicts);
          setConflictTotal(err.totalCount);
        } else {
          logger.error("failed to bulk create class trip", {
            targetLabel,
            error: err instanceof Error ? err.message : String(err),
          });
        }
        await formErrors.show(err, {
          object: "die Klassenfahrt",
          retry: () => void latestSubmitRef.current(),
        });
      }
    });
  };
  useLayoutEffect(() => {
    latestSubmitRef.current = handleSubmit;
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      error={formErrors.error}
      title={
        schoolReport
          ? `Ausflug oder Klassenfahrt melden: ${targetLabel}`
          : `Klassenfahrt planen: ${targetLabel}`
      }
      size="md"
      mobilePosition="bottom"
      footer={
        <div className="flex items-center justify-end gap-2 p-4">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="success"
            size="md"
            onClick={handleSubmit}
            disabled={
              saving || chosenIds.length === 0 || !from || !to || to < from
            }
          >
            {saving
              ? "Speichern..."
              : schoolReport
                ? `Für ${chosenIds.length} ${chosenIds.length === 1 ? "Kind" : "Kinder"} melden`
                : `Für ${students.length} Schüler speichern`}
          </Button>
        </div>
      }
    >
      <div className="space-y-4 p-4">
        <Alert
          type="info"
          announce="off"
          color={LOCATION_COLORS.CLASS_TRIP}
          icon={<MotoConceptIcon concept={classTripConcept} size={18} />}
          message={
            schoolReport ? (
              "Die OGS sieht die ausgewählten Kinder an diesen Tagen als unterwegs."
            ) : (
              <>
                Der Status wird für alle {students.length} Schüler in{" "}
                <strong>{targetLabel}</strong> gesetzt und über Nacht nicht
                zurückgesetzt.
              </>
            )
          }
        />
        <div className="grid gap-3 sm:grid-cols-2">
          <ISODatePicker
            label="Von"
            id="class-trip-bulk-from"
            value={from}
            onChange={setFrom}
            calendarLayout="popover"
          />
          <ISODatePicker
            label="Bis"
            id="class-trip-bulk-to"
            value={to}
            min={from || undefined}
            onChange={setTo}
            calendarLayout="popover"
          />
        </div>
        {schoolReport && returnTimeOffered ? (
          <TimeField
            label="Rückkehrzeit (optional)"
            hint="Kommt die Klasse am selben Tag zurück? Dann kommen die Kinder um diese Uhrzeit in die OGS."
            placeholder="13:30"
            value={returnTime}
            onChange={setReturnTime}
          />
        ) : null}
        {schoolReport ? (
          <div>
            <FieldLabel htmlFor={studentsFieldId}>Kinder</FieldLabel>
            <MultiSelect
              id={studentsFieldId}
              ariaLabel="Kinder"
              value={selectedIds}
              options={students.map((student) => ({
                value: student.id,
                label: studentDisplayName(student),
              }))}
              onChange={setSelectedIds}
              placeholder="Kein Kind ausgewählt"
              unavailableLabel="Keine Optionen verfügbar"
              summaryLabel={(count) =>
                count === students.length
                  ? `Alle ${count} Kinder`
                  : `${count} von ${students.length} Kindern`
              }
              searchable
              selectAll
              searchPlaceholder="Kind suchen"
            />
          </div>
        ) : null}
        <Textarea
          label={schoolReport ? "Ziel (optional)" : "Hinweis (optional)"}
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          rows={2}
          maxLength={schoolReport ? 200 : 2000}
          placeholder={
            schoolReport ? "z. B. Zoo" : "z. B. Ziel oder Kontakt vor Ort"
          }
        />
        {conflicts.length > 0 ? (
          <Alert
            type="warning"
            message={formatBulkConflictMessage(
              conflicts,
              students,
              conflictTotal,
            )}
          />
        ) : null}
      </div>
    </FormModal>
  );
}

const MAX_BULK_CONFLICT_DETAILS = 8;

function formatBulkConflictMessage(
  conflicts: ClassTripConflictDay[],
  students: ClassTripStudent[],
  totalCount: number,
): string {
  const studentNameById = new Map(
    students.map((student) => [student.id, studentDisplayName(student)]),
  );
  const total = Math.max(totalCount, conflicts.length);
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

function studentDisplayName(student: ClassTripStudent): string {
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
