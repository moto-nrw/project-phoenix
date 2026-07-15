"use client";

import { CareRequestReviewList } from "~/components/students/care-request-review-list";
import { ExcusedRequestReviewList } from "~/components/students/excused-request-review-list";
import { MasterDataReviewList } from "~/components/students/master-data-review-list";
import { Loading } from "~/components/ui/loading";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

export default function AdminChangeRequestsPage() {
  // Gated on users:update (not admin-only): the same permission as editing a
  // child directly. The backend scopes both queues per child, so a supervisor
  // sees only their own group's requests.
  const { isReady } = useRequirePermission("users:update");
  if (!isReady) return <Loading fullPage={false} />;

  // Two stacked sections instead of tabs: both queues are short, and tabs
  // would hide the pending count of the inactive one. Unlike the enrollment
  // queues, these guardian change requests are not tied to Anmeldung and are
  // fully reviewable on mobile — so no DesktopOnlyNotice gate here. Each
  // request renders as a calm card (flex-wrap, mobile-safe), not a bare table.
  return (
    <div className="-mt-1.5 w-full">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold text-gray-900">
          Änderungsanfragen
        </h1>
        <p className="mt-1 text-sm text-gray-600">
          Von Eltern eingereichte Änderungen, die eine Freigabe benötigen.
        </p>
      </header>

      <section className="mb-8">
        <h2 className="text-base font-semibold text-gray-900">Stammdaten</h2>
        <p className="mt-1 mb-3 text-sm text-gray-600">
          Anfragen zu Name, Geburtsdatum und erlaubten Abholarten des Kindes.
          Freigeben übernimmt die Änderung direkt in die Stammdaten.
        </p>
        <MasterDataReviewList />
      </section>

      <section className="mb-8">
        <h2 className="text-base font-semibold text-gray-900">
          Betreuungszeiten
        </h2>
        <p className="mt-1 mb-3 text-sm text-gray-600">
          Anfragen zu dauerhaften Bring- und Abholzeiten sowie zur Abholart.
          Freigeben übernimmt die Änderungen direkt in den Wochenplan des
          Kindes.
        </p>
        <CareRequestReviewList />
      </section>

      <section>
        <h2 className="text-base font-semibold text-gray-900">
          Entschuldigte Abmeldungen
        </h2>
        <p className="mt-1 mb-3 text-sm text-gray-600">
          Von Eltern gemeldete entschuldigte Abwesenheiten, die eine Bestätigung
          benötigen. Das Kind bleibt bis zur Freigabe eingeplant. Freigeben
          meldet das Kind für die gemeldeten Tage ab.
        </p>
        <ExcusedRequestReviewList />
      </section>
    </div>
  );
}
