"use client";

import { BackButton } from "~/components/ui/back-button";
import { TenantPage } from "~/components/ui/tenant-page";
import { TeamThreadSkeleton } from "./page-skeleton";

/** Route-level loading UI für eine Unterhaltung. */
export default function TeamThreadLoading() {
  return (
    <TenantPage title="Unterhaltung" statsLoading>
      <BackButton referrer="/team-chat" />

      <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        <TeamThreadSkeleton />
      </div>
    </TenantPage>
  );
}
