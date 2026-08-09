"use client";

import { useEffect, useState } from "react";
import { ChevronDown, Loader2, StickyNote } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
} from "~/lib/arrival-schedule-helpers";
import type {
  BulkPickupScheduleFormData,
  PickupScheduleFormData,
} from "~/lib/pickup-schedule-helpers";

interface CareWeeklyPlanModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly initialArrivalSchedules: ArrivalScheduleFormEntry[];
  readonly initialPickupSchedules: PickupScheduleFormData[];
  readonly onSubmit: (data: {
    arrivalSchedules: ArrivalScheduleFormEntry[];
    pickupData: BulkPickupScheduleFormData;
  }) => Promise<void>;
  // Success toast shown after onSubmit resolves. Defaults to the post-creation
  // "saved" wording; the create-student flow overrides it because the plan is
  // only staged locally there, not yet persisted.
  readonly successMessage?: string;
}

interface CareWeeklyPlanRow {
  readonly weekday: number;
  readonly arrivalTime: string;
  readonly arrivalNotes?: string | null;
  readonly pickupTime: string;
  readonly pickupNotes?: string;
}

const TIME_PATTERN = /^([01]?\d|2[0-3]):[0-5]\d$/;

export function CareWeeklyPlanModal({
  isOpen,
  onClose,
  initialArrivalSchedules,
  initialPickupSchedules,
  onSubmit,
  successMessage = "Wochenplan wurde gespeichert",
}: CareWeeklyPlanModalProps) {
  const toast = useToast();
  const [rows, setRows] = useState<CareWeeklyPlanRow[]>([]);
  const [expandedWeekdays, setExpandedWeekdays] = useState<Set<number>>(
    () => new Set(),
  );
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setRows(
      WEEKDAYS.map((day) => {
        const arrival = initialArrivalSchedules.find(
          (schedule) => schedule.weekday === day.value,
        );
        const pickup = initialPickupSchedules.find(
          (schedule) => schedule.weekday === day.value,
        );
        return {
          weekday: day.value,
          arrivalTime: arrival?.expected_arrival ?? "",
          arrivalNotes: arrival?.notes ?? null,
          pickupTime: pickup?.pickupTime ?? "",
          pickupNotes: pickup?.notes,
        };
      }),
    );
    setExpandedWeekdays(
      new Set(
        WEEKDAYS.filter((day) => {
          const arrival = initialArrivalSchedules.find(
            (schedule) => schedule.weekday === day.value,
          );
          const pickup = initialPickupSchedules.find(
            (schedule) => schedule.weekday === day.value,
          );
          return Boolean(arrival?.notes || pickup?.notes);
        }).map((day) => day.value),
      ),
    );
    setError(null);
  }, [isOpen, initialArrivalSchedules, initialPickupSchedules]);

  const updateRow = (
    weekday: number,
    field: "arrivalTime" | "pickupTime",
    value: string,
  ) => {
    setRows((currentRows) =>
      currentRows.map((row) =>
        row.weekday === weekday ? { ...row, [field]: value } : row,
      ),
    );
  };

  const updateNote = (
    weekday: number,
    field: "arrivalNotes" | "pickupNotes",
    value: string,
  ) => {
    setRows((currentRows) =>
      currentRows.map((row) =>
        row.weekday === weekday
          ? { ...row, [field]: value.trim() ? value : undefined }
          : row,
      ),
    );
  };

  const toggleNotes = (weekday: number) => {
    setExpandedWeekdays((current) => {
      const next = new Set(current);
      if (next.has(weekday)) {
        next.delete(weekday);
      } else {
        next.add(weekday);
      }
      return next;
    });
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);

    for (const row of rows) {
      const day = WEEKDAYS.find((weekday) => weekday.value === row.weekday);
      if (row.arrivalTime && !TIME_PATTERN.test(row.arrivalTime)) {
        setError(`Ungültige Ankunftszeit für ${day?.label ?? "diesen Tag"}.`);
        return;
      }
      if (row.pickupTime && !TIME_PATTERN.test(row.pickupTime)) {
        setError(`Ungültige Abholzeit für ${day?.label ?? "diesen Tag"}.`);
        return;
      }
    }

    const arrivalSchedules = rows
      .filter((row) => row.arrivalTime.trim() !== "")
      .map((row) => ({
        weekday: row.weekday,
        expected_arrival: row.arrivalTime,
        notes: row.arrivalNotes ?? null,
      }));
    const pickupData = {
      schedules: rows
        .filter((row) => row.pickupTime.trim() !== "")
        .map((row) => ({
          weekday: row.weekday,
          pickupTime: row.pickupTime,
          notes: row.pickupNotes,
        })),
    };

    setIsSubmitting(true);
    try {
      await onSubmit({ arrivalSchedules, pickupData });
      toast.success(successMessage);
      onClose();
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Wochenplan konnte nicht gespeichert werden";
      setError(message);
      toast.error(message);
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        disabled={isSubmitting}
        className="h-10 w-full rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
      >
        Abbrechen
      </button>
      <button
        type="submit"
        form="care-weekly-plan-form"
        disabled={isSubmitting}
        className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
      >
        {isSubmitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
        Wochenplan speichern
      </button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Wochenplan bearbeiten"
      footer={footer}
      size="xl"
      mobilePosition="bottom"
    >
      <form id="care-weekly-plan-form" onSubmit={handleSubmit}>
        {error ? (
          <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong mb-4 rounded-xl border px-4 py-3 text-sm">
            {error}
          </div>
        ) : null}

        <div className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm sm:rounded-2xl">
          <div className="hidden grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)] gap-3 border-b border-gray-100 bg-gray-50 px-4 py-3 text-xs font-semibold tracking-wide text-gray-500 uppercase sm:grid">
            <span>Tag</span>
            <span>Ankunft</span>
            <span>Abholung</span>
          </div>
          <div className="divide-y divide-gray-100">
            {WEEKDAYS.map((day) => {
              const row = rows.find((entry) => entry.weekday === day.value);
              return (
                <div
                  key={day.value}
                  className="grid gap-3 px-3 py-4 sm:grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)] sm:items-center sm:px-4"
                >
                  <div>
                    <div className="text-sm font-semibold text-gray-900">
                      {day.label}
                    </div>
                    <button
                      type="button"
                      onClick={() => toggleNotes(day.value)}
                      className="mt-1 inline-flex items-center gap-1 text-xs font-medium text-gray-500 transition-colors hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    >
                      <StickyNote className="h-3.5 w-3.5" aria-hidden="true" />
                      Notizen
                      <ChevronDown
                        className={`h-3.5 w-3.5 transition-transform ${
                          expandedWeekdays.has(day.value) ? "rotate-180" : ""
                        }`}
                        aria-hidden="true"
                      />
                    </button>
                  </div>
                  <TimeField
                    id={`weekly-arrival-${day.value}`}
                    label="Ankunft"
                    value={row?.arrivalTime ?? ""}
                    onChange={(value) =>
                      updateRow(day.value, "arrivalTime", value)
                    }
                  />
                  <TimeField
                    id={`weekly-pickup-${day.value}`}
                    label="Abholung"
                    value={row?.pickupTime ?? ""}
                    onChange={(value) =>
                      updateRow(day.value, "pickupTime", value)
                    }
                  />
                  {expandedWeekdays.has(day.value) ? (
                    <div className="grid gap-3 sm:col-span-3 sm:grid-cols-[minmax(100px,0.7fr)_minmax(140px,1fr)_minmax(140px,1fr)]">
                      <div className="hidden sm:block" />
                      <NoteField
                        id={`weekly-arrival-notes-${day.value}`}
                        label="Ankunftsnotiz"
                        value={row?.arrivalNotes ?? ""}
                        onChange={(value) =>
                          updateNote(day.value, "arrivalNotes", value)
                        }
                      />
                      <NoteField
                        id={`weekly-pickup-notes-${day.value}`}
                        label="Abholnotiz"
                        value={row?.pickupNotes ?? ""}
                        onChange={(value) =>
                          updateNote(day.value, "pickupNotes", value)
                        }
                      />
                    </div>
                  ) : null}
                </div>
              );
            })}
          </div>
        </div>
      </form>
    </FormModal>
  );
}

function NoteField({
  id,
  label,
  value,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label className="block" htmlFor={id}>
      <span className="mb-1 block text-xs font-medium text-gray-500">
        {label}
      </span>
      <input
        id={id}
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        maxLength={500}
        className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
        placeholder="Optional"
      />
    </label>
  );
}

function TimeField({
  id,
  label,
  value,
  onChange,
}: {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label className="block" htmlFor={id}>
      <span className="mb-1 block text-xs font-medium text-gray-500 sm:hidden">
        {label}
      </span>
      <input
        id={id}
        type="time"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-11 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none sm:h-10"
      />
    </label>
  );
}
