"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { RoomsGridSkeleton } from "./page-skeleton";

export default function RoomsLoading() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Räume"
        search={{
          value: "",
          onChange: () => {},
          inputProps: { disabled: true },
        }}
      />
      <RoomsGridSkeleton />
    </div>
  );
}
