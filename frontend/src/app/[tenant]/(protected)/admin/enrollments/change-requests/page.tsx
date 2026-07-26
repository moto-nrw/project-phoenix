"use client";

import { AdminEnrollmentChangeRequestsList } from "~/components/enrollment/admin-enrollment-change-requests";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminEnrollmentChangeRequestsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

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
