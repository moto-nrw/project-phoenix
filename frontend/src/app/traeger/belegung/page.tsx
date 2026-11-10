"use client";

// Belegung des Träger-Portals (#4676): wie viele Kinder jede Schule, jede
// OGS-Gruppe und jede Betreuungsform heute hat, damit das Träger-Büro planen
// kann. Oben die Summe und die freien Plätze, darunter drei Reiter: je Schule,
// je Gruppe und je Betreuungsform. Ein laufendes Angebot steht auch ohne
// Buchung da, mit allen Plätzen frei (#5977). Nur Zahlen und die Namen von Gruppen und Angeboten, keine
// Kinder. Der Export enthält dieselben Zahlen als CSV.

import { type MouseEvent, type ReactNode, useMemo, useState } from "react";
import { School } from "lucide-react";
import { OfferingOccupancyCell } from "~/components/enrollment/offering-occupancy";
import { offeringOccupancy } from "~/lib/care-offering-booking-stats";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import {
  ExportMenu,
  type ExportMenuOption,
  exportFormatOptions,
} from "~/components/ui/export-menu";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import NavigationLink from "~/components/ui/navigation-link";
import { ProgressBar } from "~/components/ui/progress-bar";
import { StatCard } from "~/components/ui/stat-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  formatShare,
  occupancyRatio,
  matchesSchool,
  occupancyCareFormRows,
  occupancyCareFormSums,
  occupancyGroupRows,
  occupancyPlaces,
  occupancyTotals,
  type OccupancyCareFormRow,
  type OccupancyCareFormSum,
  type OccupancyGroupRow,
  type OccupancyPlaces,
  type OccupancyTotals,
} from "~/components/traeger/occupancy-model";
import { useTraegerOccupancy } from "~/components/traeger/use-traeger-occupancy";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { carePlanConcept } from "~/lib/concepts/carePlan";
import { childrenConcept } from "~/lib/concepts/children";
import { groupsConcept } from "~/lib/concepts/groups";
import { utilizationConcept } from "~/lib/concepts/utilization";
import { formatBerlinTime, formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  downloadTraegerOccupancy,
  type TraegerSchoolOccupancy,
} from "~/lib/traeger-occupancy-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerOccupancyPage" });

const EXPORT_FORMATS: readonly ExportMenuOption[] = exportFormatOptions([
  "csv",
]);

type View = "schulen" | "gruppen" | "betreuungsformen";

const VIEWS: readonly { value: View; label: string }[] = [
  { value: "schulen", label: "Schulen" },
  { value: "gruppen", label: "Gruppen" },
  { value: "betreuungsformen", label: "Betreuungsformen" },
];

function SchoolCell({ school }: Readonly<{ school: TraegerSchoolOccupancy }>) {
  return (
    <div className="min-w-0">
      <NavigationLink
        href={traegerPath(`/traeger/schulen/${school.id}`)}
        className="font-medium text-gray-900 hover:underline"
        onClick={(event: MouseEvent<HTMLAnchorElement>) =>
          event.stopPropagation()
        }
      >
        {school.name}
      </NavigationLink>
      {school.city ? (
        <span className="block text-xs text-gray-500 max-md:hidden">
          {school.city}
        </span>
      ) : null}
    </div>
  );
}

/** „92 von 112“ mit einem Balken darunter. */
function ShareCell({
  part,
  whole,
  label,
}: Readonly<{ part: number; whole: number; label: string }>) {
  return (
    <div className="w-40 max-md:w-full">
      <span className="text-sm text-gray-900 tabular-nums">
        <span className="font-semibold">{part}</span>
        <span className="text-gray-500"> von {whole}</span>
        <span className="ml-2 text-sm text-gray-500">
          {formatShare(part, whole)}
        </span>
      </span>
      {/* The same green share as the tiles above. */}
      <ProgressBar
        value={part}
        max={Math.max(whole, 1)}
        label={`${part} von ${whole} ${label}`}
        className="mt-1.5"
      />
    </div>
  );
}

function Figure({ value }: Readonly<{ value: number }>) {
  return (
    <span className="text-sm font-semibold text-gray-900 tabular-nums">
      {value}
    </span>
  );
}

/**
 * Freie Plätze einer Schule: die Summe über ihre Angebote mit Grenze, dazu
 * wie viele davon voll sind. Ohne Angebot mit Grenze „–“.
 */
function FreePlacesCell({
  school,
}: Readonly<{ school: TraegerSchoolOccupancy }>) {
  const places = occupancyPlaces([school]);
  if (places.limited === 0) {
    return (
      <span className="text-sm text-gray-500">
        –<span className="sr-only">, kein Angebot mit Platzgrenze</span>
      </span>
    );
  }
  return (
    <span className="inline-flex flex-wrap items-center justify-end gap-2 text-sm tabular-nums">
      <span>
        <span className="font-semibold text-gray-900">{places.free}</span>
        <span className="text-gray-500"> von {places.places}</span>
      </span>
      {places.full > 0 ? (
        <StatusBadge
          tone="orange"
          size="xs"
          label={`${places.full} voll`}
          title={
            places.full === 1
              ? "Bei einer Betreuungsform sind alle Plätze vergeben."
              : `Bei ${places.full} Betreuungsformen sind alle Plätze vergeben.`
          }
        />
      ) : null}
    </span>
  );
}

function WithoutLabel({ children }: Readonly<{ children: ReactNode }>) {
  return <span className="text-sm text-gray-500 italic">{children}</span>;
}

const SCHOOL_COLUMNS: DataTableColumn<TraegerSchoolOccupancy>[] = [
  {
    key: "name",
    header: "Schule",
    render: (school) => <SchoolCell school={school} />,
    sortValue: (school) => school.name,
    stacked: "title",
  },
  {
    key: "children",
    header: "Kinder",
    align: "right",
    render: (school) => <Figure value={school.children} />,
    sortValue: (school) => school.children,
  },
  {
    key: "groups",
    header: "In einer Gruppe",
    render: (school) => (
      <ShareCell
        part={school.children - school.withoutGroup}
        whole={school.children}
        label="Kindern in einer Gruppe"
      />
    ),
    sortValue: (school) => school.children - school.withoutGroup,
  },
  {
    key: "careForms",
    header: "Mit Betreuungsform",
    render: (school) => (
      <ShareCell
        part={school.children - school.withoutCareForm}
        whole={school.children}
        label="Kindern mit einer Betreuungsform"
      />
    ),
    sortValue: (school) => school.children - school.withoutCareForm,
  },
  {
    key: "freePlaces",
    header: "Freie Plätze",
    align: "right",
    render: (school) => <FreePlacesCell school={school} />,
    sortValue: (school) => {
      const places = occupancyPlaces([school]);
      return places.limited === 0 ? null : places.free;
    },
  },
  {
    key: "groupCount",
    header: "Gruppen",
    align: "right",
    render: (school) => (
      <span className="text-sm text-gray-700 tabular-nums">
        {school.groups.length}
      </span>
    ),
    sortValue: (school) => school.groups.length,
  },
];

const GROUP_COLUMNS: DataTableColumn<OccupancyGroupRow>[] = [
  {
    key: "group",
    header: "Gruppe",
    render: (row) =>
      row.name === null ? (
        <WithoutLabel>Ohne Gruppe</WithoutLabel>
      ) : (
        <span className="font-medium text-gray-900">{row.name}</span>
      ),
    sortValue: (row) => row.name ?? "",
    stacked: "title",
  },
  {
    key: "school",
    header: "Schule",
    render: (row) => <SchoolCell school={row.school} />,
    sortValue: (row) => row.school.name,
  },
  {
    key: "children",
    header: "Kinder",
    align: "right",
    render: (row) => <Figure value={row.children} />,
    sortValue: (row) => row.children,
  },
  {
    key: "share",
    header: "Anteil an der Schule",
    align: "right",
    render: (row) => (
      <span className="text-sm text-gray-700 tabular-nums">
        {formatShare(row.children, row.school.children)}
      </span>
    ),
    sortValue: (row) =>
      row.school.children > 0 ? row.children / row.school.children : 0,
  },
];

const CARE_FORM_COLUMNS: DataTableColumn<OccupancyCareFormRow>[] = [
  {
    key: "form",
    header: "Betreuungsform",
    render: (row) =>
      row.form === null ? (
        <WithoutLabel>Ohne Betreuungsform</WithoutLabel>
      ) : (
        <span className="font-medium text-gray-900">{row.form.name}</span>
      ),
    sortValue: (row) => row.form?.name ?? "",
    stacked: "title",
  },
  {
    key: "school",
    header: "Schule",
    render: (row) => <SchoolCell school={row.school} />,
    sortValue: (row) => row.school.name,
  },
  {
    key: "children",
    header: "Kinder",
    align: "right",
    render: (row) => <Figure value={row.children} />,
    sortValue: (row) => row.children,
  },
  {
    key: "places",
    header: "Plätze",
    render: (row) =>
      row.form === null ? (
        <span className="text-sm text-gray-500">–</span>
      ) : (
        <OfferingOccupancyCell
          occupancy={offeringOccupancy({
            booked: row.children,
            capacity: row.form.capacity,
          })}
          isActive={row.form.active}
          variant="progress"
        />
      ),
    sortValue: (row) =>
      occupancyRatio(row.children, row.form?.capacity ?? null),
  },
];

const SUM_COLUMNS: DataTableColumn<OccupancyCareFormSum>[] = [
  {
    key: "name",
    header: "Betreuungsform",
    render: (sum) => (
      <span className="font-medium text-gray-900">{sum.name}</span>
    ),
    sortValue: (sum) => sum.name,
    stacked: "title",
  },
  {
    key: "schools",
    header: "Schulen",
    align: "right",
    render: (sum) => (
      <span className="text-sm text-gray-700 tabular-nums">{sum.schools}</span>
    ),
    sortValue: (sum) => sum.schools,
  },
  {
    key: "children",
    header: "Kinder",
    align: "right",
    render: (sum) => <Figure value={sum.children} />,
    sortValue: (sum) => sum.children,
  },
  {
    key: "places",
    header: "Plätze",
    render: (sum) => (
      <OfferingOccupancyCell
        occupancy={offeringOccupancy({
          booked: sum.children,
          capacity: sum.capacity,
        })}
        isActive
        variant="progress"
      />
    ),
    sortValue: (sum) => occupancyRatio(sum.children, sum.capacity),
  },
];

/**
 * „In 4 Betreuungsformen mit Platzgrenze. Eine ist voll.“ One word with the
 * tile and the tab: the page calls an offer „Betreuungsform“.
 */
function freePlacesHint(places: OccupancyPlaces): string {
  if (places.limited === 0) return "Keine Betreuungsform hat eine Platzgrenze.";
  const offers =
    places.limited === 1
      ? "In einer Betreuungsform mit Platzgrenze."
      : `In ${places.limited} Betreuungsformen mit Platzgrenze.`;
  if (places.full === 0) return offers;
  return places.full === 1
    ? `${offers} Eine ist voll.`
    : `${offers} ${places.full} sind voll.`;
}

function TotalFigures({
  totals,
  places,
  loading,
}: Readonly<{
  totals: OccupancyTotals;
  places: OccupancyPlaces;
  loading: boolean;
}>) {
  return (
    <div className="grid gap-4 max-sm:gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div>
        <StatCard
          label="Kinder heute"
          value={totals.children}
          hint={
            loading
              ? undefined
              : "Alle Kinder, die Ihre Schulen heute betreuen."
          }
          icon={<MotoConceptIcon concept={childrenConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
      <StatCard
        label="In einer Gruppe"
        value={loading ? 0 : `${totals.inGroup} von ${totals.children}`}
        hint={
          loading
            ? undefined
            : `${formatShare(totals.inGroup, totals.children)} der Kinder haben eine OGS-Gruppe.`
        }
        progressPct={
          loading || totals.children === 0
            ? undefined
            : (totals.inGroup / totals.children) * 100
        }
        icon={<MotoConceptIcon concept={groupsConcept} />}
        loading={loading}
        tone="green"
      />
      <StatCard
        label="Mit Betreuungsform"
        value={loading ? 0 : `${totals.withCareForm} von ${totals.children}`}
        hint={
          loading
            ? undefined
            : `${formatShare(totals.withCareForm, totals.children)} der Kinder haben heute eine Betreuungsform gebucht.`
        }
        progressPct={
          loading || totals.children === 0
            ? undefined
            : (totals.withCareForm / totals.children) * 100
        }
        icon={<MotoConceptIcon concept={carePlanConcept} />}
        loading={loading}
        tone="green"
      />
      <StatCard
        label="Freie Plätze"
        value={
          loading || places.limited === 0
            ? "–"
            : `${places.free} von ${places.places}`
        }
        hint={loading ? undefined : freePlacesHint(places)}
        progressPct={
          loading || places.places === 0
            ? undefined
            : (places.free / places.places) * 100
        }
        icon={<MotoConceptIcon concept={utilizationConcept} />}
        loading={loading}
        // Free capacity in blue; orange when no place is left.
        tone={places.limited > 0 && places.free === 0 ? "orange" : "blue"}
      />
    </div>
  );
}

const NO_SCHOOL_MATCH = (
  <EmptyState
    icon={<School className="h-12 w-12" aria-hidden="true" />}
    title="Keine Schule passt zu Ihrer Suche."
    description="Prüfen Sie den Namen oder suchen Sie nach dem Ort."
  />
);

function emptyRows(search: string, title: string, description: string) {
  return search.trim() !== "" ? (
    NO_SCHOOL_MATCH
  ) : (
    <EmptyState title={title} description={description} />
  );
}

function ViewContent({
  view,
  schools,
  search,
}: Readonly<{
  view: View;
  schools: readonly TraegerSchoolOccupancy[];
  search: string;
}>) {
  const groupRows = useMemo(() => occupancyGroupRows(schools), [schools]);
  const formRows = useMemo(() => occupancyCareFormRows(schools), [schools]);
  const sums = useMemo(() => occupancyCareFormSums(schools), [schools]);

  if (view === "gruppen") {
    return (
      <DataTable
        heading={{
          title: "Kinder je Gruppe",
          count: groupRows.length,
          level: 2,
        }}
        columns={GROUP_COLUMNS}
        rows={groupRows}
        getRowKey={(row) => row.key}
        defaultSortKey="school"
        stackedOnMobile
        emptyState={emptyRows(
          search,
          "Noch keine Gruppen",
          "Ihre Schulen haben noch keine OGS-Gruppen angelegt.",
        )}
      />
    );
  }
  if (view === "betreuungsformen") {
    return (
      <>
        {sums.some((sum) => sum.schools > 1) ? (
          <DataTable
            heading={{
              title: "Gleiche Angebote über alle Schulen",
              count: sums.length,
              level: 2,
            }}
            columns={SUM_COLUMNS}
            rows={sums}
            getRowKey={(sum) => sum.name}
            defaultSortKey="children"
            defaultSortDirection="desc"
            stackedOnMobile
          />
        ) : null}
        <DataTable
          heading={{
            title: "Kinder je Betreuungsform und Schule",
            count: formRows.length,
            level: 2,
          }}
          columns={CARE_FORM_COLUMNS}
          rows={formRows}
          getRowKey={(row) => row.key}
          defaultSortKey="school"
          stackedOnMobile
          emptyState={emptyRows(
            search,
            "Heute läuft keine Betreuungsform",
            "Hier steht jede Betreuungsform, die heute läuft. Auch eine ohne Buchung.",
          )}
        />
      </>
    );
  }
  return (
    <DataTable
      heading={{ title: "Kinder je Schule", count: schools.length, level: 2 }}
      columns={SCHOOL_COLUMNS}
      rows={[...schools]}
      getRowKey={(school) => school.id}
      defaultSortKey="name"
      stackedOnMobile
      emptyState={NO_SCHOOL_MATCH}
    />
  );
}

export default function TraegerOccupancyPage() {
  const { occupancy, error, reportError } = useTraegerOccupancy();
  const { show: showActionError } = useApiErrorDisplay();
  const [view, setView] = useState<View>("schulen");
  const [search, setSearch] = useState("");
  const [exporting, setExporting] = useState(false);
  const loading = occupancy === null && !error;
  const allSchools = useMemo(() => occupancy?.schools ?? [], [occupancy]);
  const schools = useMemo(
    () => allSchools.filter((school) => matchesSchool(school, search)),
    [allSchools, search],
  );
  const totals = useMemo(() => occupancyTotals(schools), [schools]);
  const places = useMemo(() => occupancyPlaces(schools), [schools]);

  async function exportCsv() {
    setExporting(true);
    try {
      const file = await downloadTraegerOccupancy();
      downloadBlob(
        file.blob,
        file.filename ?? `belegung-${occupancy?.day ?? "heute"}.csv`,
      );
    } catch (err) {
      reportError(err);
      logger.error("traeger_occupancy_export_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "die Exportdatei" });
    } finally {
      setExporting(false);
    }
  }

  return (
    <TenantPage
      title="Belegung"
      info={
        <>
          Hier sehen Sie, wie viele Kinder Ihre Schulen heute betreuen. Die
          Zahlen stehen je Schule, je OGS-Gruppe und je Betreuungsform. Eine
          Betreuungsform ist ein Betreuungsangebot der Schule, das als Betreuung
          zählt, zum Beispiel „OGS bis 16 Uhr“. Mittagessen zählt nicht. Jedes
          Angebot, das heute läuft, steht hier, auch ohne Buchung. Daneben sehen
          Sie seine freien Plätze. Gibt es ein Angebot mit gleichem Namen an
          mehreren Schulen, steht es unter „Betreuungsformen“ auch
          zusammengezählt. Es zählen dieselben Kinder wie bei der Stichtagszahl.
          moto zeigt hier keine Namen von Kindern. Mit „Herunterladen“ bekommen
          Sie alle Zahlen als Tabelle.
        </>
      }
      actions={
        <ExportMenu
          ariaLabel="Belegung herunterladen"
          options={EXPORT_FORMATS}
          busyKey={exporting ? "csv" : null}
          unavailable={!occupancy}
          onSelect={() => void exportCsv()}
        />
      }
      stats={
        occupancy ? (
          <TenantPageStats
            items={[
              {
                value: `${formatDate(occupancy.day)}, Stand ${formatBerlinTime(occupancy.asOf)} Uhr`,
                label: "",
              },
              {
                value: allSchools.length,
                label: allSchools.length === 1 ? "Schule" : "Schulen",
              },
            ]}
          />
        ) : null
      }
      statsLoading={loading}
      error={error}
      empty={
        occupancy && allSchools.length === 0
          ? {
              title: "Noch keine Schulen",
              description:
                "Sobald das moto-Team Ihrem Träger Schulen zuordnet, steht hier ihre Belegung.",
              icon: <School className="h-12 w-12" aria-hidden="true" />,
            }
          : null
      }
      search={
        allSchools.length > 1
          ? {
              value: search,
              onChange: setSearch,
              placeholder: "Schule oder Ort suchen…",
            }
          : undefined
      }
      tabs={{
        value: view,
        onChange: (next) => setView(next as View),
        items: VIEWS,
        label: "Ansicht der Belegung",
      }}
    >
      <TotalFigures totals={totals} places={places} loading={loading} />
      {occupancy ? (
        <ViewContent view={view} schools={schools} search={search} />
      ) : (
        <DataTable
          columns={SCHOOL_COLUMNS}
          rows={[]}
          getRowKey={(school) => school.id}
          isLoading={loading}
          loadingRowCount={3}
        />
      )}
    </TenantPage>
  );
}
