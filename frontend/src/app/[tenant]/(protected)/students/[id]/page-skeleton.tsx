"use client";

import { DetailSectionSkeleton } from "~/components/ui/page-skeletons";
import { Skeleton } from "~/components/ui/skeleton";
import { TenantPage } from "~/components/ui/tenant-page";

const LOADING_TABS = [
  "Stammdaten",
  "Nachrichten",
  "Erziehungsberechtigte",
  "Betreuungsplan",
  "Betreuungszeiten",
  "Anmeldungen",
  "Dokumente",
  "Änderungsprotokoll",
  "Historie",
];

/**
 * Das geladene Seitengerüst der Kindakte. Titel und verfügbare Reiter hängen
 * von den noch nicht geladenen Kind- und Berechtigungsdaten ab; die
 * Platzhalter-Reiter zeigen deshalb die vollständige Struktur deaktiviert.
 */
export function StudentDetailLoadingPage({
  referrer,
  backLabel = "Zurück zur Kinderübersicht",
  preview,
}: Readonly<{
  referrer?: string;
  backLabel?: string;
  preview?: { name: string; context?: string };
}>) {
  return (
    <TenantPage
      title={preview?.name ?? "Kindakte"}
      back
      backHref={referrer}
      backLabel={backLabel}
      leading={<Skeleton shape="inner" className="h-12 w-12 shrink-0" />}
      stats={preview?.context}
      statsLoading={!preview?.context}
      tabs={{
        value: "stammdaten",
        onChange: () => {},
        items: LOADING_TABS.map((label, index) => ({
          value: index === 0 ? "stammdaten" : `loading-${index}`,
          label,
          disabled: true,
        })),
      }}
    >
      <StudentDetailSkeleton />
    </TenantPage>
  );
}

// Mirrors the checkout/checkin action-card row and a Stammdaten-shaped field
// section. The header and tabs deliberately belong to StudentDetailLoadingPage
// so loading and loaded states share TenantPage as their page root.
function StudentDetailSkeleton() {
  return (
    <output
      role="status"
      aria-busy="true"
      aria-label="Kind wird geladen"
      data-testid="student-detail-skeleton"
      className="w-full"
    >
      <div className="flex gap-3 sm:gap-4">
        <Skeleton shape="card" className="h-20 w-full" />
        <Skeleton shape="card" className="h-20 w-full" />
      </div>

      <div className="mt-6 space-y-6 max-sm:mt-3 max-sm:space-y-3">
        <DetailSectionSkeleton fields={6} />
      </div>
    </output>
  );
}
