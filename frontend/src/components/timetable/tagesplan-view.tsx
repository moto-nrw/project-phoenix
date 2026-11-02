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
  type CSSProperties,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useId,
  useRef,
  useState,
} from "react";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { MobilePageHelpTarget } from "~/components/help/mobile-page-help-target";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import {
  OwnBlockHint,
  blockRowStyle,
  blockStaffNames,
} from "~/components/timetable/block-row-parts";
import {
  type TagesplanGroup,
  groupParallelBlocks,
} from "~/components/timetable/tagesplan-groups";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { blockChildrenLabel, blockEndLine } from "~/lib/block-children";
import {
  formatDate,
  formatStatusDate,
  isValidISODate,
} from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { GROUP_ROOM_SHADES, LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { STAFF_FLAT_PAGES } from "~/lib/section-navigation";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { useSWRAuth } from "~/lib/swr/hooks";
import {
  useOperationalOverviewScope,
  useTimetableEnabled,
  useWeekendFollowsFriday,
} from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import { nextCareDayISO, previousCareDayISO } from "~/lib/timetable-helpers";
import { canStartPlannedInstance } from "~/lib/timetable-lifecycle";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import { roomsConcept } from "~/lib/concepts/rooms";
import { staffConcept } from "~/lib/concepts/staff";

const logger = createLogger({ component: "TagesplanView" });

// Zwischen Navigationen gemerkte Scrollposition (#2383): wer aus einer
// Kinderliste zurückkommt, landet wieder beim Block, den er angetippt hat.
const SCROLL_STORAGE_KEY = "tagesplan-scroll";

// Genau zwei farbige Signale auf der ganzen Seite: das grüne "Läuft" und der
// Starten-Knopf. Beendetes und Verpasstes wird gedimmt statt etikettiert,
// eine Absage steht als rote Textzeile ("Fällt aus · Grund") in der Zeile —
// DANGER, nicht SICK: der Termin fällt aus, niemand ist krank.

// Nur echte Kalendertage aus ?d= übernehmen: die geteilte Prüfung weist auch
// formgültige, aber unmögliche Daten wie "2026-02-31" ab, statt sie an die
// API weiterzureichen (dort gäbe es nur einen 400).
function isValidISODay(value: string | null): value is string {
  return value != null && isValidISODate(value);
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

// Kinderzahl so knapp wie möglich — der Tagesplan soll sich in einem Blick
// lesen, nicht in Sätzen ("4 Kinder" statt "4 Kinder erwartet"). Laufend
// zählt, wer gerade da ist, und wer gegangen ist, steht daneben (#3921).
function childrenShort(instance: PlannedTimetableInstance): string | null {
  if (instance.status === "cancelled") return null;
  return blockChildrenLabel(instance);
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
        className="text-xs font-semibold tabular-nums"
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

const TIME_CLASS = "w-[4.25rem] shrink-0 sm:w-24";
// Schmale Spalte: Start und Ende in einer Zeile, ab @xs wieder als Spalte.
const TIME_PARALLEL_CLASS =
  "flex items-baseline gap-2 @xs:block @xs:w-24 @xs:shrink-0";

function TagesplanRow({
  instance,
  effectiveEndTime,
  isToday,
  dayIsPast,
  nowHHMM,
  showMine,
  startBusy,
  onOpenSession,
  onStart,
  placement,
}: Readonly<{
  instance: PlannedTimetableInstance;
  /** Normalisiertes Ende für die Statusanzeige; die angezeigte Zeit bleibt unverändert. */
  effectiveEndTime: string;
  isToday: boolean;
  showMine: boolean;
  dayIsPast: boolean;
  nowHHMM: string;
  startBusy: boolean;
  onOpenSession: (activeGroupId: string) => void;
  onStart: (instance: PlannedTimetableInstance) => void;
  /** Platz im Raster einer Gruppe zeitgleicher Blöcke (#5020). */
  placement?: CSSProperties;
}>) {
  const parallel = placement != null;
  // Ein Dienst (#3822) hat keine Kinder und wird nie gestartet; er kann
  // auch ohne Raum stattfinden.
  const isDuty = instance.isDuty === true;
  const room =
    instance.roomName ??
    (isDuty && instance.roomId === "0" ? null : `Raum ${instance.roomId}`);
  const running = instance.status === "active";
  const cancelled = instance.status === "cancelled";
  const openable = running && instance.activeGroupId != null;
  const startable =
    !isDuty &&
    isToday &&
    instance.status === "planned" &&
    canStartPlannedInstance(instance, new Date());
  const ended =
    instance.status === "planned" &&
    (dayIsPast || (isToday && effectiveEndTime <= nowHHMM));
  const missed = ended && !isDuty;
  // Vorbei ist vorbei: gedimmte Zeilen lassen das Laufende und Kommende von
  // selbst hervortreten — statt eines Etiketts an jeder Zeile.
  const over =
    instance.status === "completed" ||
    ended ||
    (cancelled && (dayIsPast || (isToday && effectiveEndTime <= nowHHMM)));
  const staff = blockStaffNames(instance);
  // Eigener Block (#4350): die Zeile ist in ihrer Blockfarbe hinterlegt, wie
  // ein PlanBlock im Raster; Vorleseprogramme hören "Mein Block". isAssigned
  // gilt auch für eine Vertretung, nur wer abwesend gemeldet ist, fällt
  // heraus. Sieht jemand ohnehin nur die eigene Einteilung, wäre jede Zeile
  // hinterlegt und die Farbe sagte nichts mehr.
  const mine = showMine && instance.isAssigned;

  // Der Raum trägt sein Konzeptsymbol mitten in der Metazeile. Die Symbole
  // sind Schmuck: der Text daneben sagt schon, was gemeint ist.
  const metaParts: ReactNode[] = [
    instance.status === "completed" ? "Beendet" : null,
    missed ? "Nicht gestartet" : null,
    isDuty ? "Dienst" : null,
    room ? (
      <Fragment key="room">
        <MotoConceptIcon
          concept={roomsConcept}
          size={14}
          className="mr-1 inline-block align-[-2px]"
        />
        {room}
      </Fragment>
    ) : null,
    instance.groupName,
    cancelled || isDuty ? null : childrenShort(instance),
  ].filter(Boolean);

  const body = (
    <>
      <span
        className={`${parallel ? TIME_PARALLEL_CLASS : TIME_CLASS} ${over ? "opacity-50" : ""}`}
      >
        <span className="block text-sm font-semibold text-gray-900 tabular-nums">
          {instance.startTime}
        </span>
        <span className="block text-xs text-gray-500 tabular-nums">
          {blockEndLine(instance)}
        </span>
      </span>
      <span
        className={`${parallel ? "w-full min-w-0 @xs:w-auto @xs:flex-1" : "min-w-0 flex-1"} ${over ? "opacity-50" : ""}`}
      >
        <span
          className={`block truncate text-sm text-gray-900 ${running ? "font-semibold" : "font-medium"}`}
        >
          {instance.title}
          <OwnBlockHint mine={mine} />
          {instance.planningTrackName ? (
            <span className="ml-2 text-xs font-normal text-gray-500">
              {instance.planningTrackName}
            </span>
          ) : null}
        </span>
        {cancelled ? (
          <span
            className="block truncate text-xs font-medium"
            style={{ color: LOCATION_COLORS.DANGER }}
          >
            {instance.cancelReason
              ? `Fällt aus · ${instance.cancelReason}`
              : "Fällt aus"}
          </span>
        ) : null}
        <span className="block truncate text-xs text-gray-500">
          {metaParts.flatMap((part, index) =>
            index === 0 ? [part] : [" · ", part],
          )}
        </span>
        {staff && !cancelled ? (
          <span className="block truncate text-xs text-gray-400">
            <MotoConceptIcon
              concept={staffConcept}
              size={14}
              className="mr-1 inline-block align-[-2px]"
            />
            {staff}
          </span>
        ) : null}
      </span>
    </>
  );
  const badge = running ? (
    <span className="shrink-0">
      <StatusColorBadge label="Läuft" color={LOCATION_COLORS.GROUP_ROOM} />
    </span>
  ) : null;
  const chevron = (
    <ChevronRight
      className="h-4 w-4 shrink-0 text-gray-400"
      aria-hidden="true"
    />
  );
  const startButton = startable ? (
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
  ) : null;

  const edgeStyle = {
    ...blockRowStyle(instance, mine),
    ...(over ? { opacity: 0.9 } : {}),
  };

  // In einer schmalen Spalte stehen Zeit, Inhalt und Aktion untereinander;
  // Badge, Chevron und Starten-Knopf rücken zusammen ans Ende.
  if (parallel) {
    const trailing =
      badge || openable || startButton ? (
        <span className="flex shrink-0 items-center gap-3">
          {badge}
          {openable ? chevron : null}
          {startButton}
        </span>
      ) : null;
    const inner =
      "flex h-full w-full flex-col items-start gap-2 px-3 py-3 @xs:flex-row @xs:items-center @xs:gap-3 @xs:px-4 @xs:py-3.5";
    return (
      <li
        className="@container border-b border-l-4 border-gray-100"
        style={{ ...edgeStyle, ...placement }}
      >
        {openable ? (
          <button
            type="button"
            onClick={() => onOpenSession(instance.activeGroupId as string)}
            className={`${inner} text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset`}
          >
            {body}
            {trailing}
          </button>
        ) : (
          <div className={inner}>
            {body}
            {trailing}
          </div>
        )}
      </li>
    );
  }

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
          {badge}
          {chevron}
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
      {badge}
      {startButton}
    </li>
  );
}

// Mehr als zwei Spalten werden auf dem Handy zu schmal: ab drei hat jede
// Spalte eine Mindestbreite und die Gruppe scrollt seitlich (#5020).
const PARALLEL_SCROLL_FROM = 3;

function ParallelGroup({
  group,
  renderRow,
}: Readonly<{
  group: TagesplanGroup;
  renderRow: (
    instance: PlannedTimetableInstance,
    placement: CSSProperties,
    effectiveEndTime: string,
  ) => ReactNode;
}>) {
  const captionId = useId();
  const scrolls = group.laneCount >= PARALLEL_SCROLL_FROM;
  const span = group.openEnd
    ? `ab ${group.startTime} Uhr`
    : `${group.startTime}–${group.endTime} Uhr`;
  return (
    <li className="border-b border-gray-100 last:border-b-0">
      <p
        id={captionId}
        className="px-4 pt-2 pb-1 text-xs font-medium text-gray-500 tabular-nums"
      >
        Gleichzeitig · {span}
      </p>
      <div
        className={scrolls ? "overflow-x-auto overflow-y-hidden" : undefined}
      >
        <ul
          aria-labelledby={captionId}
          className="-mb-px grid"
          style={{
            gridTemplateColumns: `repeat(${group.laneCount}, ${scrolls ? "minmax(11rem, 1fr)" : "minmax(0, 1fr)"})`,
          }}
        >
          {group.cells.map((cell) => (
            <Fragment key={cell.instance.id}>
              {renderRow(
                cell.instance,
                {
                  gridColumn: `${cell.lane + 1} / span ${cell.laneSpan}`,
                  gridRow: `${cell.row + 1}`,
                },
                cell.effectiveEndTime,
              )}
            </Fragment>
          ))}
        </ul>
      </div>
    </li>
  );
}

export function TagesplanView() {
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  const timetableEnabled = useTimetableEnabled();
  const overviewScope = useOperationalOverviewScope();
  const weekendOpen = useWeekendFollowsFriday();
  const now = useMinuteClock();

  const today = useBerlinToday();
  const dayParam = searchParams.get("d");
  const day = isValidISODay(dayParam) ? dayParam : today;
  const isToday = day === today;
  const nowHHMM = berlinNowHHMM(now);

  const [startBusyId, setStartBusyId] = useState<string | null>(null);
  // Starten ist ein Knopf in der Liste, kein Formular: ein Fehler kommt als
  // Toast mit Katalogtext (#2516).
  const { show: showActionError } = useApiErrorDisplay();
  const latestStartRef = useRef<(instance: PlannedTimetableInstance) => void>(
    () => undefined,
  );

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
  // Ladefehler vor Ort mit Wiederholen (#2516). Fehlende Rechte nennt der
  // Katalogtext der Klasse, keine eigene Leerseite je HTTP-Status.
  const listLoadError = useSwrLoadError(
    listError,
    "die Liste der Termine",
    () => reloadList(),
  );

  // Zeitgleiche Blöcke bilden eine Gruppe und stehen nebeneinander (#5020);
  // ein Block ohne Überschneidung ist eine Gruppe für sich in voller Breite.
  const groups = useMemo(
    () => groupParallelBlocks(instances ?? [], isToday ? nowHHMM : null),
    [instances, isToday, nowHHMM],
  );

  // "Jetzt"-Markierung: vor der ersten Gruppe, die noch nicht vorbei ist.
  const nowIndex = useMemo(() => {
    if (!isToday || groups.length === 0) return -1;
    const index = groups.findIndex((group) => group.endTime > nowHHMM);
    return index === -1 ? groups.length : index;
  }, [isToday, groups, nowHHMM]);

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
      router.replace(iso === today ? "/tagesplan" : `/tagesplan?d=${iso}`);
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
      try {
        const result = await timetableOperationsApi.start(instance.id);
        // Die Seitenleiste soll die neue Aufsicht sofort zeigen, auch ohne
        // Live-Ereignis (etwa für die AG-Leitung, #4573).
        window.dispatchEvent(new CustomEvent("phoenix:supervision-stale"));
        saveScrollPosition(day);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
      } catch (err) {
        logger.error("tagesplan_start_failed", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        void showActionError(err, {
          object: "das Starten des Termins",
          retry: () => latestStartRef.current(instance),
        });
        await reloadList();
      } finally {
        setStartBusyId(null);
      }
    },
    [day, reloadList, router, showActionError],
  );

  useLayoutEffect(() => {
    latestStartRef.current = (instance) => void handleStart(instance);
  });

  const renderRow = (
    instance: PlannedTimetableInstance,
    placement?: CSSProperties,
    effectiveEndTime: string = instance.endTime,
  ) => (
    <TagesplanRow
      key={instance.id}
      instance={instance}
      effectiveEndTime={effectiveEndTime}
      isToday={isToday}
      dayIsPast={day < today}
      nowHHMM={nowHHMM}
      showMine={overviewScope === "all_staff"}
      startBusy={startBusyId === instance.id}
      onOpenSession={openSession}
      onStart={(entry) => void handleStart(entry)}
      placement={placement}
    />
  );

  if (!timetableEnabled) {
    return (
      <PlanningDisabledState
        pageTitle={STAFF_FLAT_PAGES.tagesplan.label}
        heading="Der Betreuungsplan ist ausgeschaltet"
        description="Ihre Schule nutzt den Betreuungsplan zurzeit nicht. Ihre Aufsichten finden Sie unter „Aktuelle Aufsicht“."
        testId="tagesplan-disabled"
      />
    );
  }

  // Die Liste erklärt sich selbst (Chevron am laufenden Block, Starten-Knopf
  // am eigenen). Nur die eingeschränkte Sicht (#2380) muss gesagt werden,
  // sonst fehlen scheinbar Termine.
  const description =
    overviewScope === "all_staff"
      ? undefined
      : "Ihre Einsätze. Sie sehen nur Termine, für die Sie eingeteilt sind.";

  return (
    <div className="w-full space-y-4">
      <SectionCard
        title={isToday ? "Heute" : formatDate(day)}
        // Welcher Tag „Heute“ ist, steht direkt daneben (#4348). Ein anderer
        // Tag trägt sein Datum schon im Titel.
        titleBadge={
          isToday ? (
            <span className="text-sm text-gray-500 tabular-nums">
              {formatStatusDate(day)}
            </span>
          ) : undefined
        }
        description={description}
        headingLevel={1}
        actions={
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label="Vorheriger Tag"
              onClick={() =>
                goToDay(previousCareDayISO(day, weekendOpen, today))
              }
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
              onClick={() => goToDay(nextCareDayISO(day, weekendOpen, today))}
            >
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            </Button>
            <MobilePageHelpTarget />
          </div>
        }
      >
        {/* Solange die Session lädt, fetcht useSWRAuth noch nicht — dann ist
            isLoading false UND instances leer. Ohne diese Bedingung würde
            kurz "keine Betreuung geplant" aufblitzen, obwohl nie geladen
            wurde. */}
        {/* Bis der Katalogtext eines Ladefehlers da ist, bleibt das Skelett
            stehen: kein kurzer Leerzustand ohne Daten. */}
        {isLoading ||
        (instances == null && !listError) ||
        (listError && !listLoadError) ? (
          <Skeleton className="h-64 w-full" />
        ) : null}

        {!isLoading && listError ? (
          <LoadErrorAlert error={listLoadError} />
        ) : null}

        {!isLoading &&
        !listError &&
        instances != null &&
        groups.length === 0 ? (
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

        {!isLoading && !listError && groups.length > 0 ? (
          <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
            <ul>
              {groups.map((group, index) => (
                <Fragment key={group.key}>
                  {index === nowIndex ? <NowDivider nowHHMM={nowHHMM} /> : null}
                  {group.cells.length === 1 ? (
                    renderRow(
                      group.cells[0]!.instance,
                      undefined,
                      group.cells[0]!.effectiveEndTime,
                    )
                  ) : (
                    <ParallelGroup group={group} renderRow={renderRow} />
                  )}
                </Fragment>
              ))}
              {nowIndex === groups.length ? (
                <NowDivider nowHHMM={nowHHMM} />
              ) : null}
            </ul>
          </div>
        ) : null}
      </SectionCard>
    </div>
  );
}
