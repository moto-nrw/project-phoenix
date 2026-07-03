"use client";

import { MasterDataReviewList } from "~/components/students/master-data-review-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminChangeRequestsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Änderungsanfragen" />
        <p className="mb-4 text-sm text-gray-600">
          Von Eltern eingereichte Änderungen an Stammdaten, die eine Freigabe
          benötigen.
        </p>
        <MasterDataReviewList />
      </div>
    </div>
  );
}
