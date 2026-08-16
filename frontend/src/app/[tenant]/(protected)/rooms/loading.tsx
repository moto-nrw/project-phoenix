"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { RoomsGridSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: renders the real header immediately (Polaris: real
 * chrome first, skeletonize only the data region) with a disabled no-op
 * search field — this component has no page state yet — followed by the
 * room-card grid skeleton.
 */
export default function RoomsLoading() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Räume"
        search={{ value: "", onChange: () => {} }}
      />
      <RoomsGridSkeleton />
    </div>
  );
}
