"use client";

// Dashboard des Träger-Portals (#4674): die Zahlen von jetzt für alle Schulen
// des Trägers auf einer Seite, oben die Summe, darunter je Schule eine Zeile.
// Nur Zahlen, keine Namen. Eine Zeile führt in die Ansicht der Schule (#5686);
// wer das Recht „In OGS wechseln" hat, öffnet von hier auch die Schul-App.

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { ExternalLink, School } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import NavigationLink from "~/components/ui/navigation-link";
import { ProgressBar } from "~/components/ui/progress-bar";
import { StatCard } from "~/components/ui/stat-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  SCHOOL_SWITCH_MISSING_HINT,
  useOpenTraegerSchool,
} from "~/components/traeger/use-open-traeger-school";
import { useTraegerSchoolsToday } from "~/components/traeger/use-traeger-schools-today";
import { childrenConcept } from "~/lib/concepts/children";
import { presentConcept } from "~/lib/concepts/present";
import { sickConcept } from "~/lib/concepts/sick";
import { staffConcept } from "~/lib/concepts/staff";
import { supervisionConcept } from "~/lib/concepts/supervision";
import { formatTime } from "~/lib/date-helpers";
import type {
  TraegerSchoolFigures,
  TraegerSchoolToday,
} from "~/lib/traeger-api";
import { TRAEGER_HANDOFF_OPEN } from "~/lib/traeger-handoff";
import { traegerPath } from "~/lib/traeger-url";

const EMPTY_VALUE = "–";

const ratioFormat = new Intl.NumberFormat("de-DE", {
  maximumFractionDigits: 1,
});

const percentFormat = new Intl.NumberFormat("de-DE", {
  style: "percent",
  maximumFractionDigits: 0,
});

/** „1 : 8,5", ohne anwesende Betreuungskraft der Strich. */
function formatRatio(ratio: number | null): string {
  return ratio === null ? EMPTY_VALUE : `1 : ${ratioFormat.format(ratio)}`;
}

function schoolPath(school: TraegerSchoolToday): string {
  return traegerPath(`/traeger/schulen/${school.id}`);
}

/** Anteil der anwesenden Kinder an allen Kindern, 0 ohne Kinder. */
function presentShare(figures: TraegerSchoolFigures): number {
  return figures.childrenTotal > 0
    ? figures.childrenPresent / figures.childrenTotal
    : 0;
}

function matchesSearch(school: TraegerSchoolToday, search: string): boolean {
  const needle = search.trim().toLocaleLowerCase("de-DE");
  if (needle === "") return true;
  return `${school.name} ${school.city}`
    .toLocaleLowerCase("de-DE")
    .includes(needle);
}

function TotalFigures({
  total,
  schoolCount,
}: Readonly<{ total: TraegerSchoolFigures | null; schoolCount: number }>) {
  const loading = total === null;
  const share = total ? presentShare(total) : 0;
  const ratio = total?.careRatio ?? null;
  return (
    <div className="grid grid-cols-2 gap-4 max-sm:gap-3 md:grid-cols-3 xl:grid-cols-5">
      <StatCard
        label="Kinder gesamt"
        value={total?.childrenTotal ?? 0}
        hint={
          schoolCount === 1
            ? "Alle Kinder, die Ihre Schule gerade betreut."
            : `Alle Kinder, die Ihre ${schoolCount} Schulen gerade betreuen.`
        }
        icon={<MotoConceptIcon concept={childrenConcept} />}
        loading={loading}
        tone="neutral"
      />
      <StatCard
        label="Jetzt anwesend"
        value={total?.childrenPresent ?? 0}
        hint={`${percentFormat.format(share)} aller Kinder sind gerade in der Schule.`}
        progressPct={share * 100}
        icon={<MotoConceptIcon concept={presentConcept} />}
        loading={loading}
        tone="green"
      />
      <StatCard
        label="Heute krank"
        value={total?.childrenSick ?? 0}
        hint="Kinder mit einer Krankmeldung für heute."
        icon={<MotoConceptIcon concept={sickConcept} />}
        loading={loading}
        tone="neutral"
      />
      <StatCard
        label="Betreuungskräfte anwesend"
        value={total ? `${total.staffPresent} von ${total.staffTotal}` : 0}
        hint="Zählt, wer in der Zeiterfassung angemeldet ist. Homeoffice zählt nicht."
        icon={<MotoConceptIcon concept={staffConcept} />}
        loading={loading}
        tone="neutral"
      />
      <div className="max-md:col-span-2">
        <StatCard
          label="Betreuungsschlüssel"
          value={formatRatio(ratio)}
          hint={
            ratio === null
              ? "Gerade ist an keiner Schule eine Betreuungskraft in der Zeiterfassung angemeldet."
              : "Eine anwesende Betreuungskraft kommt im Schnitt auf so viele anwesende Kinder."
          }
          icon={<MotoConceptIcon concept={supervisionConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
    </div>
  );
}

const COLUMNS: DataTableColumn<TraegerSchoolToday>[] = [
  {
    key: "name",
    header: "Schule",
    // Der Link ist der Weg per Tastatur und Vorlesehilfe; die Zeile hat eigene
    // Bedienelemente und wird deshalb selbst kein Knopf.
    render: (school) => (
      <div className="min-w-0">
        <NavigationLink
          href={schoolPath(school)}
          className="font-medium text-gray-900 hover:underline"
          onClick={(event) => event.stopPropagation()}
        >
          {school.name}
        </NavigationLink>
        {school.city ? (
          <span className="block text-xs text-gray-500 max-md:hidden">
            {school.city}
          </span>
        ) : null}
      </div>
    ),
    sortValue: (school) => school.name,
    stacked: "title",
  },
  {
    key: "present",
    header: "Jetzt anwesend",
    render: (school) => (
      <div className="w-36 max-md:w-full">
        <span className="text-sm text-gray-900 tabular-nums">
          <span className="font-semibold">
            {school.figures.childrenPresent}
          </span>
          <span className="text-gray-500">
            {" "}
            von {school.figures.childrenTotal}
          </span>
        </span>
        <ProgressBar
          value={school.figures.childrenPresent}
          max={school.figures.childrenTotal}
          label={`${school.figures.childrenPresent} von ${school.figures.childrenTotal} Kindern anwesend`}
          className="mt-1.5"
        />
      </div>
    ),
    sortValue: (school) => school.figures.childrenPresent,
  },
  {
    key: "sick",
    header: "Heute krank",
    align: "right",
    render: (school) => (
      <span className="text-sm text-gray-900 tabular-nums">
        {school.figures.childrenSick}
      </span>
    ),
    sortValue: (school) => school.figures.childrenSick,
  },
  {
    key: "staff",
    header: "Betreuungskräfte",
    align: "right",
    render: (school) => (
      <span className="text-sm text-gray-900 tabular-nums">
        <span className="font-semibold">{school.figures.staffPresent}</span>
        <span className="text-gray-500"> von {school.figures.staffTotal}</span>
      </span>
    ),
    sortValue: (school) => school.figures.staffPresent,
  },
  {
    key: "ratio",
    header: "Schlüssel",
    align: "right",
    render: (school) =>
      school.figures.careRatio === null ? (
        <span className="text-sm text-gray-500">
          {EMPTY_VALUE}
          <span className="sr-only">
            , keine Betreuungskraft in der Zeiterfassung angemeldet
          </span>
        </span>
      ) : (
        <span className="text-sm font-semibold text-gray-900 tabular-nums">
          {formatRatio(school.figures.careRatio)}
        </span>
      ),
    // Ohne Betreuungskraft fehlt der Wert und sortiert nach allen anderen.
    sortValue: (school) => school.figures.careRatio,
  },
];

export default function TraegerDashboardPage() {
  const { today, error } = useTraegerSchoolsToday();
  const router = useRouter();
  const { canOpen, opening, open } = useOpenTraegerSchool();
  const [search, setSearch] = useState("");
  const loading = today === null && !error;
  const schools = today?.schools ?? [];
  const rows = useMemo(
    () =>
      (today?.schools ?? []).filter((school) => matchesSearch(school, search)),
    [today, search],
  );

  const columns = useMemo<DataTableColumn<TraegerSchoolToday>[]>(
    () =>
      canOpen
        ? [
            ...COLUMNS,
            {
              key: "actions",
              header: "",
              align: "right",
              stacked: "meta",
              stackedLabel: false,
              render: (school) => (
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  aria-label={`${TRAEGER_HANDOFF_OPEN}: ${school.name}, neuer Tab`}
                  title="Öffnet in einem neuen Tab"
                  disabled={opening !== null}
                  onClick={(event) => {
                    // Die Aktion öffnet die Schul-App, nicht die Schulansicht.
                    event.stopPropagation();
                    void open(school);
                  }}
                >
                  {opening === school.id ? (
                    "Wird geöffnet …"
                  ) : (
                    <>
                      {TRAEGER_HANDOFF_OPEN}
                      <ExternalLink className="h-4 w-4" aria-hidden="true" />
                    </>
                  )}
                </Button>
              ),
            },
          ]
        : COLUMNS,
    [canOpen, open, opening],
  );

  return (
    <TenantPage
      title="Dashboard"
      info={
        <>
          Die Zahlen von jetzt für alle Schulen Ihres Trägers. Oben steht die
          Summe, darunter jede Schule. moto zeigt hier keine Namen. Die Zahlen
          aktualisieren sich jede Minute. Ein Klick auf eine Schule zeigt ihre
          Zahlen und Stammdaten.
          {canOpen
            ? " „Schul-App öffnen“ zeigt die Schule in einem neuen Tab."
            : ` ${SCHOOL_SWITCH_MISSING_HINT}`}
        </>
      }
      stats={
        today ? (
          <TenantPageStats
            items={[
              {
                value: schools.length,
                label: schools.length === 1 ? "Schule" : "Schulen",
              },
              {
                value: `Stand ${formatTime(today.total.asOf)} Uhr`,
                label: "",
              },
            ]}
          />
        ) : null
      }
      statsLoading={loading}
      error={error}
      search={
        schools.length > 1
          ? {
              value: search,
              onChange: setSearch,
              placeholder: "Schule oder Ort suchen…",
            }
          : undefined
      }
    >
      <TotalFigures total={today?.total ?? null} schoolCount={schools.length} />
      <DataTable
        heading={{ title: "Zahlen je Schule", count: rows.length }}
        columns={columns}
        rows={rows}
        getRowKey={(school) => school.id}
        isLoading={loading}
        loadingRowCount={3}
        defaultSortKey="name"
        stackedOnMobile
        rowHasInteractiveControls
        onRowClick={(school) => router.push(schoolPath(school))}
        emptyState={
          schools.length === 0 ? (
            <EmptyState
              icon={<School className="h-12 w-12" aria-hidden="true" />}
              title="Ihrem Träger sind noch keine Schulen zugeordnet."
              description="Das moto-Team ordnet die Schulen Ihrem Träger zu."
            />
          ) : (
            <EmptyState
              icon={<School className="h-12 w-12" aria-hidden="true" />}
              title="Keine Schule passt zu Ihrer Suche."
              description="Prüfen Sie den Namen oder suchen Sie nach dem Ort."
            />
          )
        }
      />
    </TenantPage>
  );
}
