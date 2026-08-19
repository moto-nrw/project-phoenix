"use client";

import { useState } from "react";

import { useSession } from "next-auth/react";

import { CareRequestReviewList } from "~/components/students/care-request-review-list";
import { ExcusedRequestReviewList } from "~/components/students/excused-request-review-list";
import { MasterDataReviewList } from "~/components/students/master-data-review-list";
import { OfferingRequestReviewList } from "~/components/students/offering-request-review-list";
import {
  CareRequestHistoryList,
  ExcusedRequestHistoryList,
  MasterDataHistoryList,
  OfferingRequestHistoryList,
} from "~/components/students/request-history-list";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SkeletonRegion, ListSkeleton } from "~/components/ui/page-skeletons";
import {
  canReviewChangeRequests,
  canReviewStudentDataRequests,
} from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

export default function AdminChangeRequestsPage() {
  // Gated on users:update OR das Paar users:absence + users:read (nicht
  // admin-only), passend zu den Backend-Routen und zur Abwesenheits-Prüfung
  // dahinter. Der Zugriff steht in canReviewChangeRequests — dieselbe Regel
  // trägt Sidebar-Eintrag, Eltern-Übersicht und Zähler-Badge. Das Backend
  // begrenzt jede Warteschlange zusätzlich pro Kind, eine betreuende Person
  // sieht also nur die Anfragen ihrer Gruppe.
  const { isReady } = useRequirePermission(canReviewChangeRequests);
  const { data: session } = useSession();
  // Only the excused-absence queue accepts users:absence. Whoever holds just
  // that permission gets a 403 on the three Stammdaten-side queues, so they are
  // not rendered at all instead of showing three error cards (#2232).
  const showStudentDataQueues = canReviewStudentDataRequests(session);
  // "open" shows the four pending queues, "history" the decided requests of
  // the same four sections. Decided requests used to vanish without a trace,
  // which read as "no requests arrive anymore" whenever a colleague had
  // already worked the queue.
  const [view, setView] = useState<"open" | "history">("open");
  if (!isReady) {
    return (
      <div className="-mt-1.5 w-full">
        <PageHeaderWithSearch title="Änderungsanfragen" />
        <SkeletonRegion label="Änderungsanfragen werden geladen…">
          <ListSkeleton rows={4} avatar={false} />
        </SkeletonRegion>
      </div>
    );
  }

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
      <p className="mb-4 max-w-3xl text-sm text-gray-600">
        {view === "open"
          ? "Von Eltern eingereichte Änderungen, die eine Freigabe benötigen."
          : "Bereits entschiedene Anfragen mit Datum, Person und Begründung."}
      </p>
      <div className="mb-6">
        <SegmentedControl
          items={[
            { value: "open", label: "Offen" },
            { value: "history", label: "Historie" },
          ]}
          value={view}
          onChange={setView}
          ariaLabel="Ansicht wählen"
        />
      </div>

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
            {view === "open" ? (
              <MasterDataReviewList />
            ) : (
              <MasterDataHistoryList />
            )}
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
            {view === "open" ? (
              <CareRequestReviewList />
            ) : (
              <CareRequestHistoryList />
            )}
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
            {view === "open" ? (
              <OfferingRequestReviewList />
            ) : (
              <OfferingRequestHistoryList />
            )}
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
        {view === "open" ? (
          <ExcusedRequestReviewList />
        ) : (
          <ExcusedRequestHistoryList />
        )}
      </section>
    </div>
  );
}
