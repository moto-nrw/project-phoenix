"use client";

import { CareOfferingsEditor } from "~/components/enrollment/care-offerings-editor";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function CareOfferingsPage() {
  const { isReady } = useRequireAdmin();

  return (
    <div className="-mt-1.5 w-full space-y-6">
      {/* Der Kopf steht bewusst außerhalb des lg-Gates: mobil ist er der
          einzige Seitentitel, den die Kopfzeile nicht liefert. */}
      <PageHeaderWithSearch title="Betreuungsangebote" />
      <DesktopOnlyNotice />
      {isReady ? (
        <div className="hidden lg:block">
          <CareOfferingsEditor />
        </div>
      ) : (
        <SkeletonRegion
          label="Betreuungsangebote werden geladen"
          className="hidden w-full lg:block"
        >
          <TableSkeleton rows={5} columns={4} />
        </SkeletonRegion>
      )}
    </div>
  );
}
