"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { TeamChatSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: the real header renders immediately (real chrome
 * first, skeletonize only the data region) with a disabled no-op search field —
 * this component has no page state yet — followed by the conversation skeleton.
 */
export default function TeamChatLoading() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Team-Chat"
        search={{
          value: "",
          onChange: () => {},
          inputProps: { disabled: true },
        }}
      />
      <TeamChatSkeleton />
    </div>
  );
}
