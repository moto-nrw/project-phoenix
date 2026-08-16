import type React from "react";

import { Skeleton } from "~/components/ui/skeleton";

/**
 * Mobile-optimized info card component
 * Displays a card with an icon, title, and content.
 *
 * While `loading` is true the static chrome (icon, eyebrow, title) renders
 * real and only the content area shows InfoItem-shaped bars — the skeleton
 * is this card's own markup, so it cannot drift from the loaded state.
 */
export function InfoCard({
  title,
  children,
  icon,
  eyebrow,
  headingLevel = 2,
  loading = false,
  loadingRows = 3,
}: Readonly<{
  title: string;
  children: React.ReactNode;
  icon: React.ReactNode;
  eyebrow?: string;
  headingLevel?: 1 | 2;
  loading?: boolean;
  loadingRows?: number;
}>) {
  const Heading = headingLevel === 1 ? "h1" : "h2";

  return (
    <div className="moto-content-surface flex h-full flex-col rounded-2xl border p-4 shadow-sm backdrop-blur sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
          {icon}
        </div>
        <div>
          {eyebrow && (
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {eyebrow}
            </p>
          )}
          <Heading className="text-base font-semibold text-gray-900 sm:text-lg">
            {title}
          </Heading>
        </div>
      </div>
      {/* flex-1 lets a child opt into bottom-pinning via mt-auto (e.g. an action
          row that should align across cards of different content length). */}
      <div className="flex flex-1 flex-col space-y-3">
        {loading ? (
          <>
            <output aria-live="polite" className="sr-only">
              Wird geladen…
            </output>
            {Array.from({ length: loadingRows }, (_, i) => (
              <InfoItemSkeleton key={i} />
            ))}
          </>
        ) : (
          children
        )}
      </div>
    </div>
  );
}

/**
 * Loading twin of InfoItem, colocated so the two share one markup shape:
 * same row layout, same label/value stack, bars instead of text.
 */
function InfoItemSkeleton({ icon = false }: Readonly<{ icon?: boolean }>) {
  return (
    <div className="flex items-start gap-3" aria-hidden="true">
      {icon && <Skeleton className="mt-0.5 h-4 w-4 flex-shrink-0 rounded" />}
      <div className="min-w-0 flex-1">
        <p className="mb-1 text-xs text-gray-500">
          <Skeleton className="h-3 w-20 rounded" />
        </p>
        <div className="text-sm font-medium text-gray-900">
          <Skeleton className="h-4 w-2/3 rounded" />
        </div>
      </div>
    </div>
  );
}

/**
 * Simplified info item component
 * Displays a label-value pair within an InfoCard
 */
export function InfoItem({
  label,
  value,
  icon,
}: Readonly<{
  /** Plain text, or text plus an inline marker (e.g. ParentVisibleBadge). */
  label: string | React.ReactNode;
  value: string | React.ReactNode;
  icon?: React.ReactNode;
}>) {
  return (
    <div className="flex items-start gap-3">
      {icon && <div className="mt-0.5 flex-shrink-0 text-gray-400">{icon}</div>}
      <div className="min-w-0 flex-1">
        <p className="mb-1 text-xs text-gray-500">{label}</p>
        <div className="text-sm font-medium text-gray-900">{value}</div>
      </div>
    </div>
  );
}
