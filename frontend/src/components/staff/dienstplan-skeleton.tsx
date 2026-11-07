"use client";

import { CapacityStrip } from "~/components/ui/planning/capacity-strip";
import {
  ResourceGrid,
  type ResourceGridColumn,
} from "~/components/ui/planning/resource-grid";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { PlanningContextBarSkeleton } from "~/components/timetable/betreuungsplan-skeleton";

// Das Skelett IST das echte Wochenraster (Issue #2026), nur mit Platzhaltern
// statt Personen und Diensten; Spaltenbreiten und Zellhöhen können deshalb
// nicht auseinanderlaufen.
const SKELETON_COLUMNS: ResourceGridColumn[] = Array.from(
  { length: 5 },
  (_, index) => ({
    key: String(index),
    label: "",
    headerNote: <Skeleton className="h-4 w-14" />,
  }),
);
const SKELETON_ROWS = Array.from({ length: 6 }, (_, index) => index);

// The grid surface card: staff column + 5 weekday columns, 6 rows, plus the
// CapacityStrip footer row (label + 5 cells). Shared by both the full-page
// skeleton and the standalone grid skeleton — no status role here so it can be
// composed under either wrapper without nesting two live regions.
function GridSkeletonBody() {
  return (
    <SectionCard padding="none">
      <div className="p-4 sm:p-6">
        <ResourceGrid
          columns={SKELETON_COLUMNS}
          rows={SKELETON_ROWS}
          getRowKey={String}
          cornerHeader={<Skeleton className="h-4 w-16" />}
          renderRowHeader={() => <Skeleton className="h-4 w-32" />}
          renderCell={() => <Skeleton className="w-full flex-1" />}
          footer={
            <CapacityStrip
              rowLabel={<Skeleton className="h-4 w-28" />}
              cells={SKELETON_COLUMNS.map((column) => ({
                key: column.key,
                content: <Skeleton className="mx-auto h-4 w-8" />,
              }))}
            />
          }
        />
      </div>
    </SectionCard>
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
