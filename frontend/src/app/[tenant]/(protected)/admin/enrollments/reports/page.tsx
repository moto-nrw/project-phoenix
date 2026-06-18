"use client";

import { EnrollmentReportsPage } from "~/components/enrollment/enrollment-reports-page";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminEnrollmentReportsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Auswertung" />
        <EnrollmentReportsPage />
      </div>
    </div>
  );
}
