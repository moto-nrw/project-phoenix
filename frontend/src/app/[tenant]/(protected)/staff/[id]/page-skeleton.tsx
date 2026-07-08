"use client";

import { Skeleton } from "~/components/ui/skeleton";

// ─── Loading skeleton ────────────────────────────────────────────────────────

export function StaffDetailSkeleton() {
  // Mirrors StaffHeader (avatar + name block + status badge) and the
  // TabsList line, then two DataGrid-shaped field sections for the
  // Übersicht tab body, so the loaded page swaps in without layout shift.
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

      <div className="space-y-6">
        {Array.from({ length: 2 }, (_, section) => (
          <div
            key={section}
            className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-sm"
          >
            <Skeleton className="mb-4 h-5 w-40 rounded" />
            <dl className="grid grid-cols-1 gap-x-4 gap-y-3 sm:grid-cols-2">
              {Array.from({ length: 4 }, (_, field) => (
                <div key={field} className="space-y-1.5">
                  <Skeleton className="h-3 w-20 rounded" />
                  <Skeleton className="h-4 w-32 rounded" />
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>
    </div>
  );
}
