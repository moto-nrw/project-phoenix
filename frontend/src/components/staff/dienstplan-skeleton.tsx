"use client";

import { CapacityStrip } from "~/components/ui/capacity-strip";
import {
  RESOURCE_GRID_METRICS,
  resourceGridMinWidth,
} from "~/components/ui/resource-grid";
import { Skeleton } from "~/components/ui/skeleton";
import { PlanningContextBarSkeleton } from "~/components/timetable/betreuungsplan-skeleton";

// Das Skelett rastert wie das echte Wochenraster (Issue #2026). Die Maße —
// Spaltenbreiten und Zellen-Mindesthöhe — kommen aus RESOURCE_GRID_METRICS und
// wandern mit; die Tabellenstruktur (Ränder, Paddings, sticky Personenspalte)
// ist bewusst nachgebaut, weil das ResourceGrid Textspalten-Köpfe erwartet und
// hier nur Platzhalter stehen. Ändert sich dort die Struktur, muss sie hier
// nachgezogen werden.
const SKELETON_DAY_COLUMNS = 5;
const DAY_COLUMN_INDEXES = Array.from(
  { length: SKELETON_DAY_COLUMNS },
  (_, index) => index,
);

// The grid surface card: staff column + 5 weekday columns, 6 rows, plus the
// CapacityStrip footer row (label + 5 cells). Shared by both the full-page
// skeleton and the standalone grid skeleton — no status role here so it can be
// composed under either wrapper without nesting two live regions.
function GridSkeletonBody() {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <div className="max-w-full overflow-clip rounded-2xl border border-gray-200">
        <div className="max-w-full overflow-x-auto">
          <table
            className="w-full table-fixed border-collapse text-sm"
            style={{ minWidth: resourceGridMinWidth(SKELETON_DAY_COLUMNS) }}
          >
            <colgroup>
              <col style={{ width: RESOURCE_GRID_METRICS.rowHeaderWidth }} />
              {DAY_COLUMN_INDEXES.map((col) => (
                <col key={col} />
              ))}
            </colgroup>
            <thead>
              <tr className="bg-gray-50">
                <th className="sticky left-0 z-10 bg-gray-50 px-2 py-1.5 text-left">
                  <Skeleton className="h-4 w-16 rounded" />
                </th>
                {DAY_COLUMN_INDEXES.map((col) => (
                  <th key={col} className="px-2 py-1.5 text-left">
                    <Skeleton className="h-4 w-14 rounded" />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {Array.from({ length: 6 }, (_, row) => (
                <tr key={row} className="border-t border-gray-100">
                  <th className="sticky left-0 z-10 bg-white px-2 py-1.5 text-left align-top font-normal">
                    <Skeleton className="h-4 w-32 rounded" />
                  </th>
                  {DAY_COLUMN_INDEXES.map((col) => (
                    <td
                      key={col}
                      className="border-l border-gray-100 px-1 py-1 align-top"
                    >
                      <Skeleton
                        className={`${RESOURCE_GRID_METRICS.cellMinHeightClass.days} w-full rounded-md`}
                      />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
            <tfoot>
              <CapacityStrip
                rowLabel={<Skeleton className="h-4 w-28 rounded" />}
                cells={DAY_COLUMN_INDEXES.map((col) => ({
                  key: String(col),
                  content: <Skeleton className="mx-auto h-4 w-8 rounded" />,
                }))}
              />
            </tfoot>
          </table>
        </div>
      </div>
    </div>
  );
}

// Grid-only skeleton for the content area while the schedule data loads —
// including the initial session/settings-schema load, so the real
// PlanningContextBar (title, week navigation) renders immediately and only
// this grid area falls back to a skeleton (docs/05 Abschnitt 5).
export function DienstplanGridSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Dienstplan wird geladen"
      data-testid="dienstplan-grid-skeleton"
    >
      <GridSkeletonBody />
    </div>
  );
}

// Ganzseitiges Skelett für die Suspense-Grenze der Seite: dort existiert auch
// die PlanningContextBar noch nicht, also steht sie hier als Platzhalter über
// dem Raster.
export function DienstplanPageSkeleton() {
  return (
    <div className="w-full space-y-6" data-testid="dienstplan-page-skeleton">
      <PlanningContextBarSkeleton
        ariaLabel="Dienstplan-Kopfzeile wird geladen"
        testId="dienstplan-header-skeleton"
      />
      <DienstplanGridSkeleton />
    </div>
  );
}
