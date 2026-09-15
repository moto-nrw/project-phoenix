"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";

export default function TransitLoading() {
  return (
    <TenantPage
      title="Unterwegs"
      statsLoading
      back
      backHref="/rooms"
      backLabel="Zurück zu den Räumen"
    >
      <SkeletonRegion label="Kinder ohne Raum werden geladen">
        <ListSkeleton rows={4} avatar={false} />
      </SkeletonRegion>
    </TenantPage>
  );
}
