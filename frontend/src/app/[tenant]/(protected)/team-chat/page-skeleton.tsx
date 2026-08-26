"use client";

import { Skeleton } from "~/components/ui/skeleton";

/**
 * Data-region skeleton: just the conversation rows. The real header and filter
 * bar render immediately regardless of loading state — only this data-bound
 * region skeletonizes.
 */
export function TeamChatSkeleton() {
  return (
    <ul
      role="status"
      aria-busy="true"
      aria-label="Unterhaltungen werden geladen"
      data-testid="team-chat-skeleton"
      className="space-y-3"
    >
      {[0, 1, 2, 3, 4].map((item) => (
        <li key={item}>
          <div className="moto-content-surface rounded-2xl border border-gray-200 bg-white p-4 sm:p-5">
            <div className="flex items-start justify-between gap-3">
              <div className="flex min-w-0 flex-1 items-start gap-3">
                <Skeleton className="h-10 w-10 flex-shrink-0 rounded-full" />
                <div className="min-w-0 flex-1 space-y-2">
                  <Skeleton className="h-4 w-2/5 rounded-full" />
                  <Skeleton className="h-3.5 w-3/5 rounded-full" />
                </div>
              </div>
              <Skeleton className="h-3 w-10 flex-shrink-0 rounded-full" />
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}
