"use client";

import { useTranslations } from "next-intl";
import { MasterDataReviewList } from "~/components/students/master-data-review-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminChangeRequestsPage() {
  const t = useTranslations("staffMasterDataReview");
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title={t("title")} />
        <p className="mb-4 text-sm text-gray-600">{t("subtitle")}</p>
        <MasterDataReviewList />
      </div>
    </div>
  );
}
