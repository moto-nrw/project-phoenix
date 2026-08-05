"use client";

import { useEffect, useMemo, useState } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "~/components/ui/button";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import type { Student } from "~/lib/api";
import { createLogger } from "~/lib/logger";
import {
  type ArrivalScheduleInput,
  WEEKDAYS,
  bulkUpsertArrivalByClass,
  fetchArrivalData,
} from "~/lib/student-arrival-api";
import { cn } from "~/lib/utils";

const logger = createLogger({ component: "ClassBulkArrivalModal" });

interface ClassBulkArrivalModalProps {
  isOpen: boolean;
  onClose: () => void;
  schoolClass: string;
  studentsInClass: Student[];
  onSuccess?: () => void;
}

type DraftState = Record<number, string>;

function childCountLabel(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

function initialDraft(): DraftState {
  const draft: DraftState = {};
  for (const day of WEEKDAYS) {
    draft[day.value] = "";
  }
  return draft;
}

function isValidTime(value: string): boolean {
  if (value === "") return true;
  return /^\d{2}:\d{2}$/.test(value);
}

export function ClassBulkArrivalModal({
  isOpen,
  onClose,
  schoolClass,
  studentsInClass,
  onSuccess,
}: ClassBulkArrivalModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [draft, setDraft] = useState<DraftState>(initialDraft);
  const [saving, setSaving] = useState(false);
  const [collisionCount, setCollisionCount] = useState<number | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setDraft(initialDraft());
    setCollisionCount(null);

    let cancelled = false;
    const ids = studentsInClass.map((student) => String(student.id));
    Promise.all(
      ids.map((id) =>
        fetchArrivalData(id)
          .then((data) => data.schedules.length > 0)
          .catch((err) => {
            logger.warn("arrival_status_fetch_failed", {
              student_id: id,
              error: err instanceof Error ? err.message : String(err),
            });
            return false;
          }),
      ),
    ).then((results) => {
      if (cancelled) return;
      setCollisionCount(results.filter(Boolean).length);
    });

    return () => {
      cancelled = true;
    };
  }, [isOpen, studentsInClass]);

  const hasAnyTime = useMemo(
    () => Object.values(draft).some((value) => value.trim() !== ""),
    [draft],
  );
  const hasInvalidEntry = useMemo(
    () => Object.values(draft).some((value) => !isValidTime(value)),
    [draft],
  );

  const handleSubmit = async () => {
    if (!hasAnyTime) {
      toastError("Mindestens eine Zeit angeben");
      return;
    }
    if (hasInvalidEntry) {
      toastError("Ungueltige Uhrzeit. Format HH:MM.");
      return;
    }

    const schedules: ArrivalScheduleInput[] = WEEKDAYS.filter(
      ({ value }) => (draft[value] ?? "").trim() !== "",
    ).map(({ value }) => ({
      weekday: value,
      expected_arrival: draft[value] ?? "",
    }));

    setSaving(true);
    try {
      await bulkUpsertArrivalByClass(schoolClass, schedules);
      toastSuccess(
        `Ankunftszeiten für Klasse ${schoolClass} gesetzt (${childCountLabel(studentsInClass.length)})`,
      );
      onSuccess?.();
      onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("Failed to bulk upsert arrival", {
        schoolClass,
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
      title={`Ankunftszeiten für Klasse ${schoolClass}`}
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
            disabled={saving || !hasAnyTime || hasInvalidEntry}
          >
            {saving
              ? "Speichern..."
              : `Für ${childCountLabel(studentsInClass.length)} setzen`}
          </Button>
        </div>
      }
    >
      <div className="space-y-4 p-4">
        <p className="text-sm text-gray-600">
          Setzt die Ankunftszeit für alle{" "}
          {childCountLabel(studentsInClass.length)} der Klasse {schoolClass}.
          Leere Felder bleiben unverändert.
        </p>
        {collisionCount !== null && collisionCount > 0 ? (
          <div className="border-moto-orange bg-moto-orange/5 flex items-start gap-2 rounded-lg border p-3 text-sm">
            <AlertTriangle
              className="text-moto-orange mt-0.5 h-4 w-4 shrink-0"
              aria-hidden
            />
            <div>
              <div className="text-moto-orange font-semibold">
                {childCountLabel(collisionCount)}{" "}
                {collisionCount === 1 ? "hat" : "haben"} bereits Ankunftszeiten
              </div>
              <div className="text-moto-orange-strong">
                Existierende Zeiten werden an den gesetzten Tagen überschrieben.
              </div>
            </div>
          </div>
        ) : null}
        <div className="space-y-2">
          {WEEKDAYS.map((day) => {
            const value = draft[day.value] ?? "";
            const invalid = !isValidTime(value);
            return (
              <div
                key={day.value}
                className={cn(
                  "flex items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-2.5",
                  invalid && "border-red-300 bg-red-50",
                )}
              >
                <label
                  htmlFor={`bulk-arrival-${day.value}`}
                  className="w-28 text-sm font-medium text-gray-700"
                >
                  {day.label}
                </label>
                <input
                  id={`bulk-arrival-${day.value}`}
                  type="time"
                  value={value}
                  onChange={(event) =>
                    setDraft((prev) => ({
                      ...prev,
                      [day.value]: event.target.value.slice(0, 5),
                    }))
                  }
                  className="focus:border-moto-green focus:ring-moto-green/30 w-32 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:ring-2 focus:outline-none"
                />
                {invalid ? (
                  <span className="text-xs text-red-600">Format HH:MM</span>
                ) : value === "" ? (
                  <span className="text-xs text-gray-400 italic">
                    nicht ändern
                  </span>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    </FormModal>
  );
}
