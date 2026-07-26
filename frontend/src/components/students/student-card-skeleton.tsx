// components/students/student-card-skeleton.tsx
// Skeleton mirror of StudentCard, shared by the OGS groups and student
// search pages so their loading state keeps the exact card-grid footprint.

import { Skeleton } from "~/components/ui/skeleton";
import { PageHeaderSkeleton } from "~/components/ui/page-header/PageHeaderSkeleton";

/**
 * Mirrors StudentCard's surface (rounded-2xl border, p-6 pb-5) and header
 * layout (avatar + name lines, badge pill right, meta rows, bottom hint)
 * so swapping in real cards causes no layout shift.
 */
function StudentCardSkeleton() {
  return (
    <div className="moto-content-surface w-full overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
      <div className="p-6 pb-5">
        <div className="mb-3 flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-start gap-3">
              <Skeleton className="h-10 w-10 flex-shrink-0 rounded-full" />
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-5 w-2/3 rounded" />
                <Skeleton className="h-4 w-1/2 rounded" />
              </div>
            </div>
            <Skeleton className="mt-3 h-3.5 w-3/5 rounded" />
          </div>
          <Skeleton className="h-6 w-20 flex-shrink-0 rounded-full" />
        </div>
        <Skeleton className="h-3 w-28 rounded" />
      </div>
    </div>
  );
}

// Same responsive grid classes as the real student-card grids in
// ogs-groups and students/search.
function StudentCardGrid({ count }: Readonly<{ count: number }>) {
  return (
    <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
      {Array.from({ length: count }, (_, i) => (
        <StudentCardSkeleton key={i} />
      ))}
    </div>
  );
}

/**
 * Standalone grid of StudentCardSkeletons with its own status role, for
 * data-loading branches that replace only the card grid.
 */
export function StudentCardGridSkeleton({
  count = 6,
  label = "Kinder werden geladen",
}: Readonly<{ count?: number; label?: string }>) {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label={label}
      data-testid="student-card-grid-skeleton"
    >
      <StudentCardGrid count={count} />
    </div>
  );
}

/**
 * Page-shell skeleton for the gate/Suspense states of the student-card
 * pages (ogs-groups, students/search): a PageHeaderWithSearch placeholder
 * followed by the card grid, so swapping in the real content causes no
 * layout shift. Owns the single status role for the whole shell.
 */
export function StudentCardPageSkeleton({
  label,
  testId,
}: Readonly<{ label: string; testId: string }>) {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label={label}
      data-testid={testId}
      className="-mt-1.5 w-full"
    >
      <PageHeaderSkeleton />
      <StudentCardGrid count={6} />
    </div>
  );
}
