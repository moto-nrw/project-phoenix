"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Mirrors the header block, den Zeitraum-Umschalter und die Karte mit
// Anteilsleiste, Legende und Diagramm, so it swaps in without layout shift.
export function FeedbackHistorySkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Feedbackhistorie wird geladen"
      data-testid="feedback-history-skeleton"
      className="w-full"
    >
      <Skeleton className="mb-4 h-9 w-24 rounded-lg" />

      {/* Spiegelt die Kopfkarte (PageIntro) des geladenen Zustands. */}
      <div className="moto-content-surface mb-6 rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex min-w-0 gap-3">
          <Skeleton className="h-12 w-12 flex-shrink-0 rounded-xl" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-3 w-32 rounded" />
            <Skeleton className="h-7 w-48 rounded" />
            <Skeleton className="h-4 w-56 rounded" />
          </div>
        </div>
      </div>

      <div className="mb-6">
        <Skeleton className="h-9 w-full max-w-md rounded-full" />
      </div>

      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="p-4 sm:p-6 md:p-8">
          <Skeleton className="mb-4 h-5 w-40 rounded" />

          <Skeleton className="mt-4 h-3 w-full rounded-full" />

          <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1">
            {Array.from({ length: 3 }, (_, i) => (
              <div key={i} className="flex items-center gap-1.5">
                <Skeleton className="h-2.5 w-2.5 rounded-full" />
                <Skeleton className="h-4 w-16 rounded" />
              </div>
            ))}
          </div>

          <Skeleton className="mt-6 h-[180px] w-full rounded-lg sm:h-[220px]" />
        </div>
      </div>
    </div>
  );
}
