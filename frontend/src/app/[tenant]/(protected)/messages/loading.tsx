"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MessagesSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: renders the real header immediately (Polaris: real
 * chrome first, skeletonize only the data region) with a disabled no-op
 * search field — this component has no page state yet — followed by the
 * thread-list skeleton.
 */
export default function MessagesLoading() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Nachrichten"
        search={{ value: "", onChange: () => {} }}
      />
      <MessagesSkeleton />
    </div>
  );
}
