"use client";

import { PlanningContextBarSkeleton } from "~/components/timetable/betreuungsplan-skeleton";
import { timetableSurfacePadded } from "~/components/timetable/timetable-style";
import { Skeleton } from "~/components/ui/skeleton";

/**
 * Ladezustand der Vertretung: der Zweiteiler aus Störungsliste (links) und
 * Tagesraster (rechts) als Platzhalter. Die PlanningContextBar rendert im
 * eingebauten Zustand bereits echt, deshalb ist die Kopfzeile hier optional
 * (`withBar`) und nur für die Suspense-Grenze gedacht, hinter der auch die
 * Kopfzeile noch nicht existiert.
 */
export function VertretungContentSkeleton({
  withBar = false,
}: {
  readonly withBar?: boolean;
}) {
  const content = (
    <div
      role="status"
      aria-live="polite"
      aria-busy="true"
      aria-label="Vertretung wird geladen"
      data-testid="vertretung-content-skeleton"
      className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,400px)_minmax(0,1fr)]"
    >
      <div className={`${timetableSurfacePadded} space-y-3`}>
        <Skeleton className="h-8 w-44 rounded-lg" />
        {[0, 1, 2, 3].map((row) => (
          <Skeleton key={row} className="h-16 w-full rounded-xl" />
        ))}
      </div>
      <div className={`${timetableSurfacePadded} space-y-3`}>
        <Skeleton className="h-8 w-32 rounded-lg" />
        <Skeleton className="h-96 w-full rounded-xl" />
      </div>
    </div>
  );

  if (!withBar) return content;

  return (
    <div className="w-full space-y-4" data-testid="vertretung-page-skeleton">
      <PlanningContextBarSkeleton
        ariaLabel="Vertretungs-Kopfzeile wird geladen"
        testId="vertretung-header-skeleton"
      />
      {content}
    </div>
  );
}
