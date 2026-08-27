"use client";

import { BackButton } from "~/components/ui/back-button";
import { Skeleton } from "~/components/ui/skeleton";
import { UebersichtTabSkeleton } from "~/components/staff/uebersicht-tab-skeleton";

// ─── Route-level loading skeleton ────────────────────────────────────────────

/**
 * Mirrors StaffHeader (avatar + name block + status badge + kebab trigger).
 * Used both standalone (route-level loading, permission still unknown) and
 * as the placeholder for the real StaffHeader while the staff fetch is in
 * flight — the name/status/menu are all data-bound, so they stay skeletons
 * until `staff` loads.
 */
export function StaffHeaderSkeleton() {
  return (
    <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="flex min-w-0 flex-1 items-start gap-4">
        <Skeleton className="h-16 w-16 flex-shrink-0 rounded-full" />
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
  );
}

export function StaffDetailSkeleton() {
  // Mirrors StaffHeader (avatar + name block + status badge) and the
  // TabsList line, then the Übersicht tab body, so the loaded page swaps in
  // without layout shift. Used only while the session (and therefore
  // permissions/tab set) is still unknown — once permissions are known the
  // page renders the real tab bar and StaffHeaderSkeleton instead (see
  // page.tsx), since the tab set itself doesn't need the staff fetch.
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Mitarbeiter wird geladen"
      data-testid="staff-detail-skeleton"
      className="-mt-1.5 w-full"
    >
      <BackButton referrer="/staff" />

      <StaffHeaderSkeleton />

      <div className="mb-6 flex gap-6 border-b border-gray-200 pb-px">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-5 w-24 rounded" />
        ))}
      </div>

      <UebersichtTabSkeleton />
    </div>
  );
}
