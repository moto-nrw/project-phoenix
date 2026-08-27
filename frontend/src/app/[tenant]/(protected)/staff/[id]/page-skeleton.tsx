"use client";

import { BackButton } from "~/components/ui/back-button";
import { Skeleton } from "~/components/ui/skeleton";
import { TenantPageHeaderSkeleton } from "~/components/ui/page-skeletons";
import { UebersichtTabSkeleton } from "~/components/staff/uebersicht-tab-skeleton";

// ─── Route-level loading skeleton ────────────────────────────────────────────

/**
 * Mirrors StaffHeader (avatar + name block + status badge + kebab trigger).
 * Used both standalone (route-level loading, permission still unknown) and
 * as the placeholder for the real StaffHeader while the staff fetch is in
 * flight — the name/status/menu are all data-bound, so they stay skeletons
 * until `staff` loads.
 */
function StaffHeaderSkeleton() {
  return (
    // Spiegelt die Kopfkarte des geladenen Zustands: dieselbe
    // moto-content-surface-Karte mit Avatar, Kicker, Titel und Aktionen.
    <TenantPageHeaderSkeleton leading />
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
      className="w-full"
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
