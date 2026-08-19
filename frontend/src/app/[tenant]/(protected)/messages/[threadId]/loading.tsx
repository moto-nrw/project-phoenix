"use client";

import { ArrowLeft } from "lucide-react";
import { BackButton } from "~/components/ui/back-button";
import { ThreadSkeleton, ThreadComposerSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: the back navigation and card frame are real,
 * static chrome (Polaris: real chrome first, skeletonize only the data
 * region) — only the header/message-list/composer regions skeletonize.
 * Mirrors MessagesBackNav in ./page.tsx (kept inline here rather than
 * imported, matching the staff/rooms loading.tsx convention of
 * reconstructing static chrome directly instead of importing from page.tsx).
 */
export default function MessageThreadLoading() {
  return (
    <div className="-mt-1.5 flex min-h-[20rem] w-full flex-col overflow-hidden">
      <BackButton referrer="/messages" />
      <button
        type="button"
        disabled
        className="mb-4 hidden items-center gap-1 text-sm text-gray-500 md:flex"
      >
        <ArrowLeft className="h-4 w-4" /> Zurück zu den Nachrichten
      </button>

      <div className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <ThreadSkeleton />
        <ThreadComposerSkeleton />
      </div>
    </div>
  );
}
