"use client";

import { Skeleton } from "~/components/ui/skeleton";
import {
  timetableSurface,
  timetableSurfacePadded,
} from "~/components/timetable/timetable-style";
import type { TimetableView } from "~/lib/timetable-helpers";

export function TimetableContentSkeleton({ view }: { view: TimetableView }) {
  if (view === "series") {
    return (
      <div
        role="status"
        aria-live="polite"
        aria-busy="true"
        aria-label="Regeltermine werden geladen"
        data-testid="timetable-content-skeleton"
        className="grid gap-3 md:grid-cols-2 xl:grid-cols-3"
      >
        {[0, 1, 2].map((item) => (
          <div key={item} className={timetableSurfacePadded}>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-3 w-28" />
              </div>
              <Skeleton className="h-8 w-8 rounded-lg" />
            </div>
            <div className="mt-5 space-y-2">
              <Skeleton className="h-10 w-full rounded-xl" />
              <Skeleton className="h-10 w-11/12 rounded-xl" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (view === "week") {
    const weekSkeletonEvents = [
      [
        { top: 300, height: 64, width: "92%" },
        { top: 374, height: 58, width: "86%" },
        { top: 456, height: 66, width: "74%" },
      ],
      [
        { top: 300, height: 64, width: "90%" },
        { top: 374, height: 58, width: "88%" },
        { top: 448, height: 50, width: "44%" },
        { top: 504, height: 54, width: "78%" },
      ],
      [
        { top: 300, height: 64, width: "92%" },
        { top: 448, height: 50, width: "46%" },
        { top: 504, height: 54, width: "76%" },
      ],
      [
        { top: 300, height: 64, width: "90%" },
        { top: 374, height: 58, width: "88%" },
        { top: 448, height: 50, width: "42%" },
      ],
      [
        { top: 300, height: 64, width: "92%" },
        { top: 374, height: 58, width: "86%" },
        { top: 448, height: 50, width: "48%" },
        { top: 504, height: 54, width: "80%" },
      ],
      [
        { top: 318, height: 54, width: "72%" },
        { top: 452, height: 54, width: "60%" },
      ],
      [{ top: 336, height: 50, width: "64%" }],
    ];

    return (
      <div
        role="status"
        aria-live="polite"
        aria-busy="true"
        aria-label="Wochenplan wird geladen"
        data-testid="timetable-content-skeleton"
        className={`${timetableSurface} overflow-hidden`}
      >
        <div className="hidden h-14 grid-cols-[64px_repeat(7,minmax(0,1fr))] border-b border-gray-200 bg-white sm:grid">
          <div aria-hidden />
          {Array.from({ length: 7 }, (_, day) => (
            <div
              key={day}
              className="flex items-center justify-center gap-2 border-l border-gray-200 px-2 py-2"
            >
              <Skeleton className="h-3 w-6" />
              <Skeleton className="h-6 w-6 rounded-full" />
            </div>
          ))}
        </div>
        <div className="relative grid h-[560px] grid-cols-[40px_minmax(0,1fr)] sm:grid-cols-[64px_repeat(7,minmax(0,1fr))]">
          <div className="space-y-16 border-r border-gray-200 bg-gray-50 px-2 py-4">
            {Array.from({ length: 7 }, (_, hour) => (
              <Skeleton key={hour} className="ml-auto h-2.5 w-8" />
            ))}
          </div>
          {Array.from({ length: 7 }, (_, day) => (
            <div
              key={day}
              className={`relative overflow-hidden border-l border-gray-200 ${day === 0 ? "block" : "hidden sm:block"}`}
            >
              {Array.from({ length: 7 }, (_, line) => (
                <div
                  key={line}
                  className="absolute right-0 left-0 border-t border-gray-100"
                  style={{ top: `${(line + 1) * 70}px` }}
                />
              ))}
              {(weekSkeletonEvents[day] ?? []).map((event) => (
                <div
                  key={`${day}-${event.top}-${event.height}-${event.width}`}
                  className="absolute left-2 rounded-xl border border-gray-200 bg-white p-2 shadow-sm sm:left-2.5"
                  style={{
                    top: `${event.top}px`,
                    height: `${event.height}px`,
                    width: event.width,
                    maxWidth: "calc(100% - 1rem)",
                  }}
                >
                  <Skeleton className="h-3 w-4/5 bg-gray-300/80" />
                  <Skeleton className="mt-2 h-2.5 w-1/2 bg-gray-300/80" />
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div
      role="status"
      aria-live="polite"
      aria-busy="true"
      aria-label="Monatsplan wird geladen"
      data-testid="timetable-content-skeleton"
      className={`${timetableSurface} overflow-hidden`}
    >
      <div className="grid grid-cols-7 border-b border-gray-200 bg-white">
        {Array.from({ length: 7 }, (_, day) => (
          <div key={day} className="px-3 py-3">
            <Skeleton className="mx-auto h-2.5 w-8" />
          </div>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {Array.from({ length: 42 }, (_, day) => (
          <div
            key={day}
            className="min-h-[112px] border-r border-b border-gray-100 bg-white p-2"
          >
            <Skeleton className="h-5 w-5 rounded-full" />
            <div className="mt-5 space-y-1.5">
              {day % 3 === 0 && (
                <Skeleton className="h-5 w-full rounded-full" />
              )}
              {day % 4 === 0 && (
                <Skeleton className="h-5 w-10/12 rounded-full" />
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * Skeleton der zweizeiligen PlanningContextBar-Kopfzeile: Navigation,
 * Ansichts-Umschalter, Primäraktion und Kontextzeile als Platzhalter — passend
 * zur neuen Kopfzeile (statt der abgebauten Werkzeugleiste + Setup-Karten).
 */
function PlanningContextBarSkeleton() {
  return (
    <div
      role="status"
      aria-live="polite"
      aria-busy="true"
      aria-label="Betreuungsplan-Kopfzeile wird geladen"
      data-testid="timetable-header-skeleton"
      className="moto-content-surface flex flex-col gap-2 rounded-2xl border px-4 py-3"
    >
      <div className="flex min-h-9 flex-wrap items-center gap-3">
        <Skeleton className="h-8 w-28 rounded-lg" />
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-8 w-48 rounded-lg" />
        <Skeleton className="ml-auto h-9 w-24 rounded-lg bg-gray-300" />
      </div>
      <div className="border-t border-gray-100 pt-2">
        <div className="min-h-8">
          <Skeleton className="h-5 w-32" />
        </div>
      </div>
    </div>
  );
}

export function TimetablePageSkeleton() {
  return (
    <div className="flex flex-col gap-4" data-testid="timetable-page-skeleton">
      <PlanningContextBarSkeleton />
      <TimetableContentSkeleton view="week" />
    </div>
  );
}
