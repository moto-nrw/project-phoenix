"use client";

import { useDeferredValue, useMemo, useState } from "react";

import { useSession } from "next-auth/react";
import type { DateRange } from "react-day-picker";

import { AggregatedRequestList } from "~/components/students/aggregated-request-list";
import type { AggregatedRequestFilters } from "~/components/students/aggregated-request-list";
import { StaffAbsenceRequestList } from "~/components/staff/staff-absence-request-list";
import type { StaffAbsenceRequestFilters } from "~/components/staff/staff-absence-request-list";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
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
  canOpenParentRequestsTab,
  canOpenRequestsPage,
  canReviewChangeRequests,
  canReviewCareWithdrawals,
  canReviewEnrollmentChangeRequests,
  canReviewStaffAbsenceRequests,
  canReviewStudentDataRequests,
} from "~/lib/change-request-access";
import { ABSENCE_TYPE_LABEL } from "~/lib/absence-helpers";
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
  { value: "excused", label: "Abwesenheiten" },
];

// Anmeldungsänderungen sieht nur, wer sie auch entscheiden darf (#2435).
const ENROLLMENT_TYPE_OPTION: {
  value: AggregatedRequestType;
  label: string;
} = { value: "enrollment", label: "Anmeldung" };

const CARE_WITHDRAWAL_TYPE_OPTION: {
  value: AggregatedRequestType;
  label: string;
} = { value: "care_withdrawal", label: "Abmeldungen" };

// Direkt-Korrekturen sind keine Anfragen: sie gibt es nur in der Historie,
// also auch den Filter nur dort (#2436).
const HISTORY_ONLY_TYPE_OPTIONS: readonly {
  value: AggregatedRequestType;
  label: string;
}[] = [{ value: "direct_correction", label: "Direkt-Korrekturen" }];

// Die Abwesenheitsarten, die Mitarbeitende beantragen können, mit den Namen
// aus der geteilten Beschriftungstabelle. Freizeitausgleich fehlt bewusst: den
// trägt die Zeiterfassung ein, er läuft nicht über eine Freigabe.
const ABSENCE_TYPE_OPTIONS: readonly { value: string; label: string }[] = [
  "vacation",
  "sick",
  "training",
  "other",
].map((value) => ({ value, label: ABSENCE_TYPE_LABEL[value] ?? value }));

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
 * Mitarbeitende-Reiter zeigt seit #2433 die Abwesenheitsanträge (offen und
 * Historie) und erscheint nur mit Freigaberecht dafür (vacation:approve).
 */
export default function AnfragenPage() {
  // Die Seite öffnet, wer mindestens einen Reiter sehen darf. Die Regeln
  // stehen in change-request-access — dieselben tragen Sidebar-Eintrag,
  // mobile Navigation und Zähler-Badge.
  const { isReady } = useRequirePermission(canOpenRequestsPage);
  const { data: session } = useSession();

  const showElternTab = canOpenParentRequestsTab(session);
  // Anmeldungsänderungen hängen an config:manage und kommen aus einem eigenen
  // Endpunkt (#2435); ohne das Recht bleiben Quelle und Filteroption weg.
  const showEnrollmentRequests = canReviewEnrollmentChangeRequests(session);
  const showCareWithdrawals = canReviewCareWithdrawals(session);
  const showMitarbeitendeTab = canReviewStaffAbsenceRequests(session);
  // Der Aggregator über die vier Kinderdaten-Arten verlangt users:update oder
  // users:absence — ohne eines von beiden darf die Quelle gar nicht angefragt
  // werden.
  const showAggregatedRequests = canReviewChangeRequests(session);
  const showStudentDataRequests = canReviewStudentDataRequests(session);
  // Wer nur die Entschuldigungs-Warteschlange hält, sieht ohnehin nur diese
  // eine Art — der Art-Filter wäre eine Liste toter Optionen (#2232).
  const showTypeFilter = showStudentDataRequests || showEnrollmentRequests;

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
  const [absenceTypeFilter, setAbsenceTypeFilter] = useState<string[]>([]);
  const [statusFilter, setStatusFilter] = useState<AggregatedRequestStatus[]>(
    [],
  );
  const [dateRange, setDateRange] = useState<DateRange | undefined>(undefined);

  // Stabil memoisiert: die Liste lädt bei jeder Identitätsänderung neu.
  const filters: AggregatedRequestFilters = useMemo(
    () => ({
      search: deferredSearch,
      includeAggregated: showAggregatedRequests,
      includeEnrollment: showEnrollmentRequests,
      includeCareWithdrawals: showCareWithdrawals,
      types:
        view === "history"
          ? typeFilter.filter((type) => type !== "care_withdrawal")
          : typeFilter.filter((type) => type !== "direct_correction"),
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
    [
      deferredSearch,
      showAggregatedRequests,
      showEnrollmentRequests,
      showCareWithdrawals,
      typeFilter,
      statusFilter,
      dateRange,
      view,
    ],
  );

  const staffFilters: StaffAbsenceRequestFilters = useMemo(
    () => ({ search: deferredSearch, types: absenceTypeFilter }),
    [deferredSearch, absenceTypeFilter],
  );

  // Die im aktuellen Umschalter-Zustand wählbaren Anfragearten. Eine Quelle
  // für Filterknöpfe UND Filter-Chips: was hier fehlt, ist auch als Chip weg.
  const typeOptions = useMemo(
    () => [
      ...(showStudentDataRequests ? REQUEST_TYPE_OPTIONS : []),
      ...(showEnrollmentRequests ? [ENROLLMENT_TYPE_OPTION] : []),
      ...(view === "open" && showCareWithdrawals
        ? [CARE_WITHDRAWAL_TYPE_OPTION]
        : []),
      ...(view === "history" && showStudentDataRequests
        ? HISTORY_ONLY_TYPE_OPTIONS
        : []),
    ],
    [
      showStudentDataRequests,
      showEnrollmentRequests,
      showCareWithdrawals,
      view,
    ],
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
            options: typeOptions.map((option) => ({ ...option })),
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
  }, [showTypeFilter, view, typeOptions, typeFilter, statusFilter, dateRange]);

  const activeFilters = useMemo(() => {
    const chips: ActiveFilter[] = [];
    for (const type of typeFilter) {
      // Eine Art, die in dieser Ansicht nicht wählbar ist, trägt auch keinen
      // Chip — in der Arbeitsliste betrifft das die Direkt-Korrekturen.
      const label = typeOptions.find((option) => option.value === type)?.label;
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
  }, [typeOptions, typeFilter, statusFilter, dateRange, view]);

  const staffFilterConfigs = useMemo<FilterConfig[]>(
    () => [
      {
        id: "abwesenheitsart",
        label: "Art",
        type: "buttons",
        multiSelect: true,
        value: absenceTypeFilter,
        onChange: (value) =>
          setAbsenceTypeFilter(
            (Array.isArray(value) ? value : [value]) as string[],
          ),
        options: ABSENCE_TYPE_OPTIONS.map((option) => ({ ...option })),
      },
    ],
    [absenceTypeFilter],
  );

  const staffActiveFilters = useMemo<ActiveFilter[]>(
    () =>
      absenceTypeFilter.map((type) => ({
        id: `abwesenheitsart-${type}`,
        label:
          ABSENCE_TYPE_OPTIONS.find((option) => option.value === type)?.label ??
          type,
        onRemove: () =>
          setAbsenceTypeFilter((prev) =>
            prev.filter((value) => value !== type),
          ),
      })),
    [absenceTypeFilter],
  );

  const clearAllFilters = () => {
    setTypeFilter([]);
    setAbsenceTypeFilter([]);
    setStatusFilter([]);
    setDateRange(undefined);
  };

  const handleElternViewChange = (nextView: "open" | "history") => {
    if (nextView === "open") {
      setTypeFilter((previous) =>
        previous.filter((type) => type !== "direct_correction"),
      );
    }
    setView(nextView);
  };

  if (!isReady) {
    return (
      <div className="w-full space-y-6">
        <PageIntro title="Anfragen" />
        <SkeletonRegion label="Anfragen werden geladen…">
          <ListSkeleton rows={4} avatar={false} />
        </SkeletonRegion>
      </div>
    );
  }

  const staffActive = activeTab === "mitarbeitende";

  const viewSwitcher = (
    <SegmentedControl
      items={[
        { value: "open", label: "Offen" },
        { value: "history", label: "Historie" },
      ]}
      value={view}
      onChange={staffActive ? setView : handleElternViewChange}
      ariaLabel="Ansicht wählen"
    />
  );

  // Ohne Reiter trüge die Reiterzeile nur den Umschalter — eine Zeile mit
  // einem einzigen Element direkt unter der Kopfkarte. Er wandert dann in die
  // Titelzeile der Kopfkarte.
  const hasTabs = visibleTabs.length > 1;

  return (
    <div className="w-full space-y-6">
      {/* Kopfkarte wie in der Eltern-App: Kicker, Titel und Erklärtext in EINER
          Karte, auf allen Breakpoints. Reiter, Suche, Filter und der
          Ansichts-Umschalter bleiben in PageHeaderWithSearch; title="" blendet
          dort nur die eigene (jetzt doppelte) Titelzeile aus. */}
      <PageIntro
        title="Anfragen"
        actions={hasTabs ? undefined : viewSwitcher}
      />
      <PageHeaderWithSearch
        title=""
        tabs={
          hasTabs
            ? {
                items: visibleTabs,
                activeTab,
                // Der Suchbegriff des einen Reiters passt nie zum anderen
                // (Kind gegen Teammitglied), also beim Wechsel leeren.
                onTabChange: (tabId) => {
                  setSelectedTab(tabId as AnfragenTabId);
                  setSearchTerm("");
                },
              }
            : undefined
        }
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: staffActive
            ? "Teammitglied suchen..."
            : "Kind suchen...",
        }}
        filters={
          staffActive
            ? staffFilterConfigs
            : filterConfigs.length > 0
              ? filterConfigs
              : undefined
        }
        activeFilters={staffActive ? staffActiveFilters : activeFilters}
        onClearAllFilters={clearAllFilters}
        filterVariant="quiet"
        activeFilterDisplay="count"
        // Der Umschalter sitzt auf einer Höhe mit den Reitern: beides ist
        // eine Auswahl, was die Liste zeigt. `tabsRowAction` hält ihn auf
        // jeder Breite dort — `actionButton` wandert auf Mobil in die
        // Titelzeile, `primaryAction` rendert nur im Desktop-Zweig.
        tabsRowAction={hasTabs ? viewSwitcher : undefined}
      />
      {staffActive ? (
        <MitarbeitendeTab view={view} filters={staffFilters} />
      ) : (
        <ElternTab view={view} filters={filters} />
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
  filters,
}: Readonly<{
  view: "open" | "history";
  filters: AggregatedRequestFilters;
}>) {
  return (
    <div className="w-full">
      {/* key={view}: die Liste mountet beim Umschalten frisch, wie zuvor die
          Einzelsektionen — so braucht die Historie keine Refresh-Listener. */}
      <AggregatedRequestList key={view} view={view} filters={filters} />
    </div>
  );
}

/**
 * Der Mitarbeitende-Reiter: Abwesenheitsanträge des Teams, offen entscheiden
 * oder in der Historie nachschlagen. Genehmigen, Ablehnen und Rückfrage
 * laufen über dieselben Modals wie zuvor auf der Mitarbeiter-Seite.
 */
function MitarbeitendeTab({
  view,
  filters,
}: Readonly<{
  view: "open" | "history";
  filters: StaffAbsenceRequestFilters;
}>) {
  return (
    <div className="w-full">
      {/* key={view}: die Liste mountet beim Umschalten frisch. */}
      <StaffAbsenceRequestList key={view} view={view} filters={filters} />
    </div>
  );
}
