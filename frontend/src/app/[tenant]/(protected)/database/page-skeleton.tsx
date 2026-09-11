"use client";

import { CollectionGrid } from "~/components/ui/collection-grid";
import { Skeleton } from "~/components/ui/skeleton";
import { TenantPageHeaderSkeleton } from "~/components/ui/page-skeletons";

function DatabaseCardSkeleton() {
  // Mirrors a database section card: icon block, count badge, title,
  // description, footer link.
  return (
    <div className="moto-content-surface w-full overflow-hidden rounded-2xl border shadow-sm">
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
// Ohne Kopfkarte: im Seitengerüst steht sie bereits über dem Inhalt.
export function DatabaseCardGridSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Datenverwaltung wird geladen"
      data-testid="database-index-skeleton"
      className="min-h-[60vh]"
    >
      <CollectionGrid minTileWidth="18rem">
        {Array.from({ length: 8 }, (_, i) => (
          <DatabaseCardSkeleton key={i} />
        ))}
      </CollectionGrid>
    </div>
  );
}

// Route-Ladezustand (loading.tsx): dort gibt es noch kein Seitengerüst,
// deshalb bringt dieses Skelett die Kopfkarte selbst mit.
export function DatabaseIndexSkeleton() {
  return (
    <div className="w-full space-y-6">
      {/* Titel ist statisch, die Statuszeile kommt mit den Zahlen. */}
      <TenantPageHeaderSkeleton />
      <DatabaseCardGridSkeleton />
    </div>
  );
}
