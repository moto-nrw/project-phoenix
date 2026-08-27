"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { RoomsGridSkeleton } from "./page-skeleton";

export default function RoomsLoading() {
  return (
    <div className="w-full">
      {/* Kicker, Titel und Erklärtext sind statisch und rendern deshalb
          sofort als echte Kopfkarte; nur das Raster skeletonisiert. */}
      <PageIntro title="Räume" className="mb-6" />
      <PageHeaderWithSearch
        title=""
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
