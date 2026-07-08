"use client";

import { useSelectedLayoutSegment } from "next/navigation";
import { MasterDetailSkeleton } from "~/components/database/master-detail-skeleton";
import { DatabaseIndexSkeleton } from "./page-skeleton";

/**
 * Picks the skeleton matching whichever /database/* route is active: the
 * card-grid skeleton for the index page, or the existing master-detail
 * skeleton for every list/detail sub-route (students, personal, rooms, ...).
 * Shared by loading.tsx (Next.js route-loading Suspense boundary) and
 * layout.tsx (RoleGuard's client-side session-loading fallback) so both
 * boundaries render the same shape — no flash between mismatched skeletons.
 */
export function DatabaseSectionSkeleton() {
  const segment = useSelectedLayoutSegment();
  return segment === null ? (
    <DatabaseIndexSkeleton />
  ) : (
    <MasterDetailSkeleton />
  );
}
