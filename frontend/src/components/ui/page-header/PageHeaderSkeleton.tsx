import { Skeleton } from "~/components/ui/skeleton";

/**
 * Placeholder for PageHeaderWithSearch, shared by page-shell skeletons so
 * there is one header geometry to maintain.
 */
export function PageHeaderSkeleton({
  search = true,
  chips = 0,
  actions = 0,
}: Readonly<{ search?: boolean; chips?: number; actions?: number }> = {}) {
  return (
    <div className="mb-4 flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3 md:hidden">
        <Skeleton className="h-6 w-32 rounded" />
        {actions > 0 ? (
          <div className="flex flex-shrink-0 items-center gap-2">
            {Array.from({ length: actions }, (_, index) => (
              <Skeleton key={index} className="h-10 w-28 rounded-lg" />
            ))}
          </div>
        ) : null}
      </div>
      <div className="flex items-center justify-between gap-3">
        {search ? (
          <Skeleton className="h-10 w-full max-w-sm flex-1 rounded-lg sm:max-w-md" />
        ) : null}
        {actions > 0 ? (
          <div className="hidden flex-shrink-0 items-center gap-2 lg:flex">
            {Array.from({ length: actions }, (_, index) => (
              <Skeleton key={index} className="h-10 w-28 rounded-lg" />
            ))}
          </div>
        ) : null}
      </div>
      {chips > 0 ? (
        <div className="flex flex-wrap gap-2">
          {Array.from({ length: chips }, (_, index) => (
            <Skeleton key={index} className="h-8 w-24 rounded-full" />
          ))}
        </div>
      ) : null}
    </div>
  );
}
