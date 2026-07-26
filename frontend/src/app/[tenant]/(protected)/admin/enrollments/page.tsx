"use client";

import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminEnrollmentsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

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
