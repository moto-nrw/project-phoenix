"use client";

import { MasterDetailSkeleton } from "~/components/database/master-detail-skeleton";

/**
 * Route-level loading UI: renders the same skeleton the page shows while its
 * data loads, so navigation shows one continuous skeleton instead of the
 * generic group-level Loading followed by the page skeleton.
 */
export default function DatabaseLoading() {
  return <MasterDetailSkeleton />;
}
