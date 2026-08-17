"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { StaffCardsSkeleton } from "./page-skeleton";

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
