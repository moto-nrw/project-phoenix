"use client";

import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminEnrollmentsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady)
    return (
      <SkeletonRegion label="Anmeldungen werden geladen">
        <ListSkeleton rows={6} avatar={false} />
      </SkeletonRegion>
    );

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Überblick" />
        <AdminEnrollmentsList />
      </div>
    </div>
  );
}
