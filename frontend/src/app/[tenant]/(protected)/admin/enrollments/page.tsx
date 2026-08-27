"use client";

import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { PhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";

export default function AdminEnrollmentsPage() {
  const { isReady } = useRequireAdmin();

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Überblick" />
      {isReady ? (
        <>
          <DesktopOnlyNotice />
          <PhaseExpiryWarnings className="mt-4" />
          <div className="hidden lg:block">
            <AdminEnrollmentsList />
          </div>
        </>
      ) : (
        <SkeletonRegion label="Anmeldungen werden geladen" className="mt-4">
          <ListSkeleton rows={6} avatar={false} />
        </SkeletonRegion>
      )}
    </div>
  );
}
