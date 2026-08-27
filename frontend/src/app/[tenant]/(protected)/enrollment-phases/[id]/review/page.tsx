"use client";

import { use } from "react";
import { RolloverReviewQueue } from "~/components/enrollment/rollover-review-queue";
import { TenantPage } from "~/components/ui/tenant-page";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { useTenantAwarePath } from "~/lib/tenant-path";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

/**
 * Admin review queue for rolled-over enrollments that could not be
 * carried forward automatically (grade above cap, missing grade, etc).
 */
export default function RolloverReviewPage({ params }: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequireAdmin();
  const tenantPath = useTenantAwarePath();

  // Titel, Statuszeile und Zurück-Knopf trägt die Prüfliste selbst.
  if (!isReady) {
    return (
      <TenantPage
        title="Prüfliste"
        back
        backHref={tenantPath("/enrollment-phases")}
        backLabel="Zurück zu den Anmeldephasen"
        statsLoading
        loading
      />
    );
  }

  return <RolloverReviewQueue phaseID={id} />;
}
