"use client";

import { useMemo, useState } from "react";

import { useSession } from "next-auth/react";
import { TrayIcon } from "@phosphor-icons/react/ssr";

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
import { EmptyState } from "~/components/ui/empty-state";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SkeletonRegion, ListSkeleton } from "~/components/ui/page-skeletons";
import {
  canOpenRequestsPage,
  canReviewChangeRequests,
  canReviewStaffAbsenceRequests,
  canReviewStudentDataRequests,
} from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

type AnfragenTabId = "eltern" | "mitarbeitende";

/**
 * Anfragen-Modul (#2429): ein Ort für alle eingereichten Wünsche, mit Reitern
 * nach Herkunft. Der Eltern-Reiter ist die unverändert umgezogene
 * Freigabeansicht (vorher /admin/change-requests); der Mitarbeitende-Reiter
 * ist bis zum Aggregator-Umbau ein Platzhalter und erscheint nur mit
 * Freigaberecht für Abwesenheiten (vacation:approve).
 */
export default function AnfragenPage() {
  // Die Seite öffnet, wer mindestens einen Reiter sehen darf. Die Regeln
  // stehen in change-request-access — dieselben tragen Sidebar-Eintrag,
  // mobile Navigation und Zähler-Badge.
  const { isReady } = useRequirePermission(canOpenRequestsPage);
  const { data: session } = useSession();

  const showElternTab = canReviewChangeRequests(session);
  const showMitarbeitendeTab = canReviewStaffAbsenceRequests(session);

  // Reiter erscheinen nur mit passender Berechtigung; wer nur einen sehen
  // darf, bekommt keine Reiterleiste mit einem einzelnen Eintrag.
  const visibleTabs = useMemo(() => {
    const tabs: { id: AnfragenTabId; label: string }[] = [];
    if (showElternTab) tabs.push({ id: "eltern", label: "Eltern" });
    if (showMitarbeitendeTab)
      tabs.push({ id: "mitarbeitende", label: "Mitarbeitende" });
    return tabs;
  }, [showElternTab, showMitarbeitendeTab]);

  const [selectedTab, setSelectedTab] = useState<AnfragenTabId>("eltern");
  // Fällt die Auswahl aus den sichtbaren Reitern (z. B. Session noch am
  // Laden), gilt der erste sichtbare — so flackert nie ein leerer Inhalt.
  const activeTab = visibleTabs.some((tab) => tab.id === selectedTab)
    ? selectedTab
    : (visibleTabs[0]?.id ?? "eltern");

  if (!isReady) {
    return (
      <div className="-mt-1.5 w-full">
        <PageHeaderWithSearch title="Anfragen" />
        <SkeletonRegion label="Anfragen werden geladen…">
          <ListSkeleton rows={4} avatar={false} />
        </SkeletonRegion>
      </div>
    );
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* Der Seitentitel steht auf dem Desktop in der Breadcrumb der
          Kopfzeile; PageHeaderWithSearch blendet seine Überschrift ab md aus
          (md:hidden), wie auf der vorherigen Freigabeansicht. */}
      <PageHeaderWithSearch
        title="Anfragen"
        tabs={
          visibleTabs.length > 1
            ? {
                items: visibleTabs,
                activeTab,
                onTabChange: (tabId) => setSelectedTab(tabId as AnfragenTabId),
              }
            : undefined
        }
      />
      {activeTab === "mitarbeitende" ? (
        <EmptyState
          icon={<TrayIcon size={48} aria-hidden="true" />}
          title="Anträge von Mitarbeitenden ziehen bald hierhin um"
          description="Urlaubs-, Krank- und Fortbildungsanträge entscheiden Sie bis dahin wie gewohnt auf der Seite Mitarbeiter."
        />
      ) : (
        <ElternTab session={session} />
      )}
    </div>
  );
}

/**
 * Die bisherige Freigabeansicht der Elternanfragen — vier Abschnitte plus
 * Umschalter Offen | Historie, funktional unverändert übernommen (#2429).
 */
function ElternTab({
  session,
}: {
  readonly session: ReturnType<typeof useSession>["data"];
}) {
  // Only the excused-absence queue accepts users:absence. Whoever holds just
  // that permission gets a 403 on the three Stammdaten-side queues, so they are
  // not rendered at all instead of showing three error cards (#2232).
  const showStudentDataQueues = canReviewStudentDataRequests(session);
  // "open" shows the four pending queues, "history" the decided requests of
  // the same four sections. Decided requests used to vanish without a trace,
  // which read as "no requests arrive anymore" whenever a colleague had
  // already worked the queue.
  const [view, setView] = useState<"open" | "history">("open");

  // Stacked sections instead of tabs: both queues are short, and tabs
  // would hide the pending count of the inactive one. Unlike the enrollment
  // queues, these guardian change requests are not tied to Anmeldung and are
  // fully reviewable on mobile — so no DesktopOnlyNotice gate here. Each
  // request renders as a calm card (flex-wrap, mobile-safe), not a bare table.
  return (
    <div className="w-full">
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
