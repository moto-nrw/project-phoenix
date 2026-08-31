"use client";

// Tages-Betreuungsplan (#2383): der Einstieg der Betreuungskräfte in den
// laufenden Tag. Eine chronologische Liste der Betreuungsblöcke des Tages —
// vergangene, laufende und kommende. Ein Tipp auf einen laufenden Block
// öffnet genau dessen Kinderliste in "Aktuelle Aufsicht"; ein eigener,
// noch nicht gestarteter Block wird über den bestehenden Start-Flow
// gestartet (POST /operations/instances/{id}/start) und öffnet danach
// dieselbe Liste. Alles andere ist bewusst reine Anzeige: kein Chevron,
// kein Hover — was man nicht öffnen kann, sieht auch nicht öffenbar aus.
//
// Wer welche Blöcke sieht, entscheidet der Server über
// operations.operational_overview_scope (#2380): bei "all_staff" den ganzen
// Tag der Schule, sonst nur die eigene Einteilung. Die Seite blendet nichts
// selbst aus und zeigt deshalb nie einen Block, dessen Öffnen mit 403
// scheitern würde.

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useSearchParams } from "next/navigation";
import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { GROUP_ROOM_SHADES, LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { useSWRAuth } from "~/lib/swr/hooks";
import {
  useOperationalOverviewScope,
  useTimetableEnabled,
} from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import { nextWorkdayISO, previousWorkdayISO } from "~/lib/timetable-helpers";
import { canStartPlannedInstance } from "~/lib/timetable-lifecycle";
import {
  TimetableOperationsApiError,
  timetableOperationsApi,
} from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const logger = createLogger({ component: "TagesplanView" });

const GENERIC_ERROR =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";

// Zwischen Navigationen gemerkte Scrollposition (#2383): wer aus einer
// Kinderliste zurückkommt, landet wieder beim Block, den er angetippt hat.
const SCROLL_STORAGE_KEY = "tagesplan-scroll";

// Nur abweichende Zustände tragen ein Etikett — "geplant" ist der Normalfall.
const STATUS_BADGES: Partial<
  Record<PlannedTimetableInstance["status"], { label: string; color: string }>
> = {
  active: { label: "Läuft", color: LOCATION_COLORS.GROUP_ROOM },
  completed: { label: "Beendet", color: LOCATION_COLORS.OTHER_ROOM },
  // DANGER, nicht SICK: der Termin fällt aus, niemand ist krank.
  cancelled: { label: "Fällt aus", color: LOCATION_COLORS.DANGER },
};

function isValidISODay(value: string | null): value is string {
  return value != null && /^\d{4}-\d{2}-\d{2}$/.test(value);
}

function berlinNowHHMM(at: Date): string {
  return new Intl.DateTimeFormat("de-DE", {
    timeZone: "Europe/Berlin",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(at);
}

function saveScrollPosition(day: string) {
  try {
    sessionStorage.setItem(
      SCROLL_STORAGE_KEY,
      JSON.stringify({ day, y: window.scrollY }),
    );
  } catch {
    // Ohne Storage geht nur die Scrollposition verloren, nicht die Funktion.
  }
}

function readScrollPosition(day: string): number | null {
  try {
    const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as { day?: string; y?: number };
    if (parsed.day !== day || typeof parsed.y !== "number") return null;
    sessionStorage.removeItem(SCROLL_STORAGE_KEY);
    return parsed.y;
  } catch {
    return null;
  }
}

function staffLine(instance: PlannedTimetableInstance): string | null {
  const names = instance.staffNames ?? [];
  if (names.length === 0) return null;
  return names
    .map((entry) =>
      entry.isSubstitute
        ? `${entry.displayName} (Vertretung)`
        : entry.displayName,
    )
    .join(", ");
}

function childrenLine(instance: PlannedTimetableInstance): string | null {
  if (instance.status === "cancelled") return null;
  if (instance.status === "active") {
    return `${instance.presentStudentsCount} von ${instance.expectedStudentsCount} Kindern da`;
  }
  if (instance.status === "completed") {
    // Nach dem Beenden trägt der Block keinen Erwartet-Stand mehr — nur wer
    // da war, ist noch eine sinnvolle Zahl.
    return instance.presentStudentsCount === 1
      ? "1 Kind war da"
      : `${instance.presentStudentsCount} Kinder waren da`;
  }
  return instance.expectedStudentsCount === 1
    ? "1 Kind erwartet"
    : `${instance.expectedStudentsCount} Kinder erwartet`;
}

// Die "Jetzt"-Linie: markiert die aktuelle Uhrzeit zwischen vergangenen und
// kommenden Blöcken, damit der laufende Zeitabschnitt sofort erkennbar ist.
function NowDivider({ nowHHMM }: Readonly<{ nowHHMM: string }>) {
  return (
    <li className="flex items-center gap-2 px-4 py-1" aria-hidden="true">
      <span
        className="h-px flex-1"
        style={{ backgroundColor: LOCATION_COLORS.GROUP_ROOM }}
      />
      <span
        className="text-[11px] font-semibold tabular-nums"
        style={{ color: GROUP_ROOM_SHADES.text }}
      >
        Jetzt · {nowHHMM} Uhr
      </span>
      <span
        className="h-px flex-1"
        style={{ backgroundColor: LOCATION_COLORS.GROUP_ROOM }}
      />
    </li>
  );
}

function TagesplanRow({
  instance,
  isToday,
  nowHHMM,
  startBusy,
  onOpenSession,
  onStart,
}: Readonly<{
  instance: PlannedTimetableInstance;
  isToday: boolean;
  nowHHMM: string;
  startBusy: boolean;
  onOpenSession: (activeGroupId: string) => void;
  onStart: (instance: PlannedTimetableInstance) => void;
}>) {
  const room = instance.roomName ?? `Raum ${instance.roomId}`;
  const badge = STATUS_BADGES[instance.status];
  const running = instance.status === "active";
  const openable = running && instance.activeGroupId != null;
  const startable =
    isToday &&
    instance.status === "planned" &&
    canStartPlannedInstance(instance, new Date());
  const missed =
    instance.status === "planned" && isToday && instance.endTime <= nowHHMM;
  const staff = staffLine(instance);
  const children = childrenLine(instance);

  const details = [room, instance.groupName, children]
    .filter(Boolean)
    .join(" · ");

  const body = (
    <>
      <span className="w-[4.25rem] shrink-0 sm:w-24">
        <span className="block text-sm font-semibold text-gray-900 tabular-nums">
          {instance.startTime}
        </span>
        <span className="block text-xs text-gray-500 tabular-nums">
          bis {instance.endTime}
        </span>
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium text-gray-900">
          {instance.title}
          {instance.planningTrackName ? (
            <span className="ml-2 text-xs font-normal text-gray-500">
              {instance.planningTrackName}
            </span>
          ) : null}
        </span>
        <span className="block truncate text-xs text-gray-500">{details}</span>
        {staff ? (
          <span className="block truncate text-xs text-gray-500">{staff}</span>
        ) : null}
        {instance.status === "cancelled" ? (
          <span className="block truncate text-xs text-gray-600">
            {instance.cancelReason
              ? `Fällt aus: ${instance.cancelReason}`
              : "Dieser Termin findet nicht statt."}
          </span>
        ) : null}
        {missed ? (
          <span className="block text-xs text-gray-500">Nicht gestartet</span>
        ) : null}
        <span className="mt-1 flex items-center gap-2 sm:hidden">
          {badge ? (
            <StatusDotBadge label={badge.label} color={badge.color} />
          ) : null}
        </span>
      </span>
      {badge ? (
        <span className="hidden shrink-0 sm:flex">
          <StatusDotBadge label={badge.label} color={badge.color} />
        </span>
      ) : null}
    </>
  );

  const edgeStyle = {
    borderLeftColor:
      instance.status === "cancelled"
        ? LOCATION_COLORS.DANGER
        : (instance.planningTrackColor ?? "#E5E7EB"),
  };

  if (openable) {
    return (
      <li>
        <button
          type="button"
          onClick={() => onOpenSession(instance.activeGroupId as string)}
          className="flex w-full items-center gap-3 border-b border-l-4 border-gray-100 px-4 py-3.5 text-left transition-colors last:border-b-0 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          style={edgeStyle}
        >
          {body}
          <ChevronRight
            className="h-4 w-4 shrink-0 text-gray-400"
            aria-hidden="true"
          />
        </button>
      </li>
    );
  }

  return (
    <li
      className="flex w-full items-center gap-3 border-b border-l-4 border-gray-100 px-4 py-3.5 last:border-b-0"
      style={edgeStyle}
    >
      {body}
      {startable ? (
        <Button
          type="button"
          size="md"
          variant="success"
          className="shrink-0"
          disabled={startBusy}
          onClick={() => onStart(instance)}
        >
          {startBusy ? "Startet..." : "Starten"}
        </Button>
      ) : null}
    </li>
  );
}

export function TagesplanView() {
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  const timetableEnabled = useTimetableEnabled();
  const overviewScope = useOperationalOverviewScope();
  const now = useMinuteClock();

  const today = berlinTodayISO();
  const dayParam = searchParams.get("d");
  const day = isValidISODay(dayParam) ? dayParam : today;
  const isToday = day === today;
  const nowHHMM = berlinNowHHMM(now);

  const [startBusyId, setStartBusyId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const {
    data: instances,
    isLoading,
    error: listError,
    mutate: reloadList,
  } = useSWRAuth(
    timetableEnabled ? `tagesplan-${day}` : null,
    () => timetableOperationsApi.plannedNow({ scope: "day", date: day }),
    { revalidateOnFocus: true, focusThrottleInterval: 60_000 },
  );

  const sorted = useMemo(
    () =>
      [...(instances ?? [])].sort(
        (a, b) =>
          a.startTime.localeCompare(b.startTime) ||
          a.endTime.localeCompare(b.endTime) ||
          a.title.localeCompare(b.title),
      ),
    [instances],
  );

  // "Jetzt"-Markierung: vor dem ersten Block, der noch nicht vorbei ist.
  const nowIndex = useMemo(() => {
    if (!isToday || sorted.length === 0) return -1;
    const index = sorted.findIndex((entry) => entry.endTime > nowHHMM);
    return index === -1 ? sorted.length : index;
  }, [isToday, sorted, nowHHMM]);

  // Zurück aus einer Kinderliste: Scrollposition des angetippten Blocks
  // wiederherstellen, sobald die Liste da ist.
  const restoredRef = useRef(false);
  useEffect(() => {
    if (restoredRef.current || instances == null) return;
    restoredRef.current = true;
    const y = readScrollPosition(day);
    if (y != null) window.scrollTo({ top: y });
  }, [instances, day]);

  const goToDay = useCallback(
    (iso: string) => {
      restoredRef.current = true;
      router.replace(
        iso === today ? "/betreuungsplan/tag" : `/betreuungsplan/tag?d=${iso}`,
      );
    },
    [router, today],
  );

  const openSession = useCallback(
    (activeGroupId: string) => {
      saveScrollPosition(day);
      router.push(`/active-supervisions?session=${activeGroupId}`);
    },
    [day, router],
  );

  const handleStart = useCallback(
    async (instance: PlannedTimetableInstance) => {
      setStartBusyId(instance.id);
      setActionError(null);
      try {
        const result = await timetableOperationsApi.start(instance.id);
        saveScrollPosition(day);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
      } catch (err) {
        logger.error("tagesplan_start_failed", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        setActionError(GENERIC_ERROR);
        await reloadList();
      } finally {
        setStartBusyId(null);
      }
    },
    [day, reloadList, router],
  );

  if (!timetableEnabled) {
    return (
      <PlanningDisabledState
        pageTitle="Tagesplan"
        heading="Der Betreuungsplan ist ausgeschaltet"
        description="Ihre Schule nutzt den Betreuungsplan zurzeit nicht. Ihre Aufsichten finden Sie unter „Aktuelle Aufsicht“."
        testId="tagesplan-disabled"
      />
    );
  }

  const forbidden =
    listError instanceof TimetableOperationsApiError &&
    listError.httpStatus === 403;

  const description = isToday
    ? overviewScope === "all_staff"
      ? "Alle Betreuungsblöcke von heute. Tippen Sie auf einen laufenden Block, um seine Kinderliste zu öffnen."
      : "Ihre Betreuungsblöcke von heute. Sie sehen nur Termine, für die Sie eingeteilt sind. Tippen Sie auf einen laufenden Block, um seine Kinderliste zu öffnen."
    : overviewScope === "all_staff"
      ? `Alle Betreuungsblöcke am ${formatDate(day)}.`
      : `Ihre Betreuungsblöcke am ${formatDate(day)}. Sie sehen nur Termine, für die Sie eingeteilt sind.`;

  return (
    <div className="w-full space-y-4">
      {actionError ? <Alert type="error" message={actionError} /> : null}

      <SectionCard
        kicker="Betreuungsplan"
        title={isToday ? "Heute" : formatDate(day)}
        description={description}
        headingLevel={1}
        actions={
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label="Vorheriger Tag"
              onClick={() => goToDay(previousWorkdayISO(day))}
            >
              <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            </Button>
            {!isToday ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={() => goToDay(today)}
              >
                Heute
              </Button>
            ) : null}
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label="Nächster Tag"
              onClick={() => goToDay(nextWorkdayISO(day))}
            >
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        }
      >
        {isLoading ? <Skeleton className="h-64 w-full" /> : null}

        {!isLoading && forbidden ? (
          <EmptyState
            title="Kein Zugriff auf den Betreuungsplan"
            description="Ihr Konto darf den Betreuungsplan nicht öffnen. Wenden Sie sich an die Leitung Ihrer Schule."
          />
        ) : null}

        {!isLoading && listError && !forbidden ? (
          <div className="space-y-3">
            <Alert
              type="error"
              message="Der Tagesplan konnte nicht geladen werden."
            />
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void reloadList()}
            >
              Noch einmal versuchen
            </Button>
          </div>
        ) : null}

        {!isLoading && !listError && sorted.length === 0 ? (
          <EmptyState
            title={
              isToday
                ? "Heute ist keine Betreuung geplant"
                : "An diesem Tag ist keine Betreuung geplant"
            }
            description={
              overviewScope === "all_staff"
                ? "Sobald für diesen Tag Termine im Betreuungsplan stehen, sehen Sie sie hier."
                : "Für diesen Tag sind Sie im Betreuungsplan nicht eingeteilt. Die Einteilung machen die Admins Ihrer Schule."
            }
          />
        ) : null}

        {!isLoading && !listError && sorted.length > 0 ? (
          <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white">
            <ul>
              {sorted.map((instance, index) => (
                <Fragment key={instance.id}>
                  {index === nowIndex ? <NowDivider nowHHMM={nowHHMM} /> : null}
                  <TagesplanRow
                    instance={instance}
                    isToday={isToday}
                    nowHHMM={nowHHMM}
                    startBusy={startBusyId === instance.id}
                    onOpenSession={openSession}
                    onStart={(entry) => void handleStart(entry)}
                  />
                </Fragment>
              ))}
              {nowIndex === sorted.length ? (
                <NowDivider nowHHMM={nowHHMM} />
              ) : null}
            </ul>
          </div>
        ) : null}
      </SectionCard>
    </div>
  );
}
