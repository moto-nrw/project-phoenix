"use client";

import { Skeleton } from "~/components/ui/skeleton";

// ─── Übersicht tab skeleton ──────────────────────────────────────────────────
// Mirrors uebersicht-tab.tsx: 3 KpiCards, then a 2-column chart-card grid
// (Tagesvergleich line chart + Saldo-Verlauf area chart), then a donut +
// legend card — so the loaded tab swaps in without layout shift.

export function UebersichtTabSkeleton() {
  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {Array.from({ length: 3 }, (_, i) => (
          <div
            key={i}
            className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-sm"
          >
            <Skeleton className="h-3 w-24 rounded" />
            <Skeleton className="mt-2 h-7 w-20 rounded" />
            <Skeleton className="mt-3 h-3 w-32 rounded" />
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
        {Array.from({ length: 2 }, (_, i) => (
          <div
            key={i}
            className="rounded-3xl border border-gray-100/50 bg-white/90 p-4 shadow-sm sm:p-6"
          >
            <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <Skeleton className="h-3 w-40 rounded" />
              <Skeleton className="h-9 w-40 rounded-lg" />
            </div>
            <Skeleton className="h-64 w-full rounded-xl" />
          </div>
        ))}
      </div>

      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-4 shadow-sm sm:p-6">
        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Skeleton className="h-3 w-32 rounded" />
          <Skeleton className="h-9 w-40 rounded-lg" />
        </div>
        <div className="grid grid-cols-1 items-center gap-6 md:grid-cols-2">
          <Skeleton className="mx-auto h-64 w-64 rounded-full" />
          <div className="flex flex-col gap-3">
            {Array.from({ length: 4 }, (_, i) => (
              <div key={i} className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <Skeleton className="h-2.5 w-2.5 rounded-full" />
                  <Skeleton className="h-3 w-20 rounded" />
                </div>
                <Skeleton className="h-3 w-16 rounded" />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Route-level loading skeleton ────────────────────────────────────────────

export function StaffDetailSkeleton() {
  // Mirrors StaffHeader (avatar + name block + status badge) and the
  // TabsList line, then the Übersicht tab body, so the loaded page swaps in
  // without layout shift.
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Mitarbeiter wird geladen"
      data-testid="staff-detail-skeleton"
      className="-mt-1.5 w-full"
    >
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 flex-1 items-start gap-4">
          <Skeleton className="h-14 w-14 flex-shrink-0 rounded-full sm:h-16 sm:w-16" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-3 w-32 rounded" />
            <Skeleton className="h-7 w-48 rounded" />
            <Skeleton className="h-4 w-40 rounded" />
          </div>
        </div>
        <div className="flex flex-shrink-0 items-center gap-2">
          <Skeleton className="h-7 w-24 rounded-full" />
          <Skeleton className="h-10 w-10 rounded-full" />
        </div>
      </div>

      <div className="mb-6 flex gap-6 border-b border-gray-200 pb-px">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-5 w-24 rounded" />
        ))}
      </div>

      <UebersichtTabSkeleton />
    </div>
  );
}
