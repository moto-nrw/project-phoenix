"use client";

import { Skeleton } from "~/components/ui/skeleton";
import { PageIntro } from "~/components/ui/page-intro";

function DatabaseCardSkeleton() {
  // Mirrors a database section card: icon block, count badge, title,
  // description, footer link.
  return (
    <div className="moto-content-surface w-full overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
      <div className="p-4 sm:p-6">
        <div className="mb-4 flex items-start justify-between">
          <Skeleton className="h-12 w-12 rounded-2xl" />
          <Skeleton className="h-6 w-16 rounded-full" />
        </div>
        <Skeleton className="mb-2 h-5 w-2/3 rounded" />
        <Skeleton className="mb-4 h-4 w-full rounded" />
        <Skeleton className="h-4 w-24 rounded" />
      </div>
    </div>
  );
}

// Content-shaped placeholder for the /database index page — mirrors the real
// card grid (DatabaseContent) so there is no layout shift once data arrives.
export function DatabaseIndexSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Datenverwaltung wird geladen"
      data-testid="database-index-skeleton"
      className="w-full"
    >
      {/* Kicker und Titel sind statisch, deshalb rendert die Kopfkarte sofort. */}
      <PageIntro title="Datenverwaltung" className="mb-6" />
      <div className="min-h-[60vh]">
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 8 }, (_, i) => (
            <DatabaseCardSkeleton key={i} />
          ))}
        </div>
      </div>
    </div>
  );
}
