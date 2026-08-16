"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { StaffCardsSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: renders the real header immediately (Polaris: real
 * chrome first, skeletonize only the data region) with a disabled no-op
 * search field — this component has no page state yet — followed by the
 * card-grid skeleton for the data region.
 */
export default function StaffLoading() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Mitarbeiter"
        search={{
          value: "",
          onChange: () => {},
          inputProps: { disabled: true },
        }}
      />
      <StaffCardsSkeleton />
    </div>
  );
}
