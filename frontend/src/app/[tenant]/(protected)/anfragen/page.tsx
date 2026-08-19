"use client";

import { useDeferredValue, useMemo, useState } from "react";

import { useSession } from "next-auth/react";
import type { DateRange } from "react-day-picker";
import { TrayIcon } from "@phosphor-icons/react/ssr";

import { AggregatedRequestList } from "~/components/students/aggregated-request-list";
import type { AggregatedRequestFilters } from "~/components/students/aggregated-request-list";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { SkeletonRegion, ListSkeleton } from "~/components/ui/page-skeletons";
import type {
  AggregatedRequestStatus,
  AggregatedRequestType,
} from "~/lib/change-request-list-api";
import {
  canOpenRequestsPage,
  canReviewChangeRequests,
  canReviewStaffAbsenceRequests,
  canReviewStudentDataRequests,
} from "~/lib/change-request-access";
import { toISODate } from "~/lib/date-helpers";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

type AnfragenTabId = "eltern" | "mitarbeitende";

const REQUEST_TYPE_OPTIONS: readonly {
  value: AggregatedRequestType;
  label: string;
}[] = [
  { value: "master_data", label: "Stammdaten" },
  { value: "care_schedule", label: "Betreuungszeiten" },
  { value: "offering", label: "Angebote und AGs" },
  { value: "excused", label: "Entschuldigungen" },
];

const STATUS_OPTIONS: readonly {
  value: AggregatedRequestStatus;
  label: string;
}[] = [
  { value: "approved", label: "Angenommen" },
  { value: "rejected", label: "Abgelehnt" },
  { value: "withdrawn", label: "Zurückgezogen" },
];

/**
 * Anfragen-Modul (#2429): ein Ort für alle eingereichten Wünsche, mit Reitern
 * nach Herkunft. Der Eltern-Reiter zeigt seit #2432 EINE Liste über alle vier
 * Anfragearten (statt vier Abschnitte), mit serverseitiger Namenssuche,
 * Art-Filter und — in der Historie — Status- und Zeitraum-Filter. Der
 * Mitarbeitende-Reiter ist bis zum Aggregator-Umbau ein Platzhalter und
 * erscheint nur mit Freigaberecht für Abwesenheiten (vacation:approve).
 */
export default function AnfragenPage() {
  // Die Seite öffnet, wer mindestens einen Reiter sehen darf. Die Regeln
  // stehen in change-request-access — dieselben tragen Sidebar-Eintrag,
  // mobile Navigation und Zähler-Badge.
  const { isReady } = useRequirePermission(canOpenRequestsPage);
  const { data: session } = useSession();

  const showElternTab = canReviewChangeRequests(session);
  const showMitarbeitendeTab = canReviewStaffAbsenceRequests(session);
  // Nur die Entschuldigungs-Warteschlange akzeptiert users:absence. Wer nur
  // das hält, sieht ohnehin nur diese Art — der Art-Filter wäre eine Liste
  // aus drei toten Optionen (#2232).
  const showTypeFilter = canReviewStudentDataRequests(session);

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

  // Such- und Filterzustand des Eltern-Reiters lebt hier, weil Suche und
  // Filterleiste in der Kopfzeile (PageHeaderWithSearch) hängen.
  const [view, setView] = useState<"open" | "history">("open");
  const [searchTerm, setSearchTerm] = useState("");
  const deferredSearch = useDeferredValue(searchTerm);
  const [typeFilter, setTypeFilter] = useState<AggregatedRequestType[]>([]);
  const [statusFilter, setStatusFilter] = useState<AggregatedRequestStatus[]>(
    [],
  );
  const [dateRange, setDateRange] = useState<DateRange | undefined>(undefined);

  // Stabil memoisiert: die Liste lädt bei jeder Identitätsänderung neu.
  const filters: AggregatedRequestFilters = useMemo(
    () => ({
      search: deferredSearch,
      types: typeFilter,
      statuses: view === "history" ? statusFilter : [],
      from:
        view === "history" && dateRange?.from
          ? toISODate(dateRange.from)
          : undefined,
      to:
        view === "history" && dateRange?.to
          ? toISODate(dateRange.to)
          : undefined,
    }),
    [deferredSearch, typeFilter, statusFilter, dateRange, view],
  );

  const filterConfigs = useMemo(() => {
    const typeConfig: FilterConfig[] = showTypeFilter
      ? [
          {
            id: "art",
            label: "Anfrageart",
            type: "buttons",
            multiSelect: true,
            value: typeFilter,
            onChange: (value) =>
              setTypeFilter(
                (Array.isArray(value)
                  ? value
                  : [value]) as AggregatedRequestType[],
              ),
            options: REQUEST_TYPE_OPTIONS.map((option) => ({ ...option })),
          },
        ]
      : [];
    const historyConfigs: FilterConfig[] =
      view === "history"
        ? [
            {
              id: "status",
              label: "Status",
              type: "buttons",
              multiSelect: true,
              value: statusFilter,
              onChange: (value) =>
                setStatusFilter(
                  (Array.isArray(value)
                    ? value
                    : [value]) as AggregatedRequestStatus[],
                ),
              options: STATUS_OPTIONS.map((option) => ({ ...option })),
            },
            {
              id: "zeitraum",
              label: "Zeitraum",
              type: "custom",
              value: "",
              onChange: () => undefined,
              options: [],
              render: (
                <DateRangePicker
                  value={dateRange}
                  onChange={setDateRange}
                  className="w-fit"
                />
              ),
            },
          ]
        : [];
    return [...typeConfig, ...historyConfigs];
  }, [showTypeFilter, view, typeFilter, statusFilter, dateRange]);

  const activeFilters = useMemo(() => {
    const chips: ActiveFilter[] = [];
    for (const type of typeFilter) {
      const label = REQUEST_TYPE_OPTIONS.find(
        (option) => option.value === type,
      )?.label;
      if (!label) continue;
      chips.push({
        id: `art-${type}`,
        label,
        onRemove: () =>
          setTypeFilter((prev) => prev.filter((value) => value !== type)),
      });
    }
    if (view === "history") {
      for (const status of statusFilter) {
        const label = STATUS_OPTIONS.find(
          (option) => option.value === status,
        )?.label;
        if (!label) continue;
        chips.push({
          id: `status-${status}`,
          label,
          onRemove: () =>
            setStatusFilter((prev) => prev.filter((value) => value !== status)),
        });
      }
      if (dateRange?.from ?? dateRange?.to) {
        chips.push({
          id: "zeitraum",
          label: "Zeitraum",
          onRemove: () => setDateRange(undefined),
        });
      }
    }
    return chips;
  }, [typeFilter, statusFilter, dateRange, view]);

  const clearAllFilters = () => {
    setTypeFilter([]);
    setStatusFilter([]);
    setDateRange(undefined);
  };

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

  const elternActive = activeTab === "eltern";

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
        search={
          elternActive
            ? {
                value: searchTerm,
                onChange: setSearchTerm,
                placeholder: "Kind suchen...",
              }
            : undefined
        }
        filters={
          elternActive && filterConfigs.length > 0 ? filterConfigs : undefined
        }
        activeFilters={elternActive ? activeFilters : undefined}
        onClearAllFilters={elternActive ? clearAllFilters : undefined}
        filterVariant="quiet"
        activeFilterDisplay="count"
      />
      {activeTab === "mitarbeitende" ? (
        <EmptyState
          icon={<TrayIcon size={48} aria-hidden="true" />}
          title="Anträge von Mitarbeitenden ziehen bald hierhin um"
          description="Urlaubs-, Krank- und Fortbildungsanträge entscheiden Sie bis dahin wie gewohnt auf der Seite Mitarbeiter."
        />
      ) : (
        <ElternTab view={view} onViewChange={setView} filters={filters} />
      )}
    </div>
  );
}

/**
 * Der Eltern-Reiter: Umschalter Offen | Historie über EINER aggregierten
 * Liste aller vier Anfragearten. Entscheiden funktioniert direkt aus der
 * Liste; die Historie zeigt Datum, Person und Begründung jeder Entscheidung.
 */
function ElternTab({
  view,
  onViewChange,
  filters,
}: Readonly<{
  view: "open" | "history";
  onViewChange: (view: "open" | "history") => void;
  filters: AggregatedRequestFilters;
}>) {
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
          onChange={onViewChange}
          ariaLabel="Ansicht wählen"
        />
      </div>
      {/* key={view}: die Liste mountet beim Umschalten frisch, wie zuvor die
          Einzelsektionen — so braucht die Historie keine Refresh-Listener. */}
      <AggregatedRequestList key={view} view={view} filters={filters} />
    </div>
  );
}
