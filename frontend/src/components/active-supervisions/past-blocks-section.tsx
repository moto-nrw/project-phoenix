"use client";

import { useCallback, useEffect, useState } from "react";
import { ChevronDown, History } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { isCareDayExpected } from "~/lib/timetable-types";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

const logger = createLogger({ component: "PastBlocksSection" });

// Read-only end-of-day review (#2335): today's completed blocks plus planned
// blocks whose end time passed without a start. Everything here is display
// only — no start button, no attendance actions.

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
  return LOCATION_COLORS.DANGER;
}

export function PastBlocksSection() {
  const [expanded, setExpanded] = useState(false);
  const [blocks, setBlocks] = useState<PlannedTimetableInstance[] | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  // Lazy: the list loads when the section opens, and reloads on every open so
  // a block that ended in the meantime shows up without a page refresh.
  useEffect(() => {
    if (!expanded) return;
    let cancelled = false;
    setIsLoading(true);
    setLoadError(null);
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
        setLoadError(
          "Vergangene Blöcke konnten nicht geladen werden. Bitte später erneut versuchen.",
        );
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [expanded]);

  return (
    <section className="moto-content-surface mt-5 overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <button
        type="button"
        onClick={() => setExpanded((current) => !current)}
        className="group focus-visible:ring-moto-blue/30 flex w-full items-center gap-3 p-4 text-left focus-visible:ring-2 focus-visible:outline-none sm:p-5"
        aria-expanded={expanded}
      >
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100">
          <History className="h-5 w-5 text-gray-600" aria-hidden="true" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="text-moto-blue block text-xs font-semibold tracking-wide uppercase">
            Rückblick
          </span>
          <span className="block text-base font-semibold text-gray-900">
            Beendete und abgelaufene Blöcke
          </span>
        </span>
        <ChevronDown
          className={`h-4 w-4 shrink-0 text-gray-400 transition-transform group-hover:text-gray-600 ${expanded ? "rotate-180" : ""}`}
          aria-hidden="true"
        />
      </button>

      {expanded ? (
        <div className="border-t border-gray-100 bg-gray-50/50 p-4 sm:p-5">
          {isLoading && blocks === null ? (
            <Loading fullPage={false} />
          ) : loadError ? (
            <Alert type="error" message={loadError} />
          ) : blocks !== null && blocks.length === 0 ? (
            <p className="text-sm text-gray-600">
              Heute sind noch keine Blöcke beendet oder abgelaufen.
            </p>
          ) : blocks !== null ? (
            <div className="grid gap-3 xl:grid-cols-2">
              {blocks.map((block) => (
                <PastBlockCard key={block.id} block={block} />
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function PastBlockCard({
  block,
}: Readonly<{ block: PlannedTimetableInstance }>) {
  const [expanded, setExpanded] = useState(false);
  const [roster, setRoster] = useState<TimetableRoster | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const completed = block.status === "completed";

  const loadRoster = useCallback(() => {
    setIsLoading(true);
    setLoadError(null);
    timetableOperationsApi
      .roster(block.id)
      .then(setRoster)
      .catch((err: unknown) => {
        logger.error("past_block_roster_load_failed", {
          instance_id: block.id,
          error: err instanceof Error ? err.message : String(err),
        });
        setLoadError("Kinderliste konnte nicht geladen werden.");
      })
      .finally(() => setIsLoading(false));
  }, [block.id]);

  const toggleRoster = () => {
    const next = !expanded;
    setExpanded(next);
    if (next && roster === null && !isLoading) loadRoster();
  };

  return (
    <article className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm">
      <div className="p-4">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="truncate text-sm font-semibold text-gray-900">
            {block.title}
          </h3>
          {completed ? (
            <StatusBadge label="Beendet" tone="gray" />
          ) : (
            <StatusBadge label="Nicht gestartet" tone="orange" />
          )}
        </div>
        <div className="mt-2 flex flex-wrap gap-3 text-sm text-gray-600">
          <span>
            {block.startTime}-{block.endTime}
          </span>
          <span>{block.roomName ?? `Raum ${block.roomId}`}</span>
          <span>
            {block.assignedStaffIds.length}{" "}
            {block.assignedStaffIds.length === 1 ? "Betreuer" : "Betreuende"}
          </span>
        </div>
      </div>

      <div className="border-t border-gray-100 bg-gray-50/70 px-4 py-3">
        <button
          type="button"
          onClick={toggleRoster}
          className="flex w-full items-center justify-between gap-3 text-left text-sm font-medium text-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          aria-expanded={expanded}
        >
          <span>Kinder</span>
          <ChevronDown
            className={`h-4 w-4 text-gray-400 transition-transform ${expanded ? "rotate-180" : ""}`}
            aria-hidden="true"
          />
        </button>

        {expanded ? (
          <div className="mt-3">
            {isLoading ? (
              <Loading fullPage={false} />
            ) : loadError ? (
              <Alert type="error" message={loadError} />
            ) : roster !== null ? (
              <div className="space-y-2">
                {!completed ? (
                  <p className="text-moto-amber-strong text-xs">
                    Keine Anwesenheit erfasst: Der Block wurde nicht gestartet.
                  </p>
                ) : null}
                {roster.rows.length === 0 ? (
                  <p className="text-sm text-gray-600">
                    Keine Kinder eingeplant.
                  </p>
                ) : (
                  roster.rows.map((row) => (
                    <PastRosterRow
                      key={row.studentId}
                      row={row}
                      showStatus={completed}
                    />
                  ))
                )}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </article>
  );
}

function PastRosterRow({
  row,
  showStatus,
}: Readonly<{ row: TimetableRosterRow; showStatus: boolean }>) {
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
        </div>
        {showStatus ? (
          <StatusDotBadge
            label={pastRosterStatusLabel(row)}
            color={pastRosterDotColor(row)}
          />
        ) : null}
      </div>
    </div>
  );
}
