"use client";

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
}: Readonly<{ referrer?: string }>) {
  return (
    <TenantPage
      title="Kindakte"
      back
      backHref={referrer}
      backLabel="Zurück zur Kinderübersicht"
      leading={<Skeleton className="h-12 w-12 shrink-0 rounded-xl" />}
      statsLoading
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
        <Skeleton className="h-20 w-full rounded-2xl" />
        <Skeleton className="h-20 w-full rounded-2xl" />
      </div>

      <div className="mt-6 space-y-6 max-sm:mt-3 max-sm:space-y-3">
        <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
          <Skeleton className="mb-4 h-5 w-40 rounded" />
          <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
            {Array.from({ length: 6 }, (_, field) => (
              <div key={field} className="space-y-1.5">
                <Skeleton className="h-3 w-20 rounded" />
                <Skeleton className="h-4 w-32 rounded" />
              </div>
            ))}
          </dl>
        </div>
      </div>
    </output>
  );
}
