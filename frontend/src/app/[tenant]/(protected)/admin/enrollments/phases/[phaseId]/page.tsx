"use client";

import { use } from "react";
import { AdminEnrollmentPhaseDetail } from "~/components/enrollment/admin-enrollment-phase-detail";
import { TenantPage } from "~/components/ui/tenant-page";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { useTenantAwarePath } from "~/lib/tenant-path";

interface PageProps {
  readonly params: Promise<{ tenant: string; phaseId: string }>;
}

export default function AdminEnrollmentPhasePage({ params }: PageProps) {
  const { phaseId } = use(params);
  const { isReady } = useRequireAdmin();
  const tenantPath = useTenantAwarePath();

  if (!isReady) {
    return (
      <TenantPage
        title="Anmeldephase"
        back
        backHref={tenantPath("/admin/enrollments")}
        backLabel="Zurück zur Anmeldungs-Übersicht"
        statsLoading
        loading
      />
    );
  }

  return <AdminEnrollmentPhaseDetail phaseId={phaseId} />;
}
