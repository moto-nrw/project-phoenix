"use client";

/**
 * ConflictWarningsBanner — top-of-plan hint listing the person double-bookings
 * (#2139) detected for the visible window: a child in two overlapping blocks,
 * or staff planned in overlapping blocks in different rooms. Room sharing and
 * pure parallelism are sanctioned and never appear here.
 *
 * Each conflict can be hidden per user ("Konflikt ausblenden") — the
 * acknowledgement is keyed by the conflict's stable fingerprint, persists on
 * the backend, and only covers this concrete conflict: if person, time, block,
 * or room change, the conflict returns. Hidden conflicts stay reachable via
 * "Ausgeblendete anzeigen" and can be restored.
 *
 * Style follows the shadcn idiom used elsewhere on this page: hairline
 * border, no shadow, subtle amber tint reserved for the icon dot. The banner
 * collapses to nothing when there is neither an open nor a hidden conflict.
 */

import { useState } from "react";
import {
  Baby,
  ChevronDown,
  Eye,
  EyeOff,
  TriangleAlert,
  UserRound,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import { formatDate } from "~/lib/date-helpers";
import { timetableSurface } from "./timetable-style";

/** One aggregated conflict (both involved blocks share the fingerprint). */
export interface ConflictBannerEntry {
  fingerprint: string;
  kind: "staff" | "student";
  /** Display name of the double-booked person; resolved by the caller. */
  personName: string;
  dateISO: string;
  overlapStart?: string;
  overlapEnd?: string;
  /** The involved blocks, deduplicated; clicking one opens its details. */
  instances: Array<{ id: string; title: string }>;
}

interface ConflictWarningsBannerProps {
  openConflicts: ConflictBannerEntry[];
  hiddenConflicts: ConflictBannerEntry[];
  /** Copy for the checked window, e.g. "diese Woche" / "in diesem Monat". */
  periodLabel: string;
  onHide: (entry: ConflictBannerEntry) => void;
  /**
   * Bulk convenience: acknowledges every currently OPEN conflict, each by its
   * own fingerprint. Deliberately not a category switch — new or changed
   * conflicts still reappear (#2139).
   */
  onHideAll: () => void;
  onUnhide: (entry: ConflictBannerEntry) => void;
  onJump: (entry: ConflictBannerEntry, instanceId: string) => void;
}

function kindLabel(kind: ConflictBannerEntry["kind"]): string {
  return kind === "student" ? "Kind" : "Personal";
}

function entrySummary(entry: ConflictBannerEntry): string {
  const window =
    entry.overlapStart && entry.overlapEnd
      ? `, ${entry.overlapStart}–${entry.overlapEnd} Uhr`
      : "";
  return `${formatDate(entry.dateISO)}${window}`;
}

function ConflictRow({
  entry,
  hidden,
  onHide,
  onUnhide,
  onJump,
}: {
  entry: ConflictBannerEntry;
  hidden: boolean;
  onHide: (entry: ConflictBannerEntry) => void;
  onUnhide: (entry: ConflictBannerEntry) => void;
  onJump: (entry: ConflictBannerEntry, instanceId: string) => void;
}) {
  const KindIcon = entry.kind === "student" ? Baby : UserRound;
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-1 py-2">
      <span className="inline-flex min-w-0 items-center gap-1.5 text-xs text-gray-700">
        <KindIcon className="h-3.5 w-3.5 shrink-0 text-gray-400" aria-hidden />
        <span className="font-semibold text-gray-900">{entry.personName}</span>
        <span className="text-gray-500">
          ({kindLabel(entry.kind)}) doppelt eingeplant · {entrySummary(entry)}
        </span>
      </span>
      <span className="inline-flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
        {entry.instances.map((instance) => (
          <button
            key={instance.id}
            type="button"
            onClick={() => onJump(entry, instance.id)}
            className="max-w-48 truncate rounded-md border border-gray-200 bg-white px-2 py-0.5 text-xs text-gray-700 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            aria-label={`Termin „${instance.title}“ öffnen`}
          >
            {instance.title}
          </button>
        ))}
      </span>
      {hidden ? (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={() => onUnhide(entry)}
          aria-label={`Konflikt für ${entry.personName} wieder anzeigen`}
        >
          <Eye className="h-3.5 w-3.5" aria-hidden />
          Wieder anzeigen
        </Button>
      ) : (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={() => onHide(entry)}
          aria-label={`Konflikt für ${entry.personName} ausblenden`}
        >
          <EyeOff className="h-3.5 w-3.5" aria-hidden />
          Ausblenden
        </Button>
      )}
    </li>
  );
}

export function ConflictWarningsBanner({
  openConflicts,
  hiddenConflicts,
  periodLabel,
  onHide,
  onHideAll,
  onUnhide,
  onJump,
}: ConflictWarningsBannerProps) {
  const [expanded, setExpanded] = useState(false);
  const [showHidden, setShowHidden] = useState(false);

  if (openConflicts.length === 0 && hiddenConflicts.length === 0) {
    return null;
  }

  const openCount = openConflicts.length;
  const hasOpen = openCount > 0;

  return (
    <div className={`${timetableSurface} px-3 py-2`}>
      <div role="status" className="flex items-center gap-2.5">
        <span
          className={`inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full ${hasOpen ? "bg-[#EAB308]/10" : "bg-gray-100"}`}
        >
          <TriangleAlert
            className={`h-3.5 w-3.5 ${hasOpen ? "text-[#EAB308]" : "text-gray-400"}`}
            aria-hidden
          />
        </span>
        <p className="min-w-0 flex-1 text-xs text-gray-600">
          {hasOpen ? (
            <>
              <span className="font-semibold text-gray-900">
                {openCount} {openCount === 1 ? "Konflikt" : "Konflikte"}
              </span>{" "}
              {periodLabel} · doppelt eingeplante Kinder oder Personal.
            </>
          ) : (
            <>Keine offenen Konflikte {periodLabel}.</>
          )}
          {hiddenConflicts.length > 0 && (
            <span className="text-gray-400">
              {" "}
              {hiddenConflicts.length} ausgeblendet.
            </span>
          )}
        </p>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={() => setExpanded((v) => !v)}
          aria-expanded={expanded}
          aria-label={
            expanded ? "Konfliktdetails verbergen" : "Konfliktdetails anzeigen"
          }
        >
          Details
          <ChevronDown
            className={`h-3.5 w-3.5 transition-transform ${expanded ? "rotate-180" : ""}`}
            aria-hidden
          />
        </Button>
      </div>

      {expanded && (
        <div className="mt-1 border-t border-gray-100 pl-8">
          {hasOpen ? (
            <>
              <ul className="divide-y divide-gray-100">
                {openConflicts.map((entry) => (
                  <ConflictRow
                    key={entry.fingerprint}
                    entry={entry}
                    hidden={false}
                    onHide={onHide}
                    onUnhide={onUnhide}
                    onJump={onJump}
                  />
                ))}
              </ul>
              {openCount > 1 && (
                <div className="flex justify-end border-t border-gray-100 py-1.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={onHideAll}
                    aria-label="Alle Konflikte ausblenden"
                  >
                    <EyeOff className="h-3.5 w-3.5" aria-hidden />
                    Alle ausblenden
                  </Button>
                </div>
              )}
            </>
          ) : (
            <p className="py-2 text-xs text-gray-500">
              Alle Konflikte sind geprüft und ausgeblendet.
            </p>
          )}

          {hiddenConflicts.length > 0 && (
            <div className="border-t border-gray-100 py-2">
              <button
                type="button"
                onClick={() => setShowHidden((v) => !v)}
                aria-expanded={showHidden}
                className="text-xs text-gray-500 underline-offset-2 hover:text-gray-700 hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                {showHidden
                  ? "Ausgeblendete Konflikte verbergen"
                  : `Ausgeblendete Konflikte anzeigen (${hiddenConflicts.length})`}
              </button>
              {showHidden && (
                <ul className="divide-y divide-gray-100">
                  {hiddenConflicts.map((entry) => (
                    <ConflictRow
                      key={entry.fingerprint}
                      entry={entry}
                      hidden
                      onHide={onHide}
                      onUnhide={onUnhide}
                      onJump={onJump}
                    />
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
