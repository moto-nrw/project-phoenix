"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { DeclarationReport } from "~/components/announcements/declaration-report";
import { PrintButton, PrintDocument } from "~/components/ui/print-document";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  fetchAnnouncement,
  fetchDeclarationStatus,
  isDeclaration,
} from "~/lib/parent-announcements-api";
import type {
  Announcement,
  DeclarationStatus,
} from "~/lib/parent-announcements-api";
import { useSWRAuth } from "~/lib/swr";

/**
 * Der Nachweisbericht einer Erklärung (#3430) als druckbare Seite: aus dem
 * Stand der Erklärung gebaut, gedruckt oder als PDF gespeichert über den
 * Druckdialog des Browsers. Die Objektseite verlinkt hierher.
 */
export default function DeclarationReportPage() {
  const params = useParams();
  const announcementId = params.id as string;
  const detailPath = `/parent-announcements/${encodeURIComponent(announcementId)}`;
  // Fixed when the page opens, so screen and printout name the same moment.
  const [generatedAt] = useState(() => new Date().toISOString());

  const {
    data: announcement,
    isLoading: announcementLoading,
    error: announcementError,
  } = useSWRAuth<Announcement | undefined>(
    `parent-announcement-${announcementId}`,
    () => fetchAnnouncement(announcementId),
    { revalidateOnFocus: false },
  );
  const declaration = announcement ? isDeclaration(announcement) : false;
  const {
    data: status,
    isLoading: statusLoading,
    error: statusError,
  } = useSWRAuth<DeclarationStatus>(
    declaration ? `parent-announcement-${announcementId}-declaration` : null,
    () => fetchDeclarationStatus(announcementId),
    { revalidateOnFocus: false },
  );

  useSetBreadcrumb({
    announcementTitle: announcement?.title,
    referrerPage: detailPath,
  });

  const loading = announcementLoading || (declaration && statusLoading);
  const error =
    announcementError || statusError
      ? "Der Nachweisbericht konnte nicht geladen werden."
      : !loading && (!announcement || !declaration)
        ? "Zu dieser Mitteilung gibt es keinen Nachweisbericht."
        : null;

  return (
    <TenantPage
      title="Nachweisbericht"
      stats={announcement?.title}
      statsLoading={announcementLoading}
      back
      backHref={detailPath}
      backLabel="Zurück zur Erklärung"
      actions={
        announcement && status ? (
          <PrintButton label="Drucken / als PDF speichern" />
        ) : null
      }
      loading={Boolean(loading)}
      loadingLabel="Nachweisbericht wird geladen…"
      error={error}
    >
      {announcement && status && (
        <SectionCard>
          <PrintDocument>
            <DeclarationReport
              announcement={announcement}
              status={status}
              generatedAt={generatedAt}
            />
          </PrintDocument>
        </SectionCard>
      )}
    </TenantPage>
  );
}
