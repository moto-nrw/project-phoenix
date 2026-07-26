import { Skeleton } from "~/components/ui/skeleton";

/**
 * Placeholder for the PageHeaderWithSearch strip (search field + action
 * button), shared by page-shell skeletons so the loaded header swaps in
 * without layout shift.
 */
export function PageHeaderSkeleton() {
  return (
    <div className="mb-4 flex items-center justify-between gap-3">
      <Skeleton className="h-10 w-full max-w-md rounded-lg" />
      <Skeleton className="h-10 w-32 flex-shrink-0 rounded-lg" />
    </div>
  );
}
