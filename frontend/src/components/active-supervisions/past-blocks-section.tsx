"use client";

import { EmptyState } from "~/components/ui/empty-state";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { ChevronDown, History } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import {
  CardGridSkeleton,
  ListSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import { berlinClockFromISO } from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { rosterPickupTimeLabel } from "~/lib/timetable-roster-helpers";
import { isCareDayExpected } from "~/lib/timetable-types";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

const logger = createLogger({ component: "PastBlocksSection" });

// End-of-day review (#2335): today's completed blocks plus planned blocks
// whose end time passed without a start. No start button, no attendance
// actions. The one action is „Wieder öffnen“ on a block that was ended by
// mistake and can still be reopened (#5676).

// Deliberately local, not shared with planned-now-section's rosterStatusLabel:
// that one describes the LIVE state of an upcoming block ("Erwartet"), these
// describe the FINAL state of a finished one ("Gegangen", "Nicht erschienen").
function pastRosterStatusLabel(row: TimetableRosterRow): string {
  if (row.currentlyPresent) return "Anwesend";
  if (row.status === "present") return "Gegangen";
  if (!isCareDayExpected(row.careDayStatus)) {
    return row.careDayStatus === "cancelled"
      ? "Abgemeldet"
      : "Nicht eingeplant";
  }
  if (row.status === "absent") return "Abwesend";
  return "Nicht erschienen";
}

function pastRosterDotColor(row: TimetableRosterRow): string {
  if (row.currentlyPresent) return LOCATION_COLORS.GROUP_ROOM;
  if (row.status === "present") return LOCATION_COLORS.HOME;
  if (!isCareDayExpected(row.careDayStatus)) return LOCATION_COLORS.UNKNOWN;
  if (row.status === "absent") return LOCATION_COLORS.DANGER;
  // "Nicht erschienen" must not share the hex with "Abwesend" (two states on
  // one list, see the color rule); the never-arrived case wears the
  // "Kommt heute nicht" navy.
  return LOCATION_COLORS.NOT_ARRIVAL;
}

interface PastBlocksSectionProps {
  /**
   * Reopens a finished block and shows its own failure; without it the
   * review stays display only.
   */
  readonly onReopen?: (block: PlannedTimetableInstance) => Promise<void>;
}

export function PastBlocksSection({
  onReopen,
}: Readonly<PastBlocksSectionProps> = {}) {
  const [expanded, setExpanded] = useState(false);
  const [blocks, setBlocks] = useState<PlannedTimetableInstance[] | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  // Erhöht von „Wiederholen“ und nach „Wieder öffnen“: lädt die Liste erneut.
  const [attempt, setAttempt] = useState(0);
  // Nach dem Wiederöffnen gehört der Block nicht mehr hierher; scheitert es,
  // zeigt die neue Liste, ob er sich noch öffnen lässt (#5676).
  const reopenAndReload = useMemo(
    () =>
      onReopen
        ? async (block: PlannedTimetableInstance) => {
            await onReopen(block);
            setAttempt((value) => value + 1);
          }
        : undefined,
    [onReopen],
  );

  // Lazy: the list loads when the section opens, and reloads on every open so
  // a block that ended in the meantime shows up without a page refresh.
  useEffect(() => {
    if (!expanded) return;
    let cancelled = false;
    setIsLoading(true);
    clearLoadError();
    timetableOperationsApi
      .plannedNow({ scope: "past" })
      .then((instances) => {
        if (cancelled) return;
        setBlocks(
          [...instances].sort((a, b) => a.startTime.localeCompare(b.startTime)),
        );
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("past_blocks_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showLoadError(err, {
          object: "die Liste der vergangenen Blöcke",
          retry: () => setAttempt((value) => value + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [expanded, attempt, showLoadError, clearLoadError]);

  return (
    <SectionCard
      title="Beendete und abgelaufene Blöcke"
      icon={History}
      collapsible
      defaultCollapsed
      onCollapsedChange={(collapsed) => setExpanded(!collapsed)}
      className="mt-5"
      flush
      bodyClassName="compact:p-4 mt-5 border-t border-gray-100 bg-gray-50/50 p-5 max-sm:p-4"
    >
      {isLoading && blocks === null ? (
        <SkeletonRegion label="Vergangene Blöcke werden geladen">
          <CardGridSkeleton
            cards={4}
            rowsPerCard={2}
            className="grid gap-3 xl:grid-cols-2"
          />
        </SkeletonRegion>
      ) : loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : blocks !== null && blocks.length === 0 ? (
        <p className="text-sm text-gray-600">
          Heute sind noch keine Blöcke beendet oder abgelaufen.
        </p>
      ) : blocks !== null ? (
        <div className="grid gap-3 xl:grid-cols-2">
          {blocks.map((block) => (
            <PastBlockCard
              key={block.id}
              block={block}
              onReopen={reopenAndReload}
            />
          ))}
        </div>
      ) : null}
    </SectionCard>
  );
}

function PastBlockCard({
  block,
  onReopen,
}: Readonly<{
  block: PlannedTimetableInstance;
  onReopen?: (block: PlannedTimetableInstance) => Promise<void>;
}>) {
  const [isReopening, setIsReopening] = useState(false);
  const reopenDeadline =
    block.status === "completed" && block.canReopen && block.reopenUntil
      ? Date.parse(block.reopenUntil)
      : Number.NaN;
  const [reopenExpired, setReopenExpired] = useState(
    () => Number.isFinite(reopenDeadline) && reopenDeadline <= Date.now(),
  );
  // Die Frist läuft auch ab, während die Liste offen bleibt: dann nimmt das
  // Backend das Wiederöffnen nicht mehr an (#5676).
  useEffect(() => {
    if (!Number.isFinite(reopenDeadline)) return;
    const remainingMs = reopenDeadline - Date.now();
    if (remainingMs <= 0) {
      setReopenExpired(true);
      return;
    }
    setReopenExpired(false);
    const timeoutId = window.setTimeout(
      () => setReopenExpired(true),
      remainingMs,
    );
    return () => window.clearTimeout(timeoutId);
  }, [reopenDeadline]);
  const [expanded, setExpanded] = useState(false);
  const [roster, setRoster] = useState<TimetableRoster | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const completed = block.status === "completed";
  const reopenUntil =
    completed &&
    block.canReopen &&
    onReopen &&
    block.reopenUntil &&
    !reopenExpired
      ? berlinClockFromISO(block.reopenUntil)
      : "";
  // „Wiederholen“ lädt die Kinderliste dieses Blocks erneut.
  const reloadRef = useRef<() => void>(() => undefined);

  const loadRoster = useCallback(() => {
    setIsLoading(true);
    clearLoadError();
    timetableOperationsApi
      .roster(block.id)
      .then(setRoster)
      .catch((err: unknown) => {
        logger.error("past_block_roster_load_failed", {
          instance_id: block.id,
          error: err instanceof Error ? err.message : String(err),
        });
        void showLoadError(err, {
          object: "die Kinderliste",
          retry: () => reloadRef.current(),
        });
      })
      .finally(() => setIsLoading(false));
  }, [block.id, showLoadError, clearLoadError]);
  useLayoutEffect(() => {
    reloadRef.current = loadRoster;
  });

  const toggleRoster = () => {
    const next = !expanded;
    setExpanded(next);
    if (next && roster === null && !isLoading) loadRoster();
  };

  return (
    <SectionCard
      title={block.title}
      headingLevel={3}
      action={
        completed ? (
          <StatusBadge label="Beendet" tone="gray" />
        ) : (
          <StatusBadge label="Nicht gestartet" tone="orange" />
        )
      }
      bodyClassName="mt-2"
      footer={
        <>
          <Button
            type="button"
            variant="ghost"
            size="md"
            onClick={toggleRoster}
            className="w-full justify-between"
            aria-expanded={expanded}
          >
            <span>Kinder</span>
            <ChevronDown
              className={`h-4 w-4 text-gray-400 transition-transform ${expanded ? "rotate-180" : ""}`}
              aria-hidden="true"
            />
          </Button>

          {expanded ? (
            <div className="mt-3">
              {isLoading ? (
                <SkeletonRegion label="Kinderliste wird geladen">
                  <ListSkeleton rows={4} avatar={false} />
                </SkeletonRegion>
              ) : loadError ? (
                <LoadErrorAlert error={loadError} />
              ) : roster !== null ? (
                <div className="space-y-2">
                  {!completed ? (
                    <p className="text-moto-amber-strong text-xs">
                      Keine Anwesenheit erfasst: Der Block wurde nicht
                      gestartet.
                    </p>
                  ) : null}
                  {roster.rows.length === 0 ? (
                    <EmptyState
                      variant="compact"
                      description="Keine Kinder eingeplant."
                    />
                  ) : (
                    <>
                      {roster.pickupTimesLoaded === false &&
                      !roster.pickupTimesRedacted ? (
                        <Alert
                          type="warning"
                          announce="polite"
                          message="Die Gehzeiten konnten nicht geladen werden. Die Anwesenheitsliste bleibt verfügbar."
                        />
                      ) : null}
                      {roster.rows.map((row) => (
                        <PastRosterRow
                          key={row.studentId}
                          row={row}
                          pickupTimesLoaded={roster.pickupTimesLoaded}
                          pickupTimesRedacted={roster.pickupTimesRedacted}
                          showStatus={completed}
                        />
                      ))}
                    </>
                  )}
                </div>
              ) : null}
            </div>
          ) : null}
        </>
      }
    >
      <div className="flex flex-wrap gap-3 text-sm text-gray-600">
        <span>
          {block.startTime}-{block.endTime}
        </span>
        <span>{block.roomName ?? `Raum ${block.roomId}`}</span>
        <span>
          {block.assignedStaffIds.length}{" "}
          {block.assignedStaffIds.length === 1 ? "Betreuer" : "Betreuende"}
        </span>
      </div>
      {reopenUntil && onReopen ? (
        <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-gray-700">
            Aus Versehen beendet? Sie können ihn bis {reopenUntil} Uhr wieder
            öffnen.
          </p>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => {
              setIsReopening(true);
              void onReopen(block).finally(() => setIsReopening(false));
            }}
            isLoading={isReopening}
            loadingText="Öffne wieder …"
            disabled={isReopening}
            aria-label={`${block.title} wieder öffnen`}
          >
            Wieder öffnen
          </Button>
        </div>
      ) : null}
    </SectionCard>
  );
}

function PastRosterRow({
  row,
  pickupTimesLoaded,
  pickupTimesRedacted,
  showStatus,
}: Readonly<{
  row: TimetableRosterRow;
  pickupTimesLoaded?: boolean;
  pickupTimesRedacted?: boolean;
  showStatus: boolean;
}>) {
  const pickupTimeLabel = rosterPickupTimeLabel(
    row.pickupTime,
    pickupTimesLoaded,
    pickupTimesRedacted,
  );
  return (
    <div className="rounded-lg bg-white px-3 py-2 text-sm shadow-[0_1px_0_rgba(17,24,39,0.04)]">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate font-medium text-gray-900">
            {row.studentName || `Kind ${row.studentId}`}
          </p>
          <p className="mt-0.5 truncate text-xs text-gray-500">
            {[row.schoolClass, row.groupName].filter(Boolean).join(" · ") ||
              "Ohne Klassengruppe"}
          </p>
          {pickupTimeLabel === null ? null : (
            <p className="mt-0.5 text-xs font-medium text-gray-700">
              Gehzeit: {pickupTimeLabel}
            </p>
          )}
        </div>
        {showStatus ? (
          <StatusColorBadge
            label={pastRosterStatusLabel(row)}
            color={pastRosterDotColor(row)}
          />
        ) : null}
      </div>
    </div>
  );
}
