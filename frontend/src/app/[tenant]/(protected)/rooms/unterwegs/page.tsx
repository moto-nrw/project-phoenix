"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { TenantPage } from "~/components/ui/tenant-page";
import { TransitStudentsSection } from "~/components/rooms/transit-students-section";
import { resolveDetailReferrer } from "~/lib/tenant-path";
import { RoomsGridSkeleton } from "../page-skeleton";

/** Der Rückweg der Kindakte aus dieser Liste. */
const TRANSIT_PATH = "/rooms/unterwegs";

/**
 * Der Einstieg „Unterwegs“ auf „Alle Kinder“ (#5026) führt dorthin zurück;
 * jeder andere Rückweg geht zu den Räumen.
 */
const CHILD_OVERVIEW_PATH = "/students/search";

/**
 * Kinder ohne Raumzuweisung (#3115). Bis dahin ein Slide-over auf /rooms
 * („Unterwegs"); jetzt eine Seite wie jeder Raum, damit „Zurück", ein
 * geteilter Link und ein Neuladen dort landen, wo man war.
 */
export default function TransitPage() {
  return (
    <BinaryModeGuard title="Unterwegs">
      <Suspense fallback={<RoomsGridSkeleton />}>
        <TransitPageContent />
      </Suspense>
    </BinaryModeGuard>
  );
}

function TransitPageContent() {
  const [totalCount, setTotalCount] = useState<number | null>(null);
  const searchParams = useSearchParams();
  const referrer = resolveDetailReferrer(searchParams.get("from"), "/rooms", [
    "/rooms",
    CHILD_OVERVIEW_PATH,
  ]);
  const backLabel = referrer.startsWith(CHILD_OVERVIEW_PATH)
    ? "Zurück zur Kinderübersicht"
    : "Zurück zu den Räumen";
  const transitReferrer = `${TRANSIT_PATH}?from=${encodeURIComponent(referrer)}`;

  return (
    <TenantPage
      title="Unterwegs"
      stats={
        totalCount === null
          ? undefined
          : `${totalCount} ${totalCount === 1 ? "Kind" : "Kinder"}`
      }
      statsLoading={totalCount === null}
      back
      backHref={referrer}
      backLabel={backLabel}
      leading={<MotoConceptIcon concept="transit" size={40} />}
    >
      <TransitStudentsSection
        fromReferrer={transitReferrer}
        onTotalCountChange={setTotalCount}
      />
    </TenantPage>
  );
}
