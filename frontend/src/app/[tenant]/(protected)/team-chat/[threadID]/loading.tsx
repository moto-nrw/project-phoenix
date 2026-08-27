"use client";

import { BackButton } from "~/components/ui/back-button";
import { TeamThreadSkeleton } from "./page-skeleton";

/** Route-level loading UI for one conversation. */
export default function TeamThreadLoading() {
  return (
    <div className="w-full space-y-4">
      <BackButton referrer="/team-chat" />
      <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <TeamThreadSkeleton />
      </div>
    </div>
  );
}
