"use client";

import { AdminEnrollmentChangeRequestsList } from "~/components/enrollment/admin-enrollment-change-requests";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminEnrollmentChangeRequestsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady)
    return (
      <SkeletonRegion label="Änderungsanfragen werden geladen">
        <ListSkeleton rows={6} avatar={false} />
      </SkeletonRegion>
    );

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Änderungsanfragen" />
        <AdminEnrollmentChangeRequestsList />
      </div>
    </div>
  );
}
