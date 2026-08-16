// components/ui/loading.tsx
// Generic content-shaped loading skeleton. Prefer a page-specific skeleton
// (see ~/components/ui/page-skeletons) when the loaded layout is known;
// Loading is the fallback for guards, token flows, and small embeds.

"use client";

import { Skeleton } from "~/components/ui/skeleton";

interface LoadingProps {
  readonly message?: string;
  readonly fullPage?: boolean;
}

export function Loading({
  message = "Lädt...",
  fullPage = true,
}: LoadingProps) {
  const containerClasses = fullPage
    ? "fixed inset-0 flex items-center justify-center bg-white/80 backdrop-blur-sm z-50"
    : "flex items-center justify-start pt-24 pb-12";

  return (
    <output
      className={containerClasses}
      aria-label={message}
      aria-live="polite"
    >
      {fullPage ? (
        // Overlay: a single content card, like a modal about to fill in.
        <div className="moto-content-surface w-full max-w-sm rounded-2xl border p-6 shadow-lg">
          <div className="mb-4 flex items-center gap-3">
            <Skeleton className="h-10 w-10 flex-shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton className="h-4 w-2/3 rounded" />
              <Skeleton className="h-3 w-2/5 rounded" />
            </div>
          </div>
          <div className="space-y-2.5">
            <Skeleton className="h-3.5 w-full rounded" />
            <Skeleton className="h-3.5 w-5/6 rounded" />
            <Skeleton className="h-3.5 w-3/4 rounded" />
          </div>
        </div>
      ) : (
        // Inline: heading line over a field-row card — the silhouette of a
        // typical content section, so the swap to real content doesn't jump.
        <div className="w-full max-w-3xl space-y-4">
          <Skeleton className="h-6 w-44 rounded" />
          <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
            <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Skeleton className="h-3 w-24 rounded" />
                <Skeleton className="h-4 w-3/4 rounded" />
              </div>
              <div className="space-y-1.5">
                <Skeleton className="h-3 w-20 rounded" />
                <Skeleton className="h-4 w-1/2 rounded" />
              </div>
              <div className="space-y-1.5">
                <Skeleton className="h-3 w-28 rounded" />
                <Skeleton className="h-4 w-2/3 rounded" />
              </div>
              <div className="space-y-1.5">
                <Skeleton className="h-3 w-24 rounded" />
                <Skeleton className="h-4 w-3/5 rounded" />
              </div>
            </div>
          </div>
        </div>
      )}
      <span className="sr-only">{message}</span>
    </output>
  );
}
