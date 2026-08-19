"use client";

import { CareOfferingsEditor } from "~/components/enrollment/care-offerings-editor";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function CareOfferingsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) {
    return (
      <SkeletonRegion
        label="Betreuungsangebote werden geladen"
        className="-mt-1.5 w-full"
      >
        <TableSkeleton rows={5} columns={4} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <CareOfferingsEditor />
      </div>
    </div>
  );
}
