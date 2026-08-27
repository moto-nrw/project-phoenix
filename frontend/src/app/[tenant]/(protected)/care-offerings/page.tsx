"use client";

import { useCallback, useState } from "react";
import { CareOfferingsEditor } from "~/components/enrollment/care-offerings-editor";
import { PageIntro } from "~/components/ui/page-intro";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { Skeleton } from "~/components/ui/skeleton";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function CareOfferingsPage() {
  const { isReady } = useRequireAdmin();
  // Statuszeile des Seitenkopfs: der Editor meldet die Zahlen seines
  // Katalogs, damit die Kopfkarte ohne zweiten Request auskommt.
  const [summary, setSummary] = useState<{
    total: number;
    active: number;
  } | null>(null);
  const handleSummaryChange = useCallback(
    (next: { total: number; active: number } | null) => setSummary(next),
    [],
  );

  return (
    <div className="w-full space-y-6">
      {/* Die Kopfkarte steht bewusst außerhalb des lg-Gates: der Editor
          darunter ist auf den Desktop beschränkt, der Seitenkopf gilt auf
          allen Breakpoints. */}
      <PageIntro
        kicker="Anmeldungen"
        title="Betreuungsangebote"
        description={
          summary ? (
            `${summary.total} ${summary.total === 1 ? "Angebot" : "Angebote"} · ${summary.active} aktiv`
          ) : (
            <Skeleton className="h-4 w-40" />
          )
        }
      />
      <DesktopOnlyNotice />
      {isReady ? (
        <div className="hidden lg:block">
          <CareOfferingsEditor onSummaryChange={handleSummaryChange} />
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
