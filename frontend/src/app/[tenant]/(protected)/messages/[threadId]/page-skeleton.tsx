"use client";

import { Skeleton } from "~/components/ui/skeleton";

/**
 * Data-region skeleton: header row (guardian name + subtitle vs. "Zum
 * Kinderprofil" button — both resolved from the loaded thread, so hidden
 * behind a placeholder while loading) and the message-bubble list. Renders
 * inside the real card frame (back nav + moto-content-surface wrapper are
 * static chrome and render immediately), so this covers only the data-bound
 * regions.
 */
export function ThreadSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Unterhaltung wird geladen"
      data-testid="thread-skeleton"
      className="flex min-h-0 flex-1 flex-col"
    >
      <div className="mb-4 flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1 space-y-2">
          <Skeleton className="h-6 w-40 rounded" />
          <Skeleton className="h-4 w-56 rounded" />
        </div>
        <Skeleton className="h-9 w-36 flex-shrink-0 rounded-lg" />
      </div>

      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton
            key={i}
            className={`h-12 rounded-2xl ${
              i % 2 === 0 ? "w-2/3" : "ml-auto w-1/2"
            }`}
          />
        ))}
      </div>
    </div>
  );
}

/**
 * Composer-bar placeholder for the route-level loading fallback, which
 * mounts before the page component (and its draft/send state) exists — the
 * real, interactive composer isn't available yet. `page.tsx` itself keeps the
 * real composer mounted (disabled while loading) once the component is live.
 */
export function ThreadComposerSkeleton() {
  return (
    <div className="mt-4">
      <Skeleton className="h-12 w-full rounded-full" />
    </div>
  );
}
