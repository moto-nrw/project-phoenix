"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import { createLogger } from "~/lib/logger";
import {
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
} from "~/lib/arrival-schedule-helpers";

const logger = createLogger({ component: "ArrivalScheduleForm" });

interface ArrivalScheduleFormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSubmit: (schedules: ArrivalScheduleFormEntry[]) => Promise<void>;
  readonly initialSchedules: ArrivalScheduleFormEntry[];
}

export function ArrivalScheduleFormModal({
  isOpen,
  onClose,
  onSubmit,
  initialSchedules,
}: ArrivalScheduleFormModalProps) {
  const [schedules, setSchedules] =
    useState<ArrivalScheduleFormEntry[]>(initialSchedules);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      setSchedules(initialSchedules);
      setError(null);
    }
  }, [isOpen, initialSchedules]);

  const handleTimeChange = (weekday: number, time: string) => {
    setSchedules((prev) =>
      prev.map((s) =>
        s.weekday === weekday ? { ...s, expected_arrival: time } : s,
      ),
    );
  };

  const handleNotesChange = (weekday: number, notes: string) => {
    setSchedules((prev) =>
      prev.map((s) =>
        s.weekday === weekday ? { ...s, notes: notes || null } : s,
      ),
    );
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    const valid = schedules.filter((s) => s.expected_arrival.trim() !== "");
    const timeRegex = /^([01]?\d|2[0-3]):[0-5]\d$/;
    for (const entry of valid) {
      if (!timeRegex.test(entry.expected_arrival)) {
        setError(
          `Ungültiges Zeitformat für ${WEEKDAYS.find((w) => w.value === entry.weekday)?.label}. Bitte HH:MM.`,
        );
        return;
      }
    }

    setIsSubmitting(true);
    try {
      await onSubmit(valid);
    } catch (err) {
      logger.error("arrival_schedule_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Speichern des Ankunftsplans",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        className="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
        disabled={isSubmitting}
      >
        Abbrechen
      </button>
      <button
        type="submit"
        form="arrival-schedule-form"
        disabled={isSubmitting}
        className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700 disabled:opacity-50"
      >
        {isSubmitting && <Loader2 className="h-4 w-4 animate-spin" />}
        Speichern
      </button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Wöchentlichen Ankunftsplan bearbeiten"
      footer={footer}
      size="md"
      mobilePosition="center"
    >
      <form id="arrival-schedule-form" onSubmit={handleSubmit}>
        <p className="mb-4 text-sm text-gray-500">
          Zeiten und Notizen gelten wöchentlich wiederkehrend für jeden
          Wochentag.
        </p>
        <div className="mb-4">
          <Alert
            type="info"
            message="Leere Tage bedeuten, dass das Kind an diesem Tag nicht zur OGS kommt."
          />
        </div>
        {error ? (
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        ) : null}

        <div className="space-y-4">
          {WEEKDAYS.map((day) => {
            const schedule = schedules.find((s) => s.weekday === day.value);
            return (
              <div
                key={day.value}
                className="rounded-lg border border-gray-200 p-3"
              >
                <div className="mb-2 font-medium text-gray-900">
                  {day.label}
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div>
                    <label
                      htmlFor={`arrival-time-${day.value}`}
                      className="mb-1 block text-xs text-gray-500"
                    >
                      Ankunftszeit
                    </label>
                    <input
                      id={`arrival-time-${day.value}`}
                      type="time"
                      value={schedule?.expected_arrival ?? ""}
                      onChange={(e) =>
                        handleTimeChange(day.value, e.target.value)
                      }
                      className="focus:border-moto-green focus:ring-moto-green w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-1 focus:outline-none"
                      placeholder="HH:MM"
                    />
                  </div>
                  <div>
                    <label
                      htmlFor={`arrival-notes-${day.value}`}
                      className="mb-1 block text-xs text-gray-500"
                    >
                      Notiz (optional)
                    </label>
                    <input
                      id={`arrival-notes-${day.value}`}
                      type="text"
                      value={schedule?.notes ?? ""}
                      onChange={(e) =>
                        handleNotesChange(day.value, e.target.value)
                      }
                      className="focus:border-moto-green focus:ring-moto-green w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-1 focus:outline-none"
                      placeholder="Besonderheiten..."
                      maxLength={500}
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <p className="mt-4 text-xs text-gray-500">
          Lassen Sie die Ankunftszeit leer für Tage ohne OGS-Besuch.
        </p>
      </form>
    </FormModal>
  );
}
