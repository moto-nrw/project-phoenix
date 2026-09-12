"use client";

import { Suspense, useState } from "react";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { TenantPage } from "~/components/ui/tenant-page";
import { TransitStudentsSection } from "~/components/rooms/transit-students-section";
import { RoomsGridSkeleton } from "../page-skeleton";

/** Der Rückweg der Kindakte aus dieser Liste. */
const TRANSIT_PATH = "/rooms/unterwegs";

/**
 * Kinder ohne Raumzuweisung (#3115). Bis dahin ein Slide-over auf /rooms
 * („Unterwegs"); jetzt eine Seite wie jeder Raum, damit „Zurück", ein
 * geteilter Link und ein Neuladen dort landen, wo man war.
 */
export default function TransitPage() {
  const [totalCount, setTotalCount] = useState<number | null>(null);

  return (
    <BinaryModeGuard title="Unterwegs">
      <Suspense fallback={<RoomsGridSkeleton />}>
        <TenantPage
          title="Unterwegs"
          stats={
            totalCount === null
              ? undefined
              : `${totalCount} ${totalCount === 1 ? "Kind" : "Kinder"}`
          }
          statsLoading={totalCount === null}
          back
          backHref="/rooms"
          backLabel="Zurück zu den Räumen"
          leading={<MotoConceptIcon concept="transit" size={40} />}
        >
          <TransitStudentsSection
            fromReferrer={TRANSIT_PATH}
            onTotalCountChange={setTotalCount}
          />
        </TenantPage>
      </Suspense>
    </BinaryModeGuard>
  );
}
