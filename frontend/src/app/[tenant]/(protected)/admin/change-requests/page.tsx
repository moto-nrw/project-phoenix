"use client";

import { useSession } from "next-auth/react";

import { CareRequestReviewList } from "~/components/students/care-request-review-list";
import { ExcusedRequestReviewList } from "~/components/students/excused-request-review-list";
import { MasterDataReviewList } from "~/components/students/master-data-review-list";
import { OfferingRequestReviewList } from "~/components/students/offering-request-review-list";
import { Loading } from "~/components/ui/loading";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { canReviewStudentDataRequests } from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

export default function AdminChangeRequestsPage() {
  // Gated on users:update OR users:absence (not admin-only), matching the
  // backend routes. The backend scopes every queue per child, so a supervisor
  // sees only their own group's requests.
  const { isReady } = useRequirePermission(["users:update", "users:absence"]);
  const { data: session } = useSession();
  // Only the excused-absence queue accepts users:absence. Whoever holds just
  // that permission gets a 403 on the three Stammdaten-side queues, so they are
  // not rendered at all instead of showing three error cards (#2232).
  const showStudentDataQueues = canReviewStudentDataRequests(session);
  if (!isReady) return <Loading fullPage={false} />;

  // Stacked sections instead of tabs: both queues are short, and tabs
  // would hide the pending count of the inactive one. Unlike the enrollment
  // queues, these guardian change requests are not tied to Anmeldung and are
  // fully reviewable on mobile — so no DesktopOnlyNotice gate here. Each
  // request renders as a calm card (flex-wrap, mobile-safe), not a bare table.
  return (
    <div className="-mt-1.5 w-full">
      {/* Der Seitentitel steht auf dem Desktop in der Breadcrumb der Kopfzeile.
          PageHeaderWithSearch blendet seine Überschrift deshalb ab md ein
          (md:hidden), genau wie auf der Schwesterseite Konto-Anfragen. Ein
          eigenes h1 hätte den Titel zweimal untereinander gezeigt. */}
      <PageHeaderWithSearch title="Änderungsanfragen" />
      <p className="mb-6 max-w-3xl text-sm text-gray-600">
        Von Eltern eingereichte Änderungen, die eine Freigabe benötigen.
      </p>

      {showStudentDataQueues && (
        <>
          <section className="mb-8">
            <h2 className="text-base font-semibold text-gray-900">
              Stammdaten
            </h2>
            <p className="mt-1 mb-3 text-sm text-gray-600">
              Anfragen zu Name, Geburtsdatum und erlaubten Abholarten des
              Kindes. Freigeben übernimmt die Änderung direkt in die Stammdaten.
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

          <section className="mb-8">
            <h2 className="text-base font-semibold text-gray-900">
              Betreuungsangebote und AGs
            </h2>
            <p className="mt-1 mb-3 text-sm text-gray-600">
              Anfragen zu den gebuchten Betreuungsangeboten. Freigeben stellt
              das Kind zum gewünschten Datum um: Bisheriges endet an diesem Tag,
              Neues beginnt dann. Vergangene Zeiträume bleiben unverändert.
            </p>
            <OfferingRequestReviewList />
          </section>
        </>
      )}

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
