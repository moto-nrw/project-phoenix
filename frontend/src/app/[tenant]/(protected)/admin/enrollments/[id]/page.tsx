"use client";

import { use } from "react";
import { AdminEnrollmentDetail } from "~/components/enrollment/admin-enrollment-detail";
import { TenantPage } from "~/components/ui/tenant-page";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { useTenantAwarePath } from "~/lib/tenant-path";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

export default function AdminEnrollmentDetailPage({ params }: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequireAdmin();
  const tenantPath = useTenantAwarePath();

  if (!isReady) {
    return (
      <TenantPage
        title="Anmeldung"
        back
        backHref={tenantPath("/admin/enrollments")}
        backLabel="Zurück zur Anmeldungs-Übersicht"
        statsLoading
        loading
      />
    );
  }

  return <AdminEnrollmentDetail requestId={id} />;
}
