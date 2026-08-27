"use client";

import { CareOfferingsEditor } from "~/components/enrollment/care-offerings-editor";
import { PageIntro } from "~/components/ui/page-intro";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function CareOfferingsPage() {
  const { isReady } = useRequireAdmin();

  return (
    <div className="w-full space-y-6">
      {/* Die Kopfkarte steht bewusst außerhalb des lg-Gates: der Editor
          darunter ist auf den Desktop beschränkt, der Seitenkopf gilt auf
          allen Breakpoints. */}
      <PageIntro
        kicker="Anmeldungen"
        title="Betreuungsangebote"
        description="Die Angebote, aus denen Eltern in der Anmeldung wählen können."
      />
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
