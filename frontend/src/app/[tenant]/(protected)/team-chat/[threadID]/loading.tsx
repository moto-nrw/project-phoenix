"use client";

import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { TeamThreadSkeleton } from "~/components/messaging/team-chat-skeletons";

/** Route-level loading UI für eine Unterhaltung. */
export default function TeamThreadLoading() {
  return (
    <TenantPage
      title="Unterhaltung"
      statsLoading
      back
      backHref="/team-chat"
      backLabel="Zurück zum Team-Chat"
    >
      <SectionCard>
        <TeamThreadSkeleton />
      </SectionCard>
    </TenantPage>
  );
}
