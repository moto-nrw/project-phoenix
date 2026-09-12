"use client";

import { AnnouncementDetailSkeleton } from "~/components/announcements/announcement-detail";
import { Skeleton } from "~/components/ui/skeleton";
import { TenantPage } from "~/components/ui/tenant-page";

/**
 * Das geladene Seitengerüst der Mitteilungsseite: Titel und Status hängen
 * an der noch nicht geladenen Mitteilung.
 */
export function AnnouncementDetailLoadingPage({
  referrer = "/parent-announcements",
  backLabel = "Zurück zu den Mitteilungen",
}: Readonly<{ referrer?: string; backLabel?: string }>) {
  return (
    <TenantPage
      title="Mitteilung"
      back
      backHref={referrer}
      backLabel={backLabel}
      leading={<Skeleton className="h-10 w-10 shrink-0 rounded-xl" />}
      statsLoading
    >
      <AnnouncementDetailSkeleton />
    </TenantPage>
  );
}
