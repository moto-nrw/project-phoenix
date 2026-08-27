"use client";

import { useCallback, useState } from "react";
import { CareOfferingsEditor } from "~/components/enrollment/care-offerings-editor";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { TenantPage } from "~/components/ui/tenant-page";
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
    <TenantPage
      title="Betreuungsangebote"
      stats={
        summary
          ? `${summary.total} ${summary.total === 1 ? "Angebot" : "Angebote"} · ${summary.active} aktiv`
          : null
      }
      statsLoading={summary === null}
    >
      {/* Der Editor darunter ist auf den Desktop beschränkt; die Kopfkarte
          gilt auf allen Breakpoints. */}
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
    </TenantPage>
  );
}
