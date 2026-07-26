"use client";

import { Skeleton } from "~/components/ui/skeleton";

// Content-shaped placeholder mirroring the thread-list rows, so there is no
// layout shift once the inbox loads.
export function MessagesSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Nachrichten werden geladen"
      data-testid="messages-skeleton"
      className="-mt-1.5 w-full"
    >
      <ul className="space-y-3">
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
    </div>
  );
}
