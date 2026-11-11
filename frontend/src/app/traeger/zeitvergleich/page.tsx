"use client";

// Länger als gebucht (#4683, ADR 0156): je Schule, wie oft Kinder länger da
// waren, als ihr Vertrag bucht. moto vergleicht die gebuchten Zeiten mit dem
// Ein- und Auschecken (Soll und Ist, ADR 0139). Die Zahlen je Schule sieht
// jede Rolle mit „Berichte lesen“, die Kinder mit Namen nur „Verträge
// verwalten“; jedes Lesen der Namen steht im Zugriffsprotokoll der Schule.

import {
  type MouseEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { CalendarOff, CircleCheck, CircleDashed } from "lucide-react";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import NavigationLink from "~/components/ui/navigation-link";
import { PlanningContextBar } from "~/components/ui/planning/planning-context-bar";
import { ProgressBar } from "~/components/ui/progress-bar";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { calendarConcept } from "~/lib/concepts/calendar";
import { careTimesConcept } from "~/lib/concepts/careTimes";
import { childrenConcept } from "~/lib/concepts/children";
import { formatDate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { getAccessibleTextColor } from "~/lib/kit-palette";
import { createLogger } from "~/lib/logger";
import { schoolClassLabel } from "~/lib/school-class-label";
import {
  fetchOverrunChildren,
  fetchOverrunSummary,
  formatExtraTime,
  overrunCoverage,
  schoolHasFigures,
  OVERRUN_NAMES_RIGHT,
  overrunDayCount,
  type OverrunChild,
  type OverrunEvaluation,
  type SchoolOverruns,
} from "~/lib/traeger-care-time-overruns-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerZeitvergleichPage" });

const ALL_SCHOOLS = "all";
/** So viele Monate zurück lässt sich die Auswertung wählen. */
const MONTHS_BACK = 12;
const MONTH_NAMES = [
  "Januar",
  "Februar",
  "März",
  "April",
  "Mai",
  "Juni",
  "Juli",
  "August",
  "September",
  "Oktober",
  "November",
  "Dezember",
];
const NO_DATA_TITLE = "Für diesen Zeitraum gibt es keine Check-ins.";
const UNCOMPARED_TITLE =
  "Die Check-ins in diesem Zeitraum ließen sich nicht vergleichen.";

/**
 * Warum nichts verglichen ist: Check-ins fehlen ganz, oder sie sind da,
 * aber ohne Plan oder ohne Auschecken. Beides lässt die Zahlen offen.
 */
function noDataTitle(evaluation: OverrunEvaluation): string {
  return evaluation.schools.some((school) => school.uncomparedDays > 0)
    ? UNCOMPARED_TITLE
    : NO_DATA_TITLE;
}
const OVERRUN_TEXT = { color: getAccessibleTextColor(LOCATION_COLORS.DANGER) };

/** "2026-11" aus "2026-11-06". */
function monthOf(date: string): string {
  return date.slice(0, 7);
}

function monthLabel(month: string): string {
  const [year, index] = month.split("-").map(Number);
  return `${MONTH_NAMES[(index ?? 1) - 1]} ${year}`;
}

/** Die letzten Monate bis zum laufenden, der neueste zuerst. */
function recentMonths(today: string): string[] {
  const [year, index] = today.split("-").map(Number);
  const months: string[] = [];
  for (let back = 0; back < MONTHS_BACK; back++) {
    const total = (year ?? 0) * 12 + ((index ?? 1) - 1) - back;
    months.push(
      `${Math.floor(total / 12)}-${String((total % 12) + 1).padStart(2, "0")}`,
    );
  }
  return months;
}

/** Erster bis letzter Tag des Monats, im laufenden Monat bis heute. */
function monthWindow(
  month: string,
  today: string,
): { from: string; to: string } {
  const [year, index] = month.split("-").map(Number);
  const lastDay = new Date(year ?? 0, index ?? 1, 0).getDate();
  const to = `${month}-${String(lastDay).padStart(2, "0")}`;
  return { from: `${month}-01`, to: to > today ? today : to };
}

/** Ohne Check-ins gibt es keine Zahl: „–“ statt einer Null, die nach Entwarnung aussieht. */
function NoData() {
  return (
    <span className="text-sm text-gray-500">
      –<span className="sr-only">, nichts verglichen</span>
    </span>
  );
}

/** Eine Zahl in Rot, sobald sie eine Überschreitung zählt. */
function OverrunFigure({
  value,
  children,
}: Readonly<{ value: number; children?: ReactNode }>) {
  return (
    <span
      className="text-sm font-semibold tabular-nums"
      style={value > 0 ? OVERRUN_TEXT : undefined}
    >
      {children ?? value}
    </span>
  );
}

function DaysCell({
  overrunDays,
  unbookedDays,
}: Readonly<{ overrunDays: number; unbookedDays: number }>) {
  const total = overrunDays + unbookedDays;
  return (
    <span className="inline-flex flex-col items-end">
      <OverrunFigure value={total} />
      {unbookedDays > 0 ? (
        <span className="text-xs text-gray-500">
          davon {unbookedDays} ohne Buchung
        </span>
      ) : null}
    </span>
  );
}

function schoolColumns(
  from: string,
  to: string,
  maxMinutes: number,
): DataTableColumn<SchoolOverruns>[] {
  return [
    {
      key: "school",
      header: "Schule",
      render: (school) => (
        <span className="block min-w-0">
          <NavigationLink
            href={traegerPath(`/traeger/schulen/${school.schoolId}`)}
            className="font-medium text-gray-900 hover:underline"
            onClick={(event: MouseEvent<HTMLAnchorElement>) =>
              event.stopPropagation()
            }
          >
            {school.schoolName}
          </NavigationLink>
          {school.dataFrom > to ? (
            <span className="block text-xs text-gray-500">
              Keine Check-ins mehr gespeichert
            </span>
          ) : school.comparedDays === 0 && school.uncomparedDays === 0 ? (
            <span className="block text-xs text-gray-500">
              Keine Check-ins im Zeitraum
            </span>
          ) : school.dataFrom > from ? (
            <span className="block text-xs text-gray-500">
              Check-ins erst ab {formatDate(school.dataFrom)} gespeichert
            </span>
          ) : null}
          {school.uncomparedDays > 0 ? (
            <span className="block text-xs text-gray-500">
              {school.uncomparedDays === 1
                ? "1 Tag nicht verglichen: ohne Plan oder ohne Auschecken"
                : `${school.uncomparedDays} Tage nicht verglichen: ohne Plan oder ohne Auschecken`}
            </span>
          ) : null}
        </span>
      ),
      sortValue: (school) => school.schoolName,
      stacked: "title",
    },
    {
      key: "children",
      header: "Kinder",
      align: "right",
      render: (school) =>
        schoolHasFigures(school, to) ? (
          <OverrunFigure value={school.figures.children} />
        ) : (
          <NoData />
        ),
      sortValue: (school) =>
        schoolHasFigures(school, to) ? school.figures.children : null,
    },
    {
      key: "days",
      header: "Tage",
      align: "right",
      render: (school) =>
        schoolHasFigures(school, to) ? (
          <DaysCell
            overrunDays={school.figures.overrunDays}
            unbookedDays={school.figures.unbookedDays}
          />
        ) : (
          <NoData />
        ),
      sortValue: (school) =>
        schoolHasFigures(school, to) ? overrunDayCount(school.figures) : null,
    },
    {
      key: "extra",
      header: "Zusätzliche Zeit",
      align: "right",
      render: (school) =>
        !schoolHasFigures(school, to) ? (
          <NoData />
        ) : (
          <span className="ml-auto flex w-full max-w-56 flex-col items-end gap-1.5">
            <OverrunFigure value={school.figures.extraMinutes}>
              {formatExtraTime(school.figures.extraMinutes)}
            </OverrunFigure>
            <ProgressBar
              value={school.figures.extraMinutes}
              max={Math.max(maxMinutes, 1)}
              color={LOCATION_COLORS.DANGER}
              size="sm"
              className="w-full"
              label={`${formatExtraTime(school.figures.extraMinutes)} zusätzliche Zeit an ${school.schoolName}`}
            />
          </span>
        ),
      sortValue: (school) =>
        schoolHasFigures(school, to) ? school.figures.extraMinutes : null,
    },
  ];
}

const CHILD_COLUMNS: DataTableColumn<OverrunChild>[] = [
  {
    key: "child",
    header: "Kind",
    render: (child) => (
      <span className="block min-w-0">
        <span className="block font-medium text-gray-900">
          {child.firstName} {child.lastName}
        </span>
        <span className="block text-sm text-gray-500">
          {child.schoolClass
            ? schoolClassLabel(child.schoolClass)
            : "Ohne Klasse"}
          <span className="md:hidden"> · {child.schoolName}</span>
        </span>
      </span>
    ),
    sortValue: (child) => `${child.lastName} ${child.firstName}`,
    stacked: "title",
  },
  {
    key: "school",
    header: "Schule",
    render: (child) => (
      <span className="text-sm text-gray-700">{child.schoolName}</span>
    ),
    sortValue: (child) => child.schoolName,
    stacked: "hidden",
  },
  {
    key: "days",
    header: "Tage",
    align: "right",
    render: (child) => (
      <DaysCell
        overrunDays={child.overrunDays}
        unbookedDays={child.unbookedDays}
      />
    ),
    sortValue: (child) => overrunDayCount(child),
  },
  {
    key: "extra",
    header: "Zusätzliche Zeit",
    align: "right",
    render: (child) => (
      <OverrunFigure value={child.extraMinutes}>
        {formatExtraTime(child.extraMinutes)}
      </OverrunFigure>
    ),
    sortValue: (child) => child.extraMinutes,
    stacked: "meta",
  },
];

/** Leere Kinderliste: „alle Zeiten passen“ nur, wenn jeder Tag verglichen ist. */
function ChildrenEmptyState({
  evaluation,
}: Readonly<{ evaluation: OverrunEvaluation | null }>) {
  const coverage = evaluation ? overrunCoverage(evaluation) : "complete";
  if (coverage === "none") {
    return (
      <EmptyState
        icon={<CalendarOff className="h-12 w-12" aria-hidden="true" />}
        title={evaluation ? noDataTitle(evaluation) : NO_DATA_TITLE}
        description={
          evaluation && noDataTitle(evaluation) === UNCOMPARED_TITLE
            ? "Es fehlt ein Betreuungsplan oder das Auschecken. Die Schulen sehen die Tage bei den Kindern unter Soll und Ist."
            : "Ohne Check-ins kann moto nicht vergleichen. Die Schulen löschen Check-ins nach ihrer Frist."
        }
      />
    );
  }
  if (coverage === "partial") {
    // Not every day was compared: no check mark and no claim for the month.
    return (
      <EmptyState
        icon={<CircleDashed className="h-12 w-12" aria-hidden="true" />}
        title="In den verglichenen Tagen war kein Kind länger da als gebucht."
        description="Verglichen sind nur Tage mit Check-in und Buchung. Andere Tage zählen hier nicht."
      />
    );
  }
  return (
    <EmptyState
      icon={<CircleCheck className="h-12 w-12" aria-hidden="true" />}
      title="Kein Kind war länger da als gebucht."
      description="In diesem Monat passen alle Zeiten zur Buchung."
    />
  );
}

function Figures({ evaluation }: Readonly<{ evaluation: OverrunEvaluation }>) {
  const { total } = evaluation;
  const days = overrunDayCount(total);
  // Without any check-in the figures are unknown, not zero.
  const noData = overrunCoverage(evaluation) === "none";
  return (
    <div className="grid grid-cols-2 gap-4 max-sm:gap-3 lg:grid-cols-3">
      <StatCard
        label="Kinder"
        value={noData ? "–" : total.children}
        tone={!noData && total.children > 0 ? "red" : "neutral"}
        hint={
          noData
            ? noDataTitle(evaluation)
            : total.children === 1
              ? "Ein Kind war länger da als gebucht."
              : `${total.children} Kinder waren länger da als gebucht.`
        }
        icon={<MotoConceptIcon concept={childrenConcept} />}
      />
      <StatCard
        label="Tage"
        value={noData ? "–" : days}
        tone={!noData && days > 0 ? "red" : "neutral"}
        hint={
          noData
            ? "Ohne verglichene Check-ins zählt moto keine Tage."
            : total.unbookedDays > 0
              ? `Davon ${total.unbookedDays} an Tagen ohne Buchung.`
              : "Tage, an denen ein Kind länger da war."
        }
        icon={<MotoConceptIcon concept={calendarConcept} />}
      />
      <div className="col-span-2 lg:col-span-1">
        <StatCard
          label="Zusätzliche Zeit"
          value={noData ? "–" : formatExtraTime(total.extraMinutes)}
          tone={!noData && total.extraMinutes > 0 ? "red" : "neutral"}
          hint={`Zählt ab ${evaluation.toleranceMinutes || 15} Minuten früher oder später.`}
          icon={<MotoConceptIcon concept={careTimesConcept} />}
        />
      </div>
    </div>
  );
}

export default function TraegerZeitvergleichPage() {
  const { data: session, status } = useSession();
  const canSeeNames = hasPermission(session ?? null, OVERRUN_NAMES_RIGHT);
  // The Träger of the session: another tab can switch it under this page.
  const orgId = session?.user?.orgId;
  // Stays mounted across midnight: the window ends today, and the current
  // month follows the calendar while the person is looking at it.
  const today = useBerlinToday();
  const months = useMemo(() => recentMonths(today), [today]);
  const [month, setMonth] = useState(() => monthOf(today));
  const shownToday = useRef(today);
  useEffect(() => {
    const before = monthOf(shownToday.current);
    shownToday.current = today;
    setMonth((current) => (current === before ? monthOf(today) : current));
  }, [today]);
  const [schoolId, setSchoolId] = useState(ALL_SCHOOLS);
  const [evaluation, setEvaluation] = useState<OverrunEvaluation | null>(null);
  const [schools, setSchools] = useState<readonly SchoolOverruns[]>([]);
  const request = useRef(0);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const range = useMemo(() => monthWindow(month, today), [month, today]);

  const load = useCallback(async () => {
    // Without a Träger in the session there is nothing to read; a new
    // Träger is a new read.
    if (orgId === undefined) return;
    const current = ++request.current;
    setEvaluation(null);
    try {
      const query = {
        ...range,
        schoolId: schoolId === ALL_SCHOOLS ? undefined : schoolId,
      };
      const result = canSeeNames
        ? await fetchOverrunChildren(query)
        : await fetchOverrunSummary(query);
      if (current !== request.current) return;
      setEvaluation(result);
      if (schoolId === ALL_SCHOOLS) setSchools(result.schools);
      clearLoadError();
    } catch (err) {
      if (current !== request.current) return;
      logger.error("traeger_care_time_overruns_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Auswertung",
        retry: () => void load(),
      });
    }
  }, [range, schoolId, canSeeNames, orgId, clearLoadError, showLoadError]);

  // A switch of the Träger drops everything the previous one showed and
  // every answer still on its way for it, before the new read starts.
  const shownOrgId = useRef(orgId);
  useEffect(() => {
    if (shownOrgId.current === orgId) return;
    shownOrgId.current = orgId;
    request.current++;
    setEvaluation(null);
    setSchools([]);
    setSchoolId(ALL_SCHOOLS);
  }, [orgId]);

  useEffect(() => {
    if (status === "loading") return;
    void load();
  }, [load, status]);

  const index = months.indexOf(month);
  const older = months[index + 1];
  const newer = index > 0 ? months[index - 1] : undefined;
  const maxMinutes = Math.max(
    0,
    ...(evaluation?.schools ?? []).map((s) => s.figures.extraMinutes),
  );
  const columns = useMemo(
    () => schoolColumns(range.from, range.to, maxMinutes),
    [range.from, range.to, maxMinutes],
  );

  return (
    <TenantPage
      title="Länger als gebucht"
      info={
        <>
          Hier sehen Sie, wie oft Kinder länger da waren als gebucht. moto
          vergleicht dafür die Zeiten im Vertrag mit dem Ein- und Auschecken.
          Ohne Vertrag gilt der Betreuungsplan der Schule. Mehr als 15 Minuten
          früher oder später zählt. Ein Tag ohne Buchung zählt ganz.
        </>
      }
      stats={
        evaluation ? (
          <TenantPageStats
            items={[
              {
                value: `${formatDate(evaluation.from)} bis ${formatDate(evaluation.to)}`,
                label: "",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!evaluation && !pageLoad.error}
      error={pageLoad.error}
      searchSlot={
        <PlanningContextBar
          withoutContextRow
          navigationInGroup
          navigationSlot={
            <CustomSelect
              ariaLabel="Monat"
              value={month}
              onChange={setMonth}
              options={months.map((value) => ({
                value,
                label: monthLabel(value),
              }))}
              className="min-w-0"
            />
          }
          onPrevious={older ? () => setMonth(older) : undefined}
          onNext={newer ? () => setMonth(newer) : undefined}
          previousLabel="Monat davor"
          nextLabel="Monat danach"
          onToday={newer ? () => setMonth(monthOf(today)) : undefined}
          todayLabel="Dieser Monat"
          actions={
            schools.length > 1 ? (
              <CustomSelect
                ariaLabel="Schule"
                value={schoolId}
                onChange={setSchoolId}
                options={[
                  { value: ALL_SCHOOLS, label: "Alle Schulen" },
                  ...schools.map((school) => ({
                    value: school.schoolId,
                    label: school.schoolName,
                  })),
                ]}
                className="min-w-0 md:w-64"
              />
            ) : undefined
          }
        />
      }
    >
      {evaluation ? <Figures evaluation={evaluation} /> : null}
      <DataTable
        heading={{ title: "Je Schule", count: evaluation?.schools.length }}
        columns={columns}
        rows={[...(evaluation?.schools ?? [])]}
        getRowKey={(school) => school.schoolId}
        defaultSortKey="extra"
        defaultSortDirection="desc"
        isLoading={!evaluation && !pageLoad.error}
        loadingRowCount={3}
        stackedOnMobile
      />
      {canSeeNames ? (
        <DataTable
          heading={{ title: "Kinder", count: evaluation?.children.length }}
          columns={CHILD_COLUMNS}
          rows={[...(evaluation?.children ?? [])]}
          getRowKey={(child) => `${child.schoolId}-${child.studentId}`}
          defaultSortKey="extra"
          defaultSortDirection="desc"
          isLoading={!evaluation && !pageLoad.error}
          loadingRowCount={3}
          stackedOnMobile
          emptyState={<ChildrenEmptyState evaluation={evaluation} />}
        />
      ) : (
        <SectionCard
          title="Kinder"
          concept={childrenConcept}
          description="Die Namen der Kinder sehen nur Personen mit dem Recht „Verträge verwalten“. Dieses Recht hat die Rolle Verwaltung."
        />
      )}
    </TenantPage>
  );
}
