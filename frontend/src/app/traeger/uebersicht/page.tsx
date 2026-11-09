"use client";

// Dashboard des Träger-Portals (#4674): die Zahlen von jetzt für alle Schulen
// des Trägers auf einer Seite, oben die Summe, darunter je Schule eine Zeile.
// Nur Zahlen, keine Namen. Eine Zeile führt in die Ansicht der Schule (#5686);
// wer das Recht „In OGS wechseln" hat, öffnet von hier auch die Schul-App.
// Zwischen Summe und Tabelle stehen die Auffälligkeiten (#4675); in der
// Tabelle ist jede Zahl hervorgehoben, die eine Grenze des Trägers erreicht.

import { type MouseEvent, useId, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ChevronDown,
  ChevronUp,
  ExternalLink,
  School,
  TriangleAlert,
} from "lucide-react";
import { CheckCircleIcon } from "@phosphor-icons/react/ssr";
import { useSession } from "next-auth/react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Button, ButtonLink } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
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
import { useTraegerSchoolsAlerts } from "~/components/traeger/use-traeger-schools-alerts";
import {
  ALERT_LABELS,
  alertSentence,
  schoolCountLabel,
  thresholdsSentence,
} from "~/lib/traeger-alerts";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import type { MotoConcept } from "~/lib/moto-concepts";
import { hasPermission } from "~/lib/auth-utils";
import { childrenConcept } from "~/lib/concepts/children";
import { presentConcept } from "~/lib/concepts/present";
import { sickConcept } from "~/lib/concepts/sick";
import { staffConcept } from "~/lib/concepts/staff";
import { supervisionConcept } from "~/lib/concepts/supervision";
import { formatBerlinTime } from "~/lib/date-helpers";
import type {
  TraegerAlertKind,
  TraegerSchoolAlerts,
  TraegerSchoolFigures,
  TraegerSchoolToday,
  TraegerSchoolsToday,
} from "~/lib/traeger-api";
import { TRAEGER_HANDOFF_OPEN } from "~/lib/traeger-handoff";
import { traegerPath } from "~/lib/traeger-url";
import { foldForSearch } from "~/lib/search-fold";

const EMPTY_VALUE = "–";

/** Rows shown before „N weitere anzeigen“, as in the other hint lists. */
const INITIAL_ROW_COUNT = 5;

/** The value of the school filter that shows every school. */
const ALL_SCHOOLS = "";

/** The settings area with the thresholds (`?bereich=` on the settings page). */
const ALERT_SETTINGS_PATH = "/traeger/einstellungen?bereich=dashboard";

const ALERT_CONCEPTS: Readonly<Record<TraegerAlertKind, MotoConcept>> = {
  staff_shortage: staffConcept,
  sick_children: sickConcept,
  open_enrollments: enrollmentsConcept,
};

/** The Träger right that changes the thresholds of the hints. */
const SETTINGS_RIGHT = "traeger_settings:manage";

/** The hints of one school by its ID; missing while they load. */
type AlertsBySchool = ReadonlyMap<string, TraegerSchoolAlerts>;

function hasAlert(
  alerts: AlertsBySchool,
  school: TraegerSchoolToday,
  kind: TraegerAlertKind,
): boolean {
  return alerts.get(school.id)?.alerts.includes(kind) ?? false;
}

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
  const needle = foldForSearch(search.trim());
  if (needle === "") return true;
  return foldForSearch(`${school.name} ${school.city}`).includes(needle);
}

/**
 * Die Gesamtzahl ist der Bestand, nicht die Anwesenheit: ohne „gerade", das
 * die Kachel daneben für die anwesenden Kinder braucht.
 */
function childrenTotalHint(schoolCount: number): string {
  if (schoolCount === 1) return "Alle Kinder, die Ihre Schule betreut.";
  if (schoolCount > 1)
    return `Alle Kinder, die Ihre ${schoolCount} Schulen betreuen.`;
  return "Alle Kinder, die Ihre Schulen betreuen.";
}

function TotalFigures({
  total,
  schoolCount,
}: Readonly<{ total: TraegerSchoolFigures | null; schoolCount: number }>) {
  // Ohne Zahlen keine Sätze mit Zahlen: „0 %" während des Ladens wäre falsch.
  const loading = total === null;
  const share = total ? presentShare(total) : 0;
  const ratio = total?.careRatio ?? null;
  return (
    // Oben die Kinder, darunter das Personal: zwei Reihen statt fünf schmaler
    // Kacheln, in denen die langen Labels umbrechen.
    <div className="grid grid-cols-2 gap-4 max-sm:gap-3 lg:grid-cols-6">
      <div className="col-span-2 sm:col-span-1 lg:col-span-2">
        <StatCard
          label="Kinder gesamt"
          value={total?.childrenTotal ?? 0}
          hint={loading ? undefined : childrenTotalHint(schoolCount)}
          icon={<MotoConceptIcon concept={childrenConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
      <div className="lg:col-span-2">
        <StatCard
          label="Jetzt anwesend"
          value={total?.childrenPresent ?? 0}
          hint={
            loading
              ? undefined
              : `${percentFormat.format(share)} aller Kinder sind gerade in der Schule.`
          }
          progressPct={loading ? undefined : share * 100}
          icon={<MotoConceptIcon concept={presentConcept} />}
          loading={loading}
          tone="green"
        />
      </div>
      <div className="lg:col-span-2">
        <StatCard
          label="Heute krank"
          value={total?.childrenSick ?? 0}
          hint="Kinder mit einer Krankmeldung für heute."
          icon={<MotoConceptIcon concept={sickConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
      <div className="col-span-2 sm:col-span-1 lg:col-span-3">
        <StatCard
          label="Betreuungskräfte anwesend"
          value={total ? `${total.staffPresent} von ${total.staffTotal}` : 0}
          hint="Zählt, wer in der Zeiterfassung angemeldet ist. Homeoffice zählt nicht."
          icon={<MotoConceptIcon concept={staffConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
      <div className="col-span-2 sm:col-span-1 lg:col-span-3">
        <StatCard
          label="Betreuungsschlüssel"
          value={formatRatio(ratio)}
          hint={
            loading
              ? undefined
              : ratio === null
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

function figureColumns(
  alerts: AlertsBySchool,
  alertsLoaded: boolean,
): DataTableColumn<TraegerSchoolToday>[] {
  return [
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
          {hasAlert(alerts, school, "sick_children") ? (
            <StatusBadge
              tone="warning"
              size="xs"
              label={String(school.figures.childrenSick)}
              accessibleLabel=", erreicht Ihre Grenze"
            />
          ) : (
            school.figures.childrenSick
          )}
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
          {hasAlert(alerts, school, "staff_shortage") ? (
            <StatusBadge
              tone="warning"
              size="xs"
              label={String(school.figures.staffPresent)}
              accessibleLabel=", erreicht Ihre Grenze"
            />
          ) : (
            <span className="font-semibold">{school.figures.staffPresent}</span>
          )}
          <span className="text-gray-500">
            {" "}
            von {school.figures.staffTotal}
          </span>
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
            {hasAlert(alerts, school, "staff_shortage") ? (
              <StatusBadge
                tone="warning"
                size="xs"
                label={formatRatio(school.figures.careRatio)}
                accessibleLabel=", erreicht Ihre Grenze"
              />
            ) : (
              formatRatio(school.figures.careRatio)
            )}
          </span>
        ),
      // Ohne Betreuungskraft fehlt der Wert und sortiert nach allen anderen.
      sortValue: (school) => school.figures.careRatio,
    },
    {
      key: "enrollments",
      header: "Offene Anmeldungen",
      align: "right",
      render: (school) => {
        const open = alerts.get(school.id)?.openEnrollments;
        return open === undefined ? (
          <span className="text-sm text-gray-500">
            {EMPTY_VALUE}
            <span className="sr-only">
              {alertsLoaded ? ", keine Angabe" : ", wird geladen"}
            </span>
          </span>
        ) : (
          <span className="text-sm text-gray-900 tabular-nums">
            {hasAlert(alerts, school, "open_enrollments") ? (
              <StatusBadge
                tone="warning"
                size="xs"
                label={String(open)}
                accessibleLabel=", erreicht Ihre Grenze"
              />
            ) : (
              open
            )}
          </span>
        );
      },
      sortValue: (school) => alerts.get(school.id)?.openEnrollments ?? null,
    },
  ];
}

export default function TraegerDashboardPage() {
  const { alerts, error, refreshFailed, reload } = useTraegerSchoolsAlerts();
  const { data: session } = useSession();
  const router = useRouter();
  const { canOpen, opening, open } = useOpenTraegerSchool();
  const [search, setSearch] = useState("");
  const [alertSchoolId, setAlertSchoolId] = useState(ALL_SCHOOLS);
  const [alertsExpanded, setAlertsExpanded] = useState(false);
  const listId = useId();
  const affected = useMemo(
    () => (alerts?.schools ?? []).filter((school) => school.alerts.length > 0),
    [alerts],
  );
  const options = useMemo(
    () => [
      { value: ALL_SCHOOLS, label: "Alle Schulen" },
      ...(alerts?.schools ?? []).map((school) => ({
        value: school.id,
        label:
          school.alerts.length > 0
            ? `${school.name} (${school.alerts.length})`
            : school.name,
      })),
    ],
    [alerts],
  );

  const alertsLoading = alerts === null;
  const selected = alerts?.schools.find(
    (school) => school.id === alertSchoolId,
  );
  const alertRows = selected
    ? affected.filter((school) => school.id === selected.id)
    : affected;
  const hiddenCount = Math.max(0, alertRows.length - INITIAL_ROW_COUNT);
  const visible = alertsExpanded
    ? alertRows
    : alertRows.slice(0, INITIAL_ROW_COUNT);
  const quiet = !alertsLoading && affected.length === 0;

  // One snapshot carries every figure, the hints and the timestamp, so the
  // sums, the rows and their markers never disagree.
  const snapshot = useMemo<TraegerSchoolsToday | null>(() => {
    if (!alerts) return null;
    const total = alerts.schools.reduce(
      (sum, { figures }) => ({
        childrenTotal: sum.childrenTotal + figures.childrenTotal,
        childrenPresent: sum.childrenPresent + figures.childrenPresent,
        childrenSick: sum.childrenSick + figures.childrenSick,
        staffTotal: sum.staffTotal + figures.staffTotal,
        staffPresent: sum.staffPresent + figures.staffPresent,
      }),
      {
        childrenTotal: 0,
        childrenPresent: 0,
        childrenSick: 0,
        staffTotal: 0,
        staffPresent: 0,
      },
    );
    return {
      schools: alerts.schools,
      total: {
        ...total,
        careRatio:
          total.staffPresent > 0
            ? total.childrenPresent / total.staffPresent
            : null,
        asOf: alerts.asOf,
      },
    };
  }, [alerts]);
  const loading = snapshot === null && !error;
  const schools = snapshot?.schools ?? [];
  const alertsBySchool = useMemo<AlertsBySchool>(
    () => new Map((alerts?.schools ?? []).map((school) => [school.id, school])),
    [alerts],
  );
  const rows = useMemo(
    () =>
      (snapshot?.schools ?? []).filter((school) =>
        matchesSearch(school, search),
      ),
    [snapshot, search],
  );
  const canChangeThresholds = hasPermission(session, SETTINGS_RIGHT);

  const columns = useMemo<DataTableColumn<TraegerSchoolToday>[]>(() => {
    const figures = figureColumns(alertsBySchool, alerts !== null);
    return canOpen
      ? [
          ...figures,
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
      : figures;
  }, [alerts, alertsBySchool, canOpen, open, opening]);

  return (
    <TenantPage
      title="Dashboard"
      info={
        <>
          Die Zahlen von jetzt für alle Schulen Ihres Trägers. Oben steht die
          Summe, darunter die Auffälligkeiten und jede Schule. moto zeigt hier
          keine Namen. Die Zahlen aktualisieren sich jede Minute. Ein Klick auf
          eine Schule zeigt ihre Zahlen und Stammdaten.
          {canOpen
            ? " „Schul-App öffnen“ zeigt die Schule in einem neuen Tab."
            : ` ${SCHOOL_SWITCH_MISSING_HINT}`}
        </>
      }
      stats={
        snapshot ? (
          <TenantPageStats
            items={[
              {
                value: `Stand ${formatBerlinTime(snapshot.total.asOf)} Uhr`,
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
      {refreshFailed && snapshot ? (
        // Bestätigte Zahlen bleiben stehen; der Hinweis sagt, wie alt sie sind.
        <Alert
          type="warning"
          message={`Stand ${formatBerlinTime(snapshot.total.asOf)} Uhr, Aktualisierung fehlgeschlagen. Die Zahlen sind vielleicht nicht mehr aktuell.`}
          action={
            <Button
              type="button"
              variant="ghost_tone"
              size="compact"
              onClick={reload}
            >
              Erneut laden
            </Button>
          }
        />
      ) : null}
      <TotalFigures
        total={snapshot?.total ?? null}
        schoolCount={schools.length}
      />
      {alerts?.schools.length === 0 ? null : (
        <SectionCard
          title="Auffälligkeiten"
          testId="traeger-school-alerts"
          loading={alertsLoading}
          titleBadge={
            affected.length > 0 ? (
              <StatusBadge
                tone="warning"
                label={schoolCountLabel(affected.length)}
              />
            ) : undefined
          }
          description="Hier stehen die Schulen, an denen gerade etwas Ihre Grenzen erreicht."
          leading={
            quiet ? (
              <CheckCircleIcon
                weight="duotone"
                className="text-moto-green-strong mt-0.5 size-5 shrink-0"
                aria-hidden="true"
              />
            ) : (
              <TriangleAlert
                className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
                aria-hidden="true"
              />
            )
          }
          actions={
            alerts && alerts.schools.length > 1 ? (
              <CustomSelect
                value={alertSchoolId}
                options={options}
                onChange={(value) => {
                  setAlertSchoolId(value);
                  setAlertsExpanded(false);
                }}
                ariaLabel="Auffälligkeiten nach Schule filtern"
                controlSize="compact"
                searchable={alerts.schools.length > 8}
                searchPlaceholder="Schule suchen"
                className="w-full sm:w-64"
              />
            ) : undefined
          }
        >
          {alerts ? (
            <div className="mb-3 flex flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
              <p className="text-sm text-gray-600">
                {thresholdsSentence(alerts.thresholds)}
              </p>
              <ButtonLink
                href={traegerPath(ALERT_SETTINGS_PATH)}
                variant="link"
                size="md"
                className="shrink-0 self-start sm:self-auto"
              >
                {canChangeThresholds ? "Grenzen ändern" : "Grenzen ansehen"}
              </ButtonLink>
            </div>
          ) : null}
          {quiet ? (
            <p className="text-sm text-gray-900">
              Gerade ist an keiner Schule etwas auffällig.
            </p>
          ) : null}
          {!quiet && selected && alertRows.length === 0 ? (
            <p className="text-sm text-gray-900">
              An dieser Schule ist gerade nichts auffällig.
            </p>
          ) : null}
          {visible.length > 0 ? (
            <ResponsiveActionList id={listId}>
              {visible.map((school) => (
                <li
                  key={school.id}
                  className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
                >
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-gray-900">
                      {school.name}
                      {school.city ? (
                        <span className="font-normal text-gray-500">
                          {" "}
                          · {school.city}
                        </span>
                      ) : null}
                    </p>
                    <ul className="mt-1.5 flex flex-col gap-1.5">
                      {school.alerts.map((kind) => (
                        <li
                          key={kind}
                          className="flex items-start gap-2 text-sm"
                        >
                          <MotoConceptIcon
                            concept={ALERT_CONCEPTS[kind]}
                            className="mt-0.5 size-4 shrink-0"
                            aria-hidden="true"
                          />
                          <span className="min-w-0 text-gray-600">
                            <span className="font-medium text-gray-900">
                              {ALERT_LABELS[kind]}:
                            </span>{" "}
                            <span className="tabular-nums">
                              {alertSentence(kind, school)}
                            </span>
                          </span>
                        </li>
                      ))}
                    </ul>
                  </div>
                  <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                    <ButtonLink
                      href={traegerPath(`/traeger/schulen/${school.id}`)}
                      variant="surface"
                      size="compact"
                      aria-label={`Zur Schule: ${school.name}`}
                    >
                      Zur Schule
                    </ButtonLink>
                  </div>
                </li>
              ))}
            </ResponsiveActionList>
          ) : null}
          {hiddenCount > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="mt-2 w-full"
              aria-expanded={alertsExpanded}
              aria-controls={listId}
              onClick={() => setAlertsExpanded((current) => !current)}
            >
              {alertsExpanded ? (
                <>
                  Weniger anzeigen
                  <ChevronUp className="size-4" aria-hidden="true" />
                </>
              ) : (
                <>
                  {hiddenCount} weitere anzeigen
                  <ChevronDown className="size-4" aria-hidden="true" />
                </>
              )}
            </Button>
          ) : null}
        </SectionCard>
      )}
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
