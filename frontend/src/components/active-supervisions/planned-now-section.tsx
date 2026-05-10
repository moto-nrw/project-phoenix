"use client";

import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

interface PlannedNowSectionProps {
  readonly plannedNow: PlannedTimetableInstance[];
  readonly isStartingInstance: string | null;
  readonly onStart: (instance: PlannedTimetableInstance) => void;
}

export function PlannedNowSection({
  plannedNow,
  isStartingInstance,
  onStart,
}: PlannedNowSectionProps) {
  if (plannedNow.length === 0) {
    return null;
  }

  return (
    <section className="mb-6 rounded-lg border border-[#83CD2D]/30 bg-white p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-base font-semibold text-gray-900">Jetzt geplant</h2>
        <span className="text-sm text-gray-500">
          {plannedNow.length} Aktivität{plannedNow.length === 1 ? "" : "en"}
        </span>
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {plannedNow.map((instance) => (
          <article
            key={instance.id}
            className="rounded-lg border border-gray-200 bg-gray-50 p-4"
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <h3 className="font-medium text-gray-900">{instance.title}</h3>
                <p className="mt-1 text-sm text-gray-600">
                  {instance.startTime}–{instance.endTime}
                </p>
              </div>
              {instance.isOverdue && (
                <span className="rounded-full bg-[#F3B63F]/20 px-2 py-1 text-xs font-medium text-[#A66F00]">
                  Überfällig
                </span>
              )}
            </div>
            <div className="mt-3 text-sm text-gray-600">
              {instance.expectedStudentsCount} erwartet
            </div>
            <button
              type="button"
              disabled={isStartingInstance === instance.id}
              onClick={() => onStart(instance)}
              className="mt-4 w-full rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
            >
              Jetzt starten
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}
