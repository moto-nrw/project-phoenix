"use client";

import { useState, useEffect } from "react";
import { Loader2 } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import type {
  BulkPickupScheduleFormData,
  PickupScheduleFormData,
} from "@/lib/pickup-schedule-helpers";
import { WEEKDAYS } from "@/lib/pickup-schedule-helpers";

interface PickupScheduleFormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSubmit: (data: BulkPickupScheduleFormData) => Promise<void>;
  readonly initialSchedules: PickupScheduleFormData[];
}

export function PickupScheduleFormModal({
  isOpen,
  onClose,
  onSubmit,
  initialSchedules,
}: PickupScheduleFormModalProps) {
  const [schedules, setSchedules] =
    useState<PickupScheduleFormData[]>(initialSchedules);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setSchedules(initialSchedules);
      setError(null);
    }
  }, [isOpen, initialSchedules]);

  const handleTimeChange = (weekday: number, time: string) => {
    setSchedules((prev) =>
      prev.map((s) => (s.weekday === weekday ? { ...s, pickupTime: time } : s)),
    );
  };

  const handleNotesChange = (weekday: number, notes: string) => {
    setSchedules((prev) =>
      prev.map((s) =>
        s.weekday === weekday ? { ...s, notes: notes || undefined } : s,
      ),
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Filter to only include schedules with a pickup time
    const validSchedules = schedules.filter((s) => s.pickupTime.trim() !== "");

    if (validSchedules.length === 0) {
      setError("Bitte geben Sie mindestens eine Abholzeit an.");
      return;
    }

    // Validate time format (HH:MM)
    const timeRegex = /^([01]?\d|2[0-3]):[0-5]\d$/;
    for (const schedule of validSchedules) {
      if (!timeRegex.test(schedule.pickupTime)) {
        setError(
          `Ungültiges Zeitformat für ${WEEKDAYS.find((w) => w.value === schedule.weekday)?.label}. Bitte verwenden Sie HH:MM.`,
        );
        return;
      }
    }

    setIsSubmitting(true);
    try {
      await onSubmit({ schedules: validSchedules });
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Speichern des Abholplans",
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
        form="pickup-schedule-form"
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
      title="Wöchentlichen Abholplan bearbeiten"
      footer={footer}
      size="md"
      mobilePosition="center"
    >
      <form id="pickup-schedule-form" onSubmit={handleSubmit}>
        {error && (
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        )}

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
                      htmlFor={`pickup-time-${day.value}`}
                      className="mb-1 block text-xs text-gray-500"
                    >
                      Abholzeit
                    </label>
                    <input
                      id={`pickup-time-${day.value}`}
                      type="time"
                      value={schedule?.pickupTime ?? ""}
                      onChange={(e) =>
                        handleTimeChange(day.value, e.target.value)
                      }
                      className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                      placeholder="HH:MM"
                    />
                  </div>
                  <div>
                    <label
                      htmlFor={`pickup-notes-${day.value}`}
                      className="mb-1 block text-xs text-gray-500"
                    >
                      Notiz (optional)
                    </label>
                    <input
                      id={`pickup-notes-${day.value}`}
                      type="text"
                      value={schedule?.notes ?? ""}
                      onChange={(e) =>
                        handleNotesChange(day.value, e.target.value)
                      }
                      className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                      placeholder="Abholer, Besonderheiten..."
                      maxLength={500}
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <p className="mt-4 text-xs text-gray-500">
          Lassen Sie die Abholzeit leer für Tage ohne feste Abholzeit.
        </p>
      </form>
    </FormModal>
  );
}
