"use client";

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import { Checkbox } from "~/components/ui/checkbox";
import { createLogger } from "~/lib/logger";
import {
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
} from "~/lib/arrival-schedule-helpers";
import type { CareDaysSource } from "~/lib/student-arrival-api";

const logger = createLogger({ component: "ArrivalScheduleForm" });

interface ArrivalScheduleFormModalProps {
  readonly isOpen: boolean;
  readonly careDaysSource: CareDaysSource;
  readonly onClose: () => void;
  readonly onSubmit: (schedules: ArrivalScheduleFormEntry[]) => Promise<void>;
  readonly initialSchedules: ArrivalScheduleFormEntry[];
}

export function ArrivalScheduleFormModal({
  isOpen,
  careDaysSource,
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

  const handleCareChange = (weekday: number, inCare: boolean) => {
    setSchedules((prev) =>
      prev.map((s) =>
        s.weekday === weekday
          ? { ...s, inCare, expected_arrival: inCare ? s.expected_arrival : "" }
          : s,
      ),
    );
  };

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

    const careDays = schedules.filter((s) => s.inCare);
    const timeRegex = /^([01]?\d|2[0-3]):[0-5]\d$/;
    for (const entry of careDays) {
      if (entry.expected_arrival.trim() === "") continue;
      if (!timeRegex.test(entry.expected_arrival)) {
        setError(
          `Ungültiges Zeitformat für ${WEEKDAYS.find((w) => w.value === entry.weekday)?.label}. Bitte HH:MM.`,
        );
        return;
      }
    }

    setIsSubmitting(true);
    try {
      const schedulesToSave = careDays.filter(
        (entry) =>
          careDaysSource === "weekly_plan" ||
          entry.expected_arrival.trim() !== "" ||
          Boolean(entry.notes?.trim()),
      );
      await onSubmit(schedulesToSave);
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
        {careDaysSource === "bookings" ? (
          <p className="mb-4 text-sm text-gray-500">
            Die Betreuungstage kommen aus den Buchungen.
          </p>
        ) : (
          <p className="mb-4 text-sm text-gray-500">
            Wählen Sie die Betreuungstage. Das gilt für jede Woche.
          </p>
        )}
        <div className="mb-4">
          <Alert
            type="info"
            message="Die Uhrzeit kommt aus der Klasse. Geben Sie hier nur eine andere Uhrzeit ein. Das gilt, wenn das Kind früher oder später kommt."
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
                <div className="mb-2 flex items-center gap-2">
                  <Checkbox
                    id={`arrival-care-${day.value}`}
                    checked={schedule?.inCare ?? false}
                    disabled={careDaysSource === "bookings"}
                    onChange={(e) =>
                      handleCareChange(day.value, e.target.checked)
                    }
                  />
                  <label
                    htmlFor={`arrival-care-${day.value}`}
                    className="font-medium text-gray-900"
                  >
                    {day.label}
                  </label>
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div>
                    <label
                      htmlFor={`arrival-time-${day.value}`}
                      className="mb-1 block text-xs text-gray-500"
                    >
                      Andere Uhrzeit (optional)
                    </label>
                    <input
                      id={`arrival-time-${day.value}`}
                      type="time"
                      value={schedule?.expected_arrival ?? ""}
                      onChange={(e) =>
                        handleTimeChange(day.value, e.target.value)
                      }
                      disabled={!schedule?.inCare}
                      className="focus:border-moto-green focus:ring-moto-green w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-1 focus:outline-none disabled:bg-gray-50 disabled:text-gray-400"
                      placeholder="HH:MM"
                    />
                    {schedule?.inCare && schedule.classTime ? (
                      <p className="mt-1 text-xs text-gray-500">
                        Ohne Eintrag gilt {schedule.classTime} Uhr aus der
                        Klasse.
                      </p>
                    ) : null}
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
                      disabled={!schedule?.inCare}
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
          {careDaysSource === "bookings"
            ? "Ändern Sie Betreuungstage bei den Buchungen."
            : "Ohne Haken kommt das Kind an diesem Tag nicht."}
        </p>
      </form>
    </FormModal>
  );
}
