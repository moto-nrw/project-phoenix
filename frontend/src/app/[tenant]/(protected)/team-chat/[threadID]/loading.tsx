"use client";

import { BackButton } from "~/components/ui/back-button";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { TeamThreadSkeleton } from "./page-skeleton";

/** Route-level loading UI für eine Unterhaltung. */
export default function TeamThreadLoading() {
  return (
    <TenantPage title="Unterhaltung" statsLoading>
      <BackButton referrer="/team-chat" />

      <SectionCard>
        <TeamThreadSkeleton />
      </SectionCard>
    </TenantPage>
  );
}
