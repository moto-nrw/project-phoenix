"use client";

import { BackButton } from "~/components/ui/back-button";
import { TeamThreadSkeleton } from "./page-skeleton";

/** Route-level loading UI for one conversation. */
export default function TeamThreadLoading() {
  return (
    <div className="-mt-1.5 w-full">
      <BackButton referrer="/team-chat" />
      <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <TeamThreadSkeleton />
      </div>
    </div>
  );
}
