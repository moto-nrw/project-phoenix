"use client";

import { CareRequestReviewList } from "~/components/students/care-request-review-list";
import { MasterDataReviewList } from "~/components/students/master-data-review-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function AdminChangeRequestsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  // Two stacked sections instead of tabs: both queues are short, and tabs
  // would hide the pending count of the inactive one.
  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Änderungsanfragen" />
        <p className="mb-4 text-sm text-gray-600">
          Von Eltern eingereichte Änderungen, die eine Freigabe benötigen.
        </p>
        <section className="mb-8">
          <h2 className="mb-2 text-base font-semibold text-gray-900">
            Stammdaten
          </h2>
          <p className="mb-3 text-sm text-gray-600">
            Anfragen zu Name, Geburtsdatum und erlaubten Abholarten des Kindes.
            Freigeben übernimmt die Änderung direkt in die Stammdaten.
          </p>
          <MasterDataReviewList />
        </section>
        <section>
          <h2 className="mb-2 text-base font-semibold text-gray-900">
            Betreuungszeiten
          </h2>
          <p className="mb-3 text-sm text-gray-600">
            Anfragen zu dauerhaften Bring- und Abholzeiten sowie zur Abholart.
            Freigeben übernimmt die Änderungen direkt in den Wochenplan des
            Kindes.
          </p>
          <CareRequestReviewList />
        </section>
      </div>
    </div>
  );
}
