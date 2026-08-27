"use client";

import { use } from "react";
import { AdminEnrollmentChangeRequestDetail } from "~/components/enrollment/admin-enrollment-change-requests";
import { TenantPage } from "~/components/ui/tenant-page";
import { canReviewEnrollmentChangeRequests } from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { useTenantAwarePath } from "~/lib/tenant-path";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

export default function AdminEnrollmentChangeRequestDetailPage({
  params,
}: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequirePermission(canReviewEnrollmentChangeRequests);
  const tenantPath = useTenantAwarePath();

  if (!isReady) {
    return (
      <TenantPage
        title="Änderungsanfrage"
        back
        backHref={tenantPath("/admin/enrollments")}
        backLabel="Zurück zur Anmeldungs-Übersicht"
        statsLoading
        loading
      />
    );
  }

  return <AdminEnrollmentChangeRequestDetail changeRequestId={id} />;
}
