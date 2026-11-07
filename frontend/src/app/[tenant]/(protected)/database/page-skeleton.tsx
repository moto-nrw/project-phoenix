"use client";

import { CollectionGrid } from "~/components/ui/collection-grid";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { TenantPageHeaderSkeleton } from "~/components/ui/page-skeletons";

function DatabaseCardSkeleton() {
  // Mirrors a database section card: icon block, count badge, title,
  // description, footer link.
  return (
    <SectionCard padding="none" className="w-full">
      <div className="p-4 sm:p-6">
        <div className="mb-4 flex items-start justify-between">
          <Skeleton shape="card" className="h-12 w-12" />
          <Skeleton shape="round" className="h-6 w-16" />
        </div>
        <Skeleton className="mb-2 h-5 w-2/3" />
        <Skeleton className="mb-4 h-4 w-full" />
        <Skeleton className="h-4 w-24" />
      </div>
    </SectionCard>
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
      {/* Ohne Statuszeile: die Zahlen stehen auf den Kacheln (#4385). */}
      <TenantPageHeaderSkeleton stats={false} info />
      <DatabaseCardGridSkeleton />
    </div>
  );
}
