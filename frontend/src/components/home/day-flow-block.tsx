"use client";

import { ChevronRight } from "lucide-react";

import Link from "~/components/ui/navigation-link";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** So viele Blöcke passen auf die Startseite, ohne sie zur Listenseite zu machen. */
const MAX_BLOCKS = 5;

/**
 * Baustein „Ablauf des Tages" (#2180): der Rahmen des Tages, nicht nur die
 * eigenen Einsätze. Zeigt, was gerade läuft und was als Nächstes ansteht;
 * gearbeitet wird weiterhin im Tagesplan, dorthin führt der Weiterlink.
 *
 * Bewusst ohne Kinderlisten: die Startseite ist eine Übersicht, und die
 * Namen gehören in den Tagesplan (GDPR).
 */
export function DayFlowBlock() {
  const tenantPath = useTenantAwarePath();
  const { data, error, isLoading } = useSWRAuth<PlannedTimetableInstance[]>(
    "home-day-flow",
    () =>
      timetableOperationsApi.plannedNow({
        limit: MAX_BLOCKS,
        includeRoster: false,
      }),
    { refreshInterval: 5 * 60 * 1000 },
  );

  const blocks = data ?? [];

  return (
    <SectionCard
      title="Ablauf des Tages"
      leading={<MotoConceptIcon concept="carePlan" size={20} />}
      className="h-full"
      actions={
        <Link
          href={tenantPath("/tagesplan")}
          aria-label="Ablauf des Tages: zum Tagesplan"
          className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          Zum Tagesplan
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      }
    >
      {(() => {
        if (error) {
          return (
            <Alert
              type="error"
              message="Der Ablauf des Tages konnte nicht geladen werden. Bitte die Seite neu laden."
            />
          );
        }
        if (isLoading && data === undefined) {
          return (
            <div className="space-y-2" aria-hidden="true">
              {[1, 2, 3].map((i) => (
                <div
                  key={i}
                  className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3"
                >
                  <div className="min-w-0 flex-1 space-y-1.5">
                    <div className="h-4 w-2/5 animate-pulse rounded bg-gray-200"></div>
                    <div className="h-3 w-1/4 animate-pulse rounded bg-gray-200"></div>
                  </div>
                  <div className="h-6 w-16 animate-pulse rounded-full bg-gray-200"></div>
                </div>
              ))}
            </div>
          );
        }
        if (blocks.length === 0) {
          return (
            <EmptyState
              className="py-8"
              title="Gerade steht nichts an"
              description="Hier erscheinen die Blöcke des Betreuungsplans, sobald der nächste beginnt."
            />
          );
        }
        return (
          <ul className="space-y-2">
            {blocks.map((block) => (
              <li
                key={block.id}
                className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 p-3"
              >
                <span className="w-24 flex-shrink-0 text-sm font-medium tabular-nums text-gray-900">
                  {block.startTime}–{block.endTime}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-gray-900">
                    {block.title}
                  </p>
                  <p className="truncate text-xs text-gray-500">
                    {[
                      block.roomName,
                      `${block.presentStudentsCount}/${block.expectedStudentsCount} Kinder`,
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                </div>
                <BlockState block={block} />
              </li>
            ))}
          </ul>
        );
      })()}
    </SectionCard>
  );
}

function BlockState({
  block,
}: {
  readonly block: PlannedTimetableInstance;
}) {
  if (block.status === "cancelled") {
    return <StatusBadge tone="red" label="Entfällt" />;
  }
  if (block.status === "active") {
    return <StatusBadge tone="green" label="Läuft" />;
  }
  if (block.status === "completed") {
    return <StatusBadge tone="gray" label="Beendet" />;
  }
  if (block.isOverdue) {
    return <StatusBadge tone="orange" label="Überfällig" />;
  }
  return <StatusBadge tone="blue" label="Geplant" />;
}
