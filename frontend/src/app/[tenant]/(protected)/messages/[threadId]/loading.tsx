"use client";

import { BackButton } from "~/components/ui/back-button";
import { ThreadSkeleton, ThreadComposerSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: the back navigation and card frame are real,
 * static chrome (Polaris: real chrome first, skeletonize only the data
 * region) — only the header/message-list/composer regions skeletonize.
 */
export default function MessageThreadLoading() {
  return (
    <div className="flex min-h-[20rem] w-full flex-col overflow-hidden">
      <BackButton referrer="/messages" />

      <div className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <ThreadSkeleton />
        <ThreadComposerSkeleton />
      </div>
    </div>
  );
}
