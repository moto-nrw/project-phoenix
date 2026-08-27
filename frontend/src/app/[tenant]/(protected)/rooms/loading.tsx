"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { Skeleton } from "~/components/ui/skeleton";
import { RoomsGridSkeleton } from "./page-skeleton";

export default function RoomsLoading() {
  return (
    <div className="w-full">
      {/* Kicker, Titel und Erklärtext sind statisch und rendern deshalb
          sofort als echte Kopfkarte; nur das Raster skeletonisiert. */}
      <PageIntro
        title="Räume"
        description={<Skeleton className="h-4 w-40" />}
        className="mb-6"
      >
        <PageHeaderWithSearch
          embedded
          title=""
          search={{
            value: "",
            onChange: () => {},
            inputProps: { disabled: true },
          }}
        />
      </PageIntro>
      <RoomsGridSkeleton />
    </div>
  );
}
